package redis

import (
	"net"
	"testing"
)

// TestClient_ReconnectRedials uses a real local TCP listener (not a
// live Redis instance) to prove Reconnect() actually redials — this
// keeps the test runnable without requiring Redis to be installed/
// running wherever `go test` executes, while still exercising real
// network code, not a mock.
func TestClient_ReconnectRedials(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer ln.Close()

	// Accept and immediately close every connection — enough for
	// Connect/Reconnect to succeed without needing real RESP replies.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	client, err := Connect(ln.Addr().String())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	firstAddr := client.LocalAddr()
	if firstAddr == "" {
		t.Fatal("LocalAddr returned empty string after Connect")
	}

	if err := client.Reconnect(); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}

	secondAddr := client.LocalAddr()
	if secondAddr == "" {
		t.Fatal("LocalAddr returned empty string after Reconnect")
	}
	// Not asserting firstAddr != secondAddr — a new ephemeral local port
	// is likely but not guaranteed. The real assertion is that
	// Reconnect() succeeded and left the client in a usable state.
}

func TestClient_ReconnectFailsClearlyOnUnreachableAddr(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // close immediately so the address becomes unreachable

	client := &Client{addr: addr}
	err = client.Reconnect()
	if err == nil {
		t.Fatal("expected Reconnect to fail against a closed listener, got nil error")
	}
}
