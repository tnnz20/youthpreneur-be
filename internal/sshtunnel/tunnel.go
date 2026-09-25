package sshtunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/tnnz20/youthpreneur-be/internal/config"
)

// Tunnel represents an active SSH port forwarder.
type Tunnel struct {
	LocalAddr string
	LocalPort int
	closeOnce sync.Once
	listener  net.Listener
	client    *ssh.Client
	done      chan struct{}
}

// Close terminates the local listener, closes the SSH client, and halts forwarding goroutines.
func (t *Tunnel) Close() error {
	var errs []error
	t.closeOnce.Do(func() {
		close(t.done)
		if err := t.listener.Close(); err != nil {
			errs = append(errs, err)
		}
		if err := t.client.Close(); err != nil {
			errs = append(errs, err)
		}
	})
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Open establishes an SSH connection to sshCfg.Host:sshCfg.Port, binds a local TCP
// listener on 127.0.0.1:0, and forwards incoming connections to remoteHost:remotePort.
func Open(ctx context.Context, sshCfg config.SSHConfig, remoteHost string, remotePort int) (*Tunnel, error) {
	if err := sshCfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate ssh config: %w", err)
	}

	var authMethods []ssh.AuthMethod
	if sshCfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(sshCfg.Password))
	}

	var hostKeyCallback ssh.HostKeyCallback
	if sshCfg.KnownHostsFile != "" {
		cb, err := knownhosts.New(sshCfg.KnownHostsFile)
		if err != nil {
			return nil, fmt.Errorf("read known hosts file %q: %w", sshCfg.KnownHostsFile, err)
		}
		hostKeyCallback = cb
	} else {
		// When no known_hosts file is specified, accept any host key for development convenience.
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	clientConfig := &ssh.ClientConfig{
		User:            sshCfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         15 * time.Second,
	}

	sshAddr := net.JoinHostPort(sshCfg.Host, strconv.Itoa(sshCfg.Port))
	client, err := ssh.Dial("tcp", sshAddr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", sshAddr, err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("listen on local port: %w", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		_ = client.Close()
		return nil, errors.New("listener is not a TCP listener")
	}

	t := &Tunnel{
		LocalAddr: tcpAddr.IP.String(),
		LocalPort: tcpAddr.Port,
		listener:  listener,
		client:    client,
		done:      make(chan struct{}),
	}

	targetAddr := net.JoinHostPort(remoteHost, strconv.Itoa(remotePort))
	go t.forward(targetAddr)

	return t, nil
}

func (t *Tunnel) forward(targetAddr string) {
	for {
		localConn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.done:
				return
			default:
				return
			}
		}

		go func(local net.Conn) {
			defer local.Close()

			remoteConn, err := t.client.Dial("tcp", targetAddr)
			if err != nil {
				return
			}
			defer remoteConn.Close()

			pipe(local, remoteConn)
		}(localConn)
	}
}

func pipe(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(c1, c2)
		if tc, ok := c1.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(c2, c1)
		if tc, ok := c2.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	wg.Wait()
}
