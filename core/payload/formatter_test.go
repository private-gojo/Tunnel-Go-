package payload

import (
	"bytes"
	"testing"
	"time"
)

func TestFormatLegacyMacros(t *testing.T) {
	f := NewFormatter()
	got, err := f.Format(" [method] [host] [port] [host_port] [protocol] [ssh][crlf][ua]", Context{
		Host: "edge.example.com", Port: 443, UserAgent: "TestAgent",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := " CONNECT edge.example.com 443 edge.example.com:443 HTTP/1.0 edge.example.com:443\r\nTestAgent"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRotateCycles(t *testing.T) {
	f := NewFormatter()
	ctx := Context{Host: "example.com", Port: 443}
	for i, want := range []string{"a", "b", "a"} {
		got, err := f.Format("[rotate=a;b]", ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("iteration %d: got %q, want %q", i, got, want)
		}
	}
}

func TestWriteFragments(t *testing.T) {
	var b bytes.Buffer
	if err := WriteFragments(&b, "A[split]B", "split", 0); err != nil {
		t.Fatal(err)
	}
	if b.String() != "AB" {
		t.Fatalf("got %q", b.String())
	}
	if err := WriteFragments(&b, "C[delay_split]D", "delay_split", time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	if b.String() != "ABCD" {
		t.Fatalf("got %q", b.String())
	}
}
