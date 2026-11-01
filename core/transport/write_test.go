package transport

import (
	"bytes"
	"testing"
)

func TestWriteFragments(t *testing.T) {
	var got bytes.Buffer
	if err := writeFragments(&got, "one[split]two", "split"); err != nil {
		t.Fatal(err)
	}
	if got.String() != "onetwo" {
		t.Fatalf("got %q", got.String())
	}
}
