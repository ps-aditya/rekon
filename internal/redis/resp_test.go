package redis

import (
	"bufio"
	"strings"
	"testing"
)

// realSlowlogCapture is the exact raw bytes captured from a live local
// Redis instance for `SLOWLOG GET 2`, via a raw socket (not redis-cli's
// decoded view) — the actual wire bytes this parser has to handle,
// including the nested array-within-array structure.
const realSlowlogCapture = "*2\r\n" +
	"*6\r\n" +
	":3\r\n" +
	":1784285895\r\n" +
	":7\r\n" +
	"*3\r\n$7\r\nslowlog\r\n$3\r\nget\r\n$1\r\n2\r\n" +
	"$15\r\n127.0.0.1:41308\r\n" +
	"$0\r\n\r\n" +
	"*6\r\n" +
	":2\r\n" +
	":1784285895\r\n" +
	":3\r\n" +
	"*2\r\n$3\r\nget\r\n$3\r\nfoo\r\n" +
	"$15\r\n127.0.0.1:41296\r\n" +
	"$0\r\n\r\n"

func TestReadValue_ParsesRealSlowlogArray(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(realSlowlogCapture))
	v, err := readValue(r)
	if err != nil {
		t.Fatalf("readValue: %v", err)
	}

	if v.Type != TypeArray {
		t.Fatalf("top-level: got type %v, want TypeArray", v.Type)
	}
	if len(v.Array) != 2 {
		t.Fatalf("top-level: got %d entries, want 2", len(v.Array))
	}

	entry := v.Array[0]
	if entry.Type != TypeArray || len(entry.Array) != 6 {
		t.Fatalf("entry 0: got type %v len %d, want Array of 6", entry.Type, len(entry.Array))
	}

	id := entry.Array[0]
	if id.Type != TypeInteger || id.Int != 3 {
		t.Errorf("entry 0 id: got %+v, want Integer 3", id)
	}

	timestamp := entry.Array[1]
	if timestamp.Type != TypeInteger || timestamp.Int != 1784285895 {
		t.Errorf("entry 0 timestamp: got %+v, want Integer 1784285895", timestamp)
	}

	duration := entry.Array[2]
	if duration.Type != TypeInteger || duration.Int != 7 {
		t.Errorf("entry 0 duration: got %+v, want Integer 7", duration)
	}

	command := entry.Array[3]
	if command.Type != TypeArray || len(command.Array) != 3 {
		t.Fatalf("entry 0 command: got type %v len %d, want Array of 3", command.Type, len(command.Array))
	}
	wantCmd := []string{"slowlog", "get", "2"}
	for i, want := range wantCmd {
		if command.Array[i].Str != want {
			t.Errorf("command[%d]: got %q, want %q", i, command.Array[i].Str, want)
		}
	}

	clientAddr := entry.Array[4]
	if clientAddr.Str != "127.0.0.1:41308" {
		t.Errorf("entry 0 client addr: got %q, want 127.0.0.1:41308", clientAddr.Str)
	}

	clientName := entry.Array[5]
	if clientName.Str != "" {
		t.Errorf("entry 0 client name: got %q, want empty string", clientName.Str)
	}
}

func TestReadValue_Integer(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(":42\r\n"))
	v, err := readValue(r)
	if err != nil {
		t.Fatalf("readValue: %v", err)
	}
	if v.Type != TypeInteger || v.Int != 42 {
		t.Errorf("got %+v, want Integer 42", v)
	}
}

func TestReadValue_SimpleString(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("+OK\r\n"))
	v, err := readValue(r)
	if err != nil {
		t.Fatalf("readValue: %v", err)
	}
	if v.Type != TypeSimpleString || v.Str != "OK" {
		t.Errorf("got %+v, want SimpleString OK", v)
	}
}

func TestReadValue_Error(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("-ERR something went wrong\r\n"))
	v, err := readValue(r)
	if err != nil {
		t.Fatalf("readValue: %v", err)
	}
	if v.Type != TypeError || v.Str != "ERR something went wrong" {
		t.Errorf("got %+v, want Error 'ERR something went wrong'", v)
	}
}

func TestReadValue_EmptyArray(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("*0\r\n"))
	v, err := readValue(r)
	if err != nil {
		t.Fatalf("readValue: %v", err)
	}
	if v.Type != TypeArray || len(v.Array) != 0 {
		t.Errorf("got %+v, want empty Array", v)
	}
}
