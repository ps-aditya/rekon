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

// call sends a command and reads back exactly one RESP value — the
// generic building block Info, SlowlogGet, and ClientList are all built
// on top of, instead of each hand-rolling their own read logic.
func (c *Client) call(args ...string) (Value, error) {
	if err := c.sendCommand(args...); err != nil {
		return Value{}, err
	}
	return readValue(c.reader)
}

// Info sends the INFO command and returns Redis's raw text reply.
// Parsing that text into structured fields is a separate concern
// (see package metrics) — this function's only job is the protocol
// round-trip and confirming the reply was actually a bulk string.
func (c *Client) Info() (string, error) {
	v, err := c.call("INFO")
	if err != nil {
		return "", err
	}
	if v.Type != TypeBulkString {
		return "", fmt.Errorf("INFO: expected bulk string reply, got type %v", v.Type)
	}
	return v.Str, nil
}
