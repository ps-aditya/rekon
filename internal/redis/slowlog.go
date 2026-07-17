package redis

import (
	"fmt"
	"strconv"
	"time"
)

// SlowlogEntry is one entry from SLOWLOG GET, with fields typed and
// named instead of left as raw RESP Values.
type SlowlogEntry struct {
	ID             int64
	Timestamp      time.Time
	DurationMicros int64
	Args           []string
	ClientAddr     string // empty if Redis's reply didn't include it (older versions)
	ClientName     string // empty if Redis's reply didn't include it, or client had no name
}

// SlowlogGet sends SLOWLOG GET <count> and returns parsed entries.
//
// The reply is an Array of Arrays: each entry has at minimum
// [id, timestamp, duration_micros, command_args], with client address
// and client name as two further optional elements depending on Redis
// version. Both cases are handled — length is checked before indexing
// rather than assuming all 6 elements are always present.
func (c *Client) SlowlogGet(count int) ([]SlowlogEntry, error) {
	v, err := c.call("SLOWLOG", "GET", strconv.Itoa(count))
	if err != nil {
		return nil, err
	}
	if v.Type != TypeArray {
		return nil, fmt.Errorf("SLOWLOG GET: expected array reply, got type %v", v.Type)
	}

	entries := make([]SlowlogEntry, 0, len(v.Array))
	for _, e := range v.Array {
		if e.Type != TypeArray || len(e.Array) < 4 {
			// Defensive skip, not a hard failure — one malformed entry
			// shouldn't take down parsing of every other entry.
			continue
		}

		entry := SlowlogEntry{
			ID:             e.Array[0].Int,
			Timestamp:      time.Unix(e.Array[1].Int, 0),
			DurationMicros: e.Array[2].Int,
		}

		for _, argVal := range e.Array[3].Array {
			entry.Args = append(entry.Args, argVal.Str)
		}

		if len(e.Array) >= 5 {
			entry.ClientAddr = e.Array[4].Str
		}
		if len(e.Array) >= 6 {
			entry.ClientName = e.Array[5].Str
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// ClientList sends CLIENT LIST and returns Redis's raw text reply.
// Unlike SLOWLOG GET, CLIENT LIST's reply is a single bulk string
// (one client per line, space-separated key=value fields) — no array
// parsing needed. Structured parsing of that text is a separate
// concern (see package metrics), same split as Info()/ParseInfo.
func (c *Client) ClientList() (string, error) {
	v, err := c.call("CLIENT", "LIST")
	if err != nil {
		return "", err
	}
	if v.Type != TypeBulkString {
		return "", fmt.Errorf("CLIENT LIST: expected bulk string reply, got type %v", v.Type)
	}
	return v.Str, nil
}
