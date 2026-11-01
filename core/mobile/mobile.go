package mobile

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/private-gojo/Tunnel-Go-/core/sshtransport"
	"github.com/private-gojo/Tunnel-Go-/core/transport"
)

var state struct {
	sync.Mutex
	cancel context.CancelFunc
	listener net.Listener
	client *sshtransport.Client
}

// StartSSH connects to an SSH endpoint and exposes it as a local SOCKS5 proxy.
func StartSSH(address, username, password, privateKey, privateKeyPassphrase, knownHostsPath, proxyAddress, serverName, payloadText, splitMode string, useTLS, insecureTLS, insecureSSH, waitForHTTPResponse bool, localPort int32) error {
	state.Lock()
	defer state.Unlock()
	if state.cancel != nil {
		return fmt.Errorf("tunnel is already running")
	}
	if localPort < 1 || localPort > 65535 {
		return fmt.Errorf("invalid local SOCKS5 port")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cfg := sshtransport.Config{
		Address: address,
		Username: username,
		Password: password,
		PrivateKey: []byte(privateKey),
		PrivateKeyPassphrase: []byte(privateKeyPassphrase),
		KnownHostsPath: knownHostsPath,
		InsecureSkipHostKeyVerification: insecureSSH,
		HandshakeTimeoutSeconds: 15,
	}
	dialer := &transport.Dialer{
		ProxyAddress: proxyAddress,
		UseTLS: useTLS,
		ServerName: serverName,
		InsecureSkipVerify: insecureTLS,
		Payload: payloadText,
		SplitMode: splitMode,
		WaitForHTTPResponse: waitForHTTPResponse,
	}
	client, err := sshtransport.ConnectVia(ctx, cfg, dialer.DialContext)
	if err != nil {
		cancel()
		return err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(localPort)))
	if err != nil {
		_ = client.Close()
		cancel()
		return fmt.Errorf("start local SOCKS5 listener: %w", err)
	}

	state.cancel = cancel
	state.listener = listener
	state.client = client
	go func() {
		_ = sshtransport.ServeSOCKS5(ctx, listener, client)
		state.Lock()
		if state.listener == listener {
			state.cancel = nil
			state.listener = nil
			state.client = nil
		}
		state.Unlock()
		_ = client.Close()
		cancel()
	}()
	return nil
}

// IsRunning reports whether the local SOCKS5 proxy is active.
func IsRunning() bool {
	state.Lock()
	defer state.Unlock()
	return state.cancel != nil
}

// Stop closes the local SOCKS5 proxy and its SSH connection.
func Stop() {
	state.Lock()
	cancel := state.cancel
	listener := state.listener
	client := state.client
	state.cancel = nil
	state.listener = nil
	state.client = nil
	state.Unlock()
	if cancel != nil {
		cancel()
	}
	if listener != nil {
		_ = listener.Close()
	}
	if client != nil {
		_ = client.Close()
	}
}
