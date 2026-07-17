package redis

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ValueType identifies which RESP reply type a Value holds. INFO only
// ever produces BulkString (Sprint 0-3 didn't need this distinction),
// but SLOWLOG GET's reply is an Array of Arrays mixing Integer, Array,
// and BulkString — hence a real tagged type is needed now.
type ValueType int

const (
	TypeBulkString ValueType = iota
	TypeArray
	TypeInteger
	TypeSimpleString
	TypeError
)

// Value is one RESP reply value, of whichever type it turned out to be.
// Only the field matching Type is meaningful; the others are zero.
type Value struct {
	Type  ValueType
	Str   string  // BulkString, SimpleString, Error
	Int   int64   // Integer
	Array []Value // Array — elements can themselves be any type, including nested Arrays
}

// readValue reads exactly one RESP value from r, dispatching on the
// leading type byte. This replaces Sprint 0's readBulkString, which
// only handled the '$' case — that logic now lives here as one case
// among several.
func readValue(r *bufio.Reader) (Value, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return Value{}, fmt.Errorf("reading reply header: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return Value{}, fmt.Errorf("empty reply header")
	}

	switch line[0] {
	case '$':
		return readBulkStringBody(r, line)
	case '*':
		return readArrayBody(r, line)
	case ':':
		n, err := strconv.ParseInt(line[1:], 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("parsing integer reply %q: %w", line, err)
		}
		return Value{Type: TypeInteger, Int: n}, nil
	case '+':
		return Value{Type: TypeSimpleString, Str: line[1:]}, nil
	case '-':
		return Value{Type: TypeError, Str: line[1:]}, nil
	default:
		return Value{}, fmt.Errorf("unrecognized RESP type byte %q in %q", line[0], line)
	}
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

// readBulkStringBody reads the payload for a '$<length>' header already
// consumed into line. Same mechanism as Sprint 0's readBulkString: read
// exactly length bytes, then discard the trailing CRLF.
func readBulkStringBody(r *bufio.Reader, line string) (Value, error) {
	length, err := strconv.Atoi(line[1:])
	if err != nil {
		return Value{}, fmt.Errorf("parsing bulk string length from %q: %w", line, err)
	}
	if length < 0 {
		// $-1 is RESP's nil bulk string.
		return Value{Type: TypeBulkString, Str: ""}, nil
	}

	payload := make([]byte, length)
	if _, err := readFull(r, payload); err != nil {
		return Value{}, fmt.Errorf("reading bulk string payload: %w", err)
	}
	if _, err := r.Discard(2); err != nil {
		return Value{}, fmt.Errorf("reading trailing CRLF: %w", err)
	}

	return Value{Type: TypeBulkString, Str: string(payload)}, nil
}

// readArrayBody reads the elements for a '*<count>' header already
// consumed into line. Each element is read with a recursive readValue
// call — this is what makes nested arrays (SLOWLOG's per-entry arrays
// inside the top-level array) work with no special-casing.
func readArrayBody(r *bufio.Reader, line string) (Value, error) {
	count, err := strconv.Atoi(line[1:])
	if err != nil {
		return Value{}, fmt.Errorf("parsing array count from %q: %w", line, err)
	}
	if count < 0 {
		// *-1 is RESP's nil array.
		return Value{Type: TypeArray, Array: nil}, nil
	}

	elements := make([]Value, count)
	for i := 0; i < count; i++ {
		v, err := readValue(r)
		if err != nil {
			return Value{}, fmt.Errorf("reading array element %d of %d: %w", i, count, err)
		}
		elements[i] = v
	}

	return Value{Type: TypeArray, Array: elements}, nil
}
