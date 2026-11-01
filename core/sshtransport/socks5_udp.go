package sshtransport

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

func serveSOCKS5UDP(control net.Conn, upstream Dialer) {
	relay, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		writeSOCKSReply(control, 1)
		return
	}
	defer relay.Close()

	address := relay.LocalAddr().(*net.UDPAddr)
	reply := []byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}
	binary.BigEndian.PutUint16(reply[8:], uint16(address.Port))
	if _, err := control.Write(reply); err != nil {
		return
	}

	peerHost, _, _ := net.SplitHostPort(control.RemoteAddr().String())
	peerIP := net.ParseIP(peerHost)
	go func() {
		_, _ = io.Copy(io.Discard, control)
		_ = relay.Close()
	}()

	buffer := make([]byte, 65535)
	for {
		n, source, err := relay.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		if peerIP != nil && !source.IP.Equal(peerIP) {
			continue
		}
		host, port, payload, err := parseSOCKS5UDP(buffer[:n])
		if err != nil || port != 53 {
			continue
		}
		response, err := exchangeDNSOverTCP(upstream, host, payload)
		if err != nil {
			continue
		}
		packet, err := buildSOCKS5UDP(host, port, response)
		if err != nil {
			continue
		}
		_, _ = relay.WriteToUDP(packet, source)
	}
}

func parseSOCKS5UDP(packet []byte) (string, int, []byte, error) {
	if len(packet) < 4 || packet[0] != 0 || packet[1] != 0 || packet[2] != 0 {
		return "", 0, nil, fmt.Errorf("invalid SOCKS5 UDP header")
	}
	offset := 4
	var host string
	switch packet[3] {
	case 1:
		if len(packet) < offset+4+2 {
			return "", 0, nil, fmt.Errorf("truncated IPv4 UDP address")
		}
		host = net.IP(packet[offset : offset+4]).String()
		offset += 4
	case 3:
		if len(packet) < offset+1 {
			return "", 0, nil, fmt.Errorf("truncated domain UDP address")
		}
		size := int(packet[offset])
		offset++
		if len(packet) < offset+size+2 {
			return "", 0, nil, fmt.Errorf("truncated domain UDP address")
		}
		host = string(packet[offset : offset+size])
		offset += size
	case 4:
		if len(packet) < offset+16+2 {
			return "", 0, nil, fmt.Errorf("truncated IPv6 UDP address")
		}
		host = net.IP(packet[offset : offset+16]).String()
		offset += 16
	default:
		return "", 0, nil, fmt.Errorf("unsupported SOCKS5 UDP address type")
	}
	port := int(binary.BigEndian.Uint16(packet[offset : offset+2]))
	offset += 2
	return host, port, packet[offset:], nil
}

func buildSOCKS5UDP(host string, port int, payload []byte) ([]byte, error) {
	packet := []byte{0, 0, 0}
	ip := net.ParseIP(host)
	switch {
	case ip == nil:
		if len(host) > 255 {
			return nil, fmt.Errorf("DNS host name is too long")
		}
		packet = append(packet, 3, byte(len(host)))
		packet = append(packet, host...)
	case ip.To4() != nil:
		packet = append(packet, 1)
		packet = append(packet, ip.To4()...)
	default:
		packet = append(packet, 4)
		packet = append(packet, ip.To16()...)
	}
	var portBytes [2]byte
	binary.BigEndian.PutUint16(portBytes[:], uint16(port))
	packet = append(packet, portBytes[:]...)
	packet = append(packet, payload...)
	return packet, nil
}

func exchangeDNSOverTCP(upstream Dialer, host string, query []byte) ([]byte, error) {
	if len(query) == 0 || len(query) > 65535 {
		return nil, fmt.Errorf("invalid DNS query size")
	}
	conn, err := upstream.Dial("tcp", net.JoinHostPort(host, "53"))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))

	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(query)))
	if _, err := conn.Write(append(length[:], query...)); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(conn, length[:]); err != nil {
		return nil, err
	}
	size := int(binary.BigEndian.Uint16(length[:]))
	if size == 0 {
		return nil, fmt.Errorf("empty DNS response")
	}
	response := make([]byte, size)
	if _, err := io.ReadFull(conn, response); err != nil {
		return nil, err
	}
	return response, nil
}

