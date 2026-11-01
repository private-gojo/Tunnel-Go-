package sshtransport

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	conn *ssh.Client
}

type ContextDialer func(context.Context, string, string) (net.Conn, error)

func Connect(ctx context.Context, cfg Config) (*Client, error) {
	return ConnectVia(ctx, cfg, nil)
}

func ConnectVia(ctx context.Context, cfg Config, dial ContextDialer) (*Client, error) {
	host, port, err := net.SplitHostPort(cfg.Address)
	if err != nil || host == "" {
		return nil, fmt.Errorf("SSH address must be host:port")
	}
	if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
		return nil, fmt.Errorf("invalid SSH port %q", port)
	}

	clientConfig, err := cfg.clientConfig()
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(cfg.HandshakeTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	var raw net.Conn
	if dial == nil {
		dialer := net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
		raw, err = dialer.DialContext(ctx, "tcp", cfg.Address)
	} else {
		raw, err = dial(ctx, "tcp", cfg.Address)
	}
	if err != nil {
		return nil, fmt.Errorf("dial SSH server: %w", err)
	}

	stopCancel := context.AfterFunc(ctx, func() { _ = raw.Close() })
	_ = raw.SetDeadline(time.Now().Add(timeout))
	sshConn, chans, reqs, err := ssh.NewClientConn(raw, cfg.Address, clientConfig)
	if err != nil {
		stopCancel()
		_ = raw.Close()
		return nil, fmt.Errorf("SSH handshake: %w", err)
	}
	stopCancel()
	_ = raw.SetDeadline(time.Time{})

	return &Client{conn: ssh.NewClient(sshConn, chans, reqs)}, nil
}

func (c *Client) Dial(network, address string) (net.Conn, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("SSH client is not connected")
	}
	return c.conn.Dial(network, address)
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
