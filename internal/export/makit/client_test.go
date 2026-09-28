package makit

import (
	"context"
	"testing"
)

func TestResolveDialAddrPassthrough(t *testing.T) {
	got, err := resolveDialAddr(context.Background(), "192.168.0.42:5683")
	if err != nil {
		t.Fatal(err)
	}
	if got != "192.168.0.42:5683" {
		t.Fatalf("got %q", got)
	}

	got, err = resolveDialAddr(context.Background(), "example.com:5683")
	if err != nil {
		t.Fatal(err)
	}
	if got != "example.com:5683" {
		t.Fatalf("non-.local got %q", got)
	}
}
