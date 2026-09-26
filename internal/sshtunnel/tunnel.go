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
	dialer := net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", sshAddr)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", sshAddr, err)
	}

	c, chans, reqs, err := ssh.NewClientConn(netConn, sshAddr, clientConfig)
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("ssh handshake %s: %w", sshAddr, err)
	}
	client := ssh.NewClient(c, chans, reqs)

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
			return
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
	var once sync.Once
	closeBoth := func() {
		_ = c1.Close()
		_ = c2.Close()
	}

	wg.Add(2)

	go func() {
		defer wg.Done()
		defer once.Do(closeBoth)
		_, _ = io.Copy(c1, c2)
	}()

	go func() {
		defer wg.Done()
		defer once.Do(closeBoth)
		_, _ = io.Copy(c2, c1)
	}()

	wg.Wait()
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// ResolvePostgres resolves PostgreSQL connection settings, automatically
// establishing an SSH tunnel when useSSH is true. The caller must close the
// returned io.Closer when finished.
func ResolvePostgres(ctx context.Context, useSSH bool) (config.PostgresConfig, io.Closer, error) {
	if !useSSH {
		cfg := config.Load()
		return cfg.Postgres, noopCloser{}, nil
	}

	sshCfg, pgCfg, err := config.LoadSSHConfig()
	if err != nil {
		return config.PostgresConfig{}, nil, fmt.Errorf("load ssh config: %w", err)
	}

	tunnel, err := Open(ctx, sshCfg, pgCfg.Host, pgCfg.Port)
	if err != nil {
		return config.PostgresConfig{}, nil, fmt.Errorf("open ssh tunnel: %w", err)
	}

	resolved := config.PostgresConfig{
		Host:     tunnel.LocalAddr,
		Port:     tunnel.LocalPort,
		User:     pgCfg.User,
		Password: pgCfg.Password,
		Database: pgCfg.Database,
		SSLMode:  pgCfg.SSLMode,
	}

	return resolved, tunnel, nil
}
