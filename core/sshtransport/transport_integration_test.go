package sshtransport_test

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/private-gojo/Tunnel-Go-/core/sshtransport"
	"github.com/private-gojo/Tunnel-Go-/core/transport"
	"golang.org/x/crypto/ssh"
)

func TestSSHHandshakeThroughHTTPConnectProxy(t *testing.T) {
	_, serverKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(serverKey)
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if meta.User() != "test-user" || string(password) != "test-pass" {
				return nil, fmt.Errorf("invalid credentials")
			}
			return nil, nil
		},
	}
	serverConfig.AddHostKey(signer)

	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	go func() {
		conn, err := backend.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		sshConn, channels, requests, err := ssh.NewServerConn(conn, serverConfig)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(requests)
		for channel := range channels {
			_ = channel.Reject(ssh.UnknownChannelType, "test server")
		}
		_ = sshConn.Close()
	}()

	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go func() {
		client, err := proxy.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		reader := bufio.NewReader(client)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.TrimRight(line, "\r\n") == "" {
				break
			}
		}
		upstream, err := net.Dial("tcp", backend.Addr().String())
		if err != nil {
			return
		}
		defer upstream.Close()
		if _, err := io.WriteString(client, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
			return
		}
		done := make(chan struct{}, 2)
		go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
		go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
		<-done
	}()

	cfg := sshtransport.Config{
		Address: "example.test:22",
		Username: "test-user",
		Password: "test-pass",
		InsecureSkipHostKeyVerification: true,
		HandshakeTimeoutSeconds: 5,
	}
	dialer := &transport.Dialer{
		ProxyAddress: proxy.Addr().String(),
		WaitForHTTPResponse: true,
		Timeout: 5 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client, err := sshtransport.ConnectVia(ctx, cfg, dialer.DialContext)
	if err != nil {
		t.Fatalf("SSH over HTTP CONNECT failed: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}
