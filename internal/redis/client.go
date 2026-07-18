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
	addr   string // remembered for Reconnect
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
		addr:   addr,
	}, nil
}

// LocalAddr returns this connection's local address as a string (e.g.
// "127.0.0.1:54012") — the same address Redis's own CLIENT LIST and
// SLOWLOG entries will report for commands issued over this
// connection, since Redis records the connecting side's address as it
// sees it. Used to distinguish Rekon's own traffic from a real client's
// traffic when watching CLIENT LIST/SLOWLOG for other activity — see
// TECHNICAL_DEBT.md's Sprint 4 entry on self-polling noise.
func (c *Client) LocalAddr() string {
	return c.conn.LocalAddr().String()
}

// Reconnect closes the current connection (if any) and redials the
// original address, replacing conn and reader. Used to recover after a
// poll fails due to a dropped connection — see poller.go's pollOnce.
func (c *Client) Reconnect() error {
	if c.conn != nil {
		c.conn.Close() // best-effort; the connection is already presumed broken
	}
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("reconnecting to redis at %s: %w", c.addr, err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
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
// call sends a command and reads back exactly one RESP value — the
// generic building block Info, SlowlogGet, and ClientList are all built
// on top of, instead of each hand-rolling their own read logic.
//
// If Redis replies with a RESP Error (e.g. a NOPERM from an ACL
// restriction), that error's actual message is returned directly —
// callers checking for a specific expected type (bulk string, array)
// don't need to separately handle the Error case themselves; they'd
// otherwise end up reporting a confusing "expected X, got type Error"
// instead of the genuinely useful message Redis provided.
func (c *Client) call(args ...string) (Value, error) {
	if err := c.sendCommand(args...); err != nil {
		return Value{}, err
	}
	v, err := readValue(c.reader)
	if err != nil {
		return Value{}, err
	}
	if v.Type == TypeError {
		return Value{}, fmt.Errorf("redis error: %s", v.Str)
	}
	return v, nil
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
