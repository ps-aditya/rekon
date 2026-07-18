package redis

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"testing"
)

// newPipedClient creates a Client wired to an in-memory net.Pipe instead
// of a real TCP connection to Redis, and a goroutine that discards
// whatever command Rekon sends and replies with a fixed, pre-scripted
// response. This exercises the real Client.call() -> sendCommand ->
// readValue path — the actual code under test — without requiring a
// live Redis instance for every unit test run.
func newPipedClient(t *testing.T, response string) *Client {
	t.Helper()
	clientConn, serverConn := net.Pipe()

	go func() {
		buf := make([]byte, 4096)
		serverConn.Read(buf) // drain the command Rekon sends
		serverConn.Write([]byte(response))
		serverConn.Close()
	}()

	t.Cleanup(func() { clientConn.Close() })

	return &Client{
		conn:   clientConn,
		reader: bufio.NewReader(clientConn),
	}
}

func TestSlowlogGet_ParsesRealCapturedResponse(t *testing.T) {
	// Reuses the exact real captured bytes from resp_test.go's fixture.
	client := newPipedClient(t, realSlowlogCapture)

	entries, err := client.SlowlogGet(2)
	if err != nil {
		t.Fatalf("SlowlogGet: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.ID != 3 {
		t.Errorf("entry 0 ID: got %d, want 3", e.ID)
	}
	if e.DurationMicros != 7 {
		t.Errorf("entry 0 DurationMicros: got %d, want 7", e.DurationMicros)
	}
	wantArgs := []string{"slowlog", "get", "2"}
	if len(e.Args) != len(wantArgs) {
		t.Fatalf("entry 0 Args: got %v, want %v", e.Args, wantArgs)
	}
	for i, want := range wantArgs {
		if e.Args[i] != want {
			t.Errorf("entry 0 Args[%d]: got %q, want %q", i, e.Args[i], want)
		}
	}
	if e.ClientAddr != "127.0.0.1:41308" {
		t.Errorf("entry 0 ClientAddr: got %q, want 127.0.0.1:41308", e.ClientAddr)
	}
}

func TestSlowlogGet_EmptySlowlog(t *testing.T) {
	client := newPipedClient(t, "*0\r\n")

	entries, err := client.SlowlogGet(10)
	if err != nil {
		t.Fatalf("SlowlogGet: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0 for an empty slowlog", len(entries))
	}
}

func TestSlowlogGet_WrongReplyTypeErrors(t *testing.T) {
	// A bulk string reply where an array was expected — SlowlogGet
	// should error clearly, not panic or silently misparse.
	client := newPipedClient(t, "$2\r\nOK\r\n")

	_, err := client.SlowlogGet(10)
	if err == nil {
		t.Fatal("expected an error for a non-array reply, got nil")
	}
}

func TestSlowlogGet_SurfacesRealACLErrorMessage(t *testing.T) {
	// The exact RESP Error shape observed when testing against a real
	// Redis instance with SLOWLOG restricted via ACL SETUSER default
	// -slowlog: a '-' prefixed error line, not an array or bulk string.
	response := "-NOPERM this user has no permissions to run the 'slowlog|get' command\r\n"
	client := newPipedClient(t, response)

	_, err := client.SlowlogGet(10)
	if err == nil {
		t.Fatal("expected an error for a NOPERM reply, got nil")
	}
	if !strings.Contains(err.Error(), "NOPERM") {
		t.Errorf("error message %q doesn't surface Redis's actual NOPERM message — "+
			"a user seeing this should see *why* it failed, not a generic type mismatch", err.Error())
	}
}

func TestClientList_ParsesBulkStringReply(t *testing.T) {
	sample := "id=3 addr=127.0.0.1:52700 name= age=0 idle=0\n"
	response := "$" + strconv.Itoa(len(sample)) + "\r\n" + sample + "\r\n"

	client := newPipedClient(t, response)

	got, err := client.ClientList()
	if err != nil {
		t.Fatalf("ClientList: %v", err)
	}
	if got != sample {
		t.Errorf("got %q, want %q", got, sample)
	}
}
