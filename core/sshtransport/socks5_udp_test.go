package sshtransport

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

type udpTestDialer struct{ address string }
func (d udpTestDialer) Dial(network, address string) (net.Conn, error) { return net.Dial(network, d.address) }

func TestUDPAssociateDNSRelay(t *testing.T) {
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	defer backend.Close()
	go func() {
		c, e := backend.Accept()
		if e != nil { return }
		defer c.Close()
		var size [2]byte
		if _, e = io.ReadFull(c, size[:]); e != nil { return }
		buf := make([]byte, binary.BigEndian.Uint16(size[:]))
		if _, e = io.ReadFull(c, buf); e != nil { return }
		answer := []byte{9, 8, 7}
		binary.BigEndian.PutUint16(size[:], uint16(len(answer)))
		_, _ = c.Write(append(size[:], answer...))
	}()
	control, server := net.Pipe()
	defer control.Close()
	go serveSOCKS5UDP(server, udpTestDialer{backend.Addr().String()})
	response := make([]byte, 10)
	if _, err := io.ReadFull(control, response); err != nil { t.Fatal(err) }
	if response[1] != 0 { t.Fatalf("associate status %d", response[1]) }
	port := binary.BigEndian.Uint16(response[8:10])
	udp, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127,0,0,1), Port: int(port)})
	if err != nil { t.Fatal(err) }
	defer udp.Close()
	_ = udp.SetDeadline(time.Now().Add(3*time.Second))
	packet, err := buildSOCKS5UDP("dns.test", 53, []byte{1,2,3})
	if err != nil { t.Fatal(err) }
	if _, err = udp.Write(packet); err != nil { t.Fatal(err) }
	buf := make([]byte, 512)
	n, err := udp.Read(buf)
	if err != nil { t.Fatal(err) }
	_, p, body, err := parseSOCKS5UDP(buf[:n])
	if err != nil { t.Fatal(err) }
	if p != 53 || string(body) != string([]byte{9,8,7}) { t.Fatalf("unexpected DNS relay response") }
}
