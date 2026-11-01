package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/private-gojo/Tunnel-Go-/core/payload"
)

// Dialer establishes the stream that the SSH handshake will run over.
type Dialer struct {
	ProxyAddress       string
	UseTLS             bool
	ServerName         string
	InsecureSkipVerify bool
	Payload            string
	SplitMode          string
	WaitForHTTPResponse bool
	Timeout            time.Duration
	Formatter          *payload.Formatter
}

func (d *Dialer) DialContext(ctx context.Context, network, target string) (net.Conn, error) {
	host, portText, err := net.SplitHostPort(target)
	if err != nil || host == "" {
		return nil, fmt.Errorf("SSH target must be host:port")
	}
	if port, err := strconv.Atoi(portText); err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid SSH target port %q", portText)
	}

	endpoint := target
	if d.ProxyAddress != "" {
		endpoint = d.ProxyAddress
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	raw, err := (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext(ctx, network, endpoint)
	if err != nil {
		return nil, fmt.Errorf("dial tunnel endpoint: %w", err)
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = raw.Close() })
	fail := func(err error) (net.Conn, error) {
		stopCancel()
		_ = raw.Close()
		return nil, err
	}
	if err := raw.SetDeadline(time.Now().Add(timeout)); err != nil {
		return fail(err)
	}

	var conn net.Conn = raw
	if d.UseTLS {
		serverName := d.ServerName
		if serverName == "" {
			serverName = host
		}
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: serverName,
			MinVersion: tls.VersionTLS12,
			InsecureSkipVerify: d.InsecureSkipVerify,
		})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fail(fmt.Errorf("TLS handshake: %w", err))
		}
		conn = tlsConn
	}

	if d.Payload != "" {
		formatter := d.Formatter
		if formatter == nil {
			formatter = payload.NewFormatter()
		}
		request, err := formatter.Format(d.Payload, payload.Context{Host: host, Port: mustPort(portText)})
		if err != nil {
			return fail(fmt.Errorf("format payload: %w", err))
		}
		if err := writeFragments(conn, request, d.SplitMode); err != nil {
			return fail(fmt.Errorf("write payload: %w", err))
		}
	} else if d.ProxyAddress != "" {
		request := fmt.Sprintf("CONNECT %s:%s HTTP/1.1\r\nHost: %s:%s\r\n\r\n", host, portText, host, portText)
		if _, err := io.WriteString(conn, request); err != nil {
			return fail(fmt.Errorf("write HTTP CONNECT: %w", err))
		}
	}

	if d.WaitForHTTPResponse {
		if err := readHTTPResponse(conn); err != nil {
			return fail(err)
		}
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fail(err)
	}
	stopCancel()
	return conn, nil
}

func mustPort(value string) int {
	port, _ := strconv.Atoi(value)
	return port
}

func writeFragments(w io.Writer, value, mode string) error {
	delimiter := ""
	switch mode {
	case "split":
		delimiter = "[split]"
	case "delay_split":
		delimiter = "[delay_split]"
	default:
		_, err := io.WriteString(w, value)
		return err
	}
	parts := strings.Split(value, delimiter)
	for i, part := range parts {
		if _, err := io.WriteString(w, part); err != nil {
			return err
		}
		if i+1 < len(parts) && mode == "delay_split" {
			time.Sleep(time.Second)
		}
	}
	return nil
}

func readHTTPResponse(conn net.Conn) error {
	line, err := readHTTPLine(conn)
	if err != nil {
		return fmt.Errorf("read proxy status: %w", err)
	}
	fields := strings.Fields(strings.TrimRight(line, "\r\n"))
	if len(fields) < 2 || !strings.HasPrefix(fields[0], "HTTP/") {
		return fmt.Errorf("invalid HTTP proxy response %q", strings.TrimSpace(line))
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil || status < 200 || status >= 300 {
		return fmt.Errorf("proxy rejected tunnel: %s", strings.TrimSpace(line))
	}
	for {
		line, err = readHTTPLine(conn)
		if err != nil {
			return fmt.Errorf("read proxy headers: %w", err)
		}
		if strings.TrimRight(line, "\r\n") == "" {
			return nil
		}
	}
}

func readHTTPLine(r io.Reader) (string, error) {
	var line []byte
	var one [1]byte
	for len(line) < 8192 {
		if _, err := io.ReadFull(r, one[:]); err != nil {
			return "", err
		}
		if one[0] == '\n' {
			return string(line), nil
		}
		line = append(line, one[0])
	}
	return "", fmt.Errorf("HTTP response line exceeds limit")
}
