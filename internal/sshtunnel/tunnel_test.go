package sshtunnel_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/tnnz20/youthpreneur-be/internal/config"
	"github.com/tnnz20/youthpreneur-be/internal/sshtunnel"
)

func startMockTargetServer(t *testing.T) (net.Listener, int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start target listener: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				_, _ = c.Write(append([]byte("echo:"), buf[:n]...))
			}(conn)
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	return listener, port
}

func startMockSSHServer(t *testing.T, targetPort int) (net.Listener, int, *config.SSHConfig) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	sshConfig := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "testpass" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}
	sshConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start ssh listener: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				sConn, chans, reqs, err := ssh.NewServerConn(c, sshConfig)
				if err != nil {
					return
				}
				defer sConn.Close()
				go ssh.DiscardRequests(reqs)

				for newChannel := range chans {
					if newChannel.ChannelType() != "direct-tcpip" {
						_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
						continue
					}

					channel, requests, err := newChannel.Accept()
					if err != nil {
						continue
					}
					go ssh.DiscardRequests(requests)

					destConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", targetPort))
					if err != nil {
						_ = channel.Close()
						continue
					}

					go func() {
						defer destConn.Close()
						defer channel.Close()
						_, _ = io.Copy(destConn, channel)
					}()
					go func() {
						defer destConn.Close()
						defer channel.Close()
						_, _ = io.Copy(channel, destConn)
					}()
				}
			}(conn)
		}
	}()

	sshPort := listener.Addr().(*net.TCPAddr).Port
	clientCfg := &config.SSHConfig{
		Host:     "127.0.0.1",
		Port:     sshPort,
		User:     "testuser",
		Password: "testpass",
	}

	return listener, sshPort, clientCfg
}

func TestSSHTunnelForwardsTraffic(t *testing.T) {
	targetListener, targetPort := startMockTargetServer(t)
	defer targetListener.Close()

	sshListener, _, sshCfg := startMockSSHServer(t, targetPort)
	defer sshListener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tunnel, err := sshtunnel.Open(ctx, *sshCfg, "127.0.0.1", targetPort)
	if err != nil {
		t.Fatalf("failed to open tunnel: %v", err)
	}
	defer func() {
		if err := tunnel.Close(); err != nil {
			t.Errorf("tunnel.Close() error = %v", err)
		}
	}()

	localAddr := net.JoinHostPort(tunnel.LocalAddr, strconv.Itoa(tunnel.LocalPort))
	conn, err := net.DialTimeout("tcp", localAddr, 3*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to local tunnel endpoint: %v", err)
	}
	defer conn.Close()

	msg := "hello-through-ssh"
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatalf("failed to write to tunnel: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read from tunnel: %v", err)
	}

	got := string(buf[:n])
	want := "echo:hello-through-ssh"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
