package sshtransport

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
)

type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

func ServeSOCKS5(ctx context.Context, listener net.Listener, upstream Dialer) error {
	if listener == nil || upstream == nil {
		return fmt.Errorf("listener and upstream dialer are required")
	}
	var workers sync.WaitGroup
	var activeMu sync.Mutex
	active := make(map[net.Conn]struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = listener.Close()
		activeMu.Lock()
		for conn := range active {
			_ = conn.Close()
		}
		activeMu.Unlock()
	})
	defer stop()

	for {
		conn, err := listener.Accept()
		if err != nil {
			workers.Wait()
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		activeMu.Lock()
		active[conn] = struct{}{}
		activeMu.Unlock()
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() {
				activeMu.Lock()
				delete(active, conn)
				activeMu.Unlock()
			}()
			handleSOCKS5(conn, upstream)
		}()
	}
}

func handleSOCKS5(client net.Conn, upstream Dialer) {
	defer client.Close()
	header := make([]byte, 2)
	if _, err := io.ReadFull(client, header); err != nil || header[0] != 5 || header[1] == 0 {
		return
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(client, methods); err != nil {
		return
	}
	noAuth := false
	for _, method := range methods {
		if method == 0 {
			noAuth = true
			break
		}
	}
	if !noAuth {
		_, _ = client.Write([]byte{5, 0xff})
		return
	}
	if _, err := client.Write([]byte{5, 0}); err != nil {
		return
	}

	request := make([]byte, 4)
	if _, err := io.ReadFull(client, request); err != nil || request[0] != 5 || request[1] != 1 || request[2] != 0 {
		writeSOCKSReply(client, 7)
		return
	}

	var host string
	switch request[3] {
	case 1:
		address := make([]byte, 4)
		if _, err := io.ReadFull(client, address); err != nil {
			return
		}
		host = net.IP(address).String()
	case 3:
		length := []byte{0}
		if _, err := io.ReadFull(client, length); err != nil {
			return
		}
		address := make([]byte, int(length[0]))
		if _, err := io.ReadFull(client, address); err != nil {
			return
		}
		host = string(address)
	case 4:
		address := make([]byte, 16)
		if _, err := io.ReadFull(client, address); err != nil {
			return
		}
		host = net.IP(address).String()
	default:
		writeSOCKSReply(client, 8)
		return
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(client, portBytes); err != nil {
		return
	}
	target := net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(portBytes))))
	remote, err := upstream.Dial("tcp", target)
	if err != nil {
		writeSOCKSReply(client, 5)
		return
	}
	defer remote.Close()
	if !writeSOCKSReply(client, 0) {
		return
	}

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(remote, client)
		if tcp, ok := remote.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, remote)
		if tcp, ok := client.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
}

func writeSOCKSReply(conn net.Conn, status byte) bool {
	_, err := conn.Write([]byte{5, status, 0, 1, 0, 0, 0, 0, 0, 0})
	return err == nil
}
