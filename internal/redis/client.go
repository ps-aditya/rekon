// Package redis implements just enough of Redis's RESP wire protocol to
// send a command and read back a reply, using only the standard library.
//
// This exists to prove the connection and protocol work in isolation,
// before any UI or concurrency is added (Sprint 0, per ROADMAP.md).
// A third-party client library (e.g. go-redis) may replace this later,
// but understanding the raw protocol first is the point of this sprint.
package redis

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Client holds an open connection to a single Redis instance.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Connect opens a TCP connection to addr (e.g. "localhost:6379").
// It does not authenticate or select a database — that's out of scope
// for Sprint 0, which only needs to prove one command round-trips.
func Connect(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connecting to redis at %s: %w", addr, err)
	}
	return &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// sendCommand encodes a command as a RESP array of bulk strings and
// writes it to the connection.
//
// Example: sendCommand("INFO") writes exactly:
//
//	*1\r\n$4\r\nINFO\r\n
//
// *1      -> an array of 1 element
// $4      -> the next bulk string is 4 bytes long
// INFO    -> the bytes themselves
// \r\n    -> line terminator (required after every RESP element)
func (c *Client) sendCommand(args ...string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
	}
	_, err := c.conn.Write([]byte(b.String()))
	if err != nil {
		return fmt.Errorf("writing command %v: %w", args, err)
	}
	return nil
}

// readBulkString reads a single RESP bulk-string reply, of the form:
//
//	$<byte-length>\r\n<payload>\r\n
//
// This is the reply type INFO returns. Other RESP types (simple strings,
// errors, integers, arrays) exist but aren't needed until later panels
// (e.g. SLOWLOG returns an array) — deliberately not handled yet, so this
// stays a small, explainable first piece.
func (c *Client) readBulkString() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading reply header: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")

	if len(line) == 0 || line[0] != '$' {
		return "", fmt.Errorf("unexpected reply type, want bulk string ($), got: %q", line)
	}

	length, err := strconv.Atoi(line[1:])
	if err != nil {
		return "", fmt.Errorf("parsing bulk string length from %q: %w", line, err)
	}
	if length < 0 {
		// $-1 is RESP's "nil" bulk string — not expected for INFO, but
		// worth naming explicitly rather than silently mishandling it.
		return "", fmt.Errorf("received nil bulk string reply")
	}

	// Read exactly `length` bytes of payload, then the trailing \r\n.
	payload := make([]byte, length)
	if _, err := readFull(c.reader, payload); err != nil {
		return "", fmt.Errorf("reading bulk string payload: %w", err)
	}
	if _, err := c.reader.Discard(2); err != nil { // trailing \r\n
		return "", fmt.Errorf("reading trailing CRLF: %w", err)
	}

	return string(payload), nil
}

// readFull reads exactly len(buf) bytes, looping if a single Read
// returns fewer bytes than requested (which bufio.Reader can do).
func readFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// Info sends the INFO command and returns Redis's raw text reply.
// Parsing that text into structured fields is deliberately a separate,
// later concern (Sprint 3) — this function's only job is proving the
// protocol round-trip works.
func (c *Client) Info() (string, error) {
	if err := c.sendCommand("INFO"); err != nil {
		return "", err
	}
	return c.readBulkString()
}
