package metrics

import "strings"

// ClientRecord is one line of CLIENT LIST's output, parsed into the
// fields Rekon's Clients panel actually needs. CLIENT LIST returns many
// more fields per client (fd, qbuf, multi-mem, etc) — only the ones
// used for display/flagging are extracted here; irrelevant fields are
// simply not kept, not an oversight.
type ClientRecord struct {
	ID          string
	Addr        string
	Name        string
	IdleSeconds int64
}

// ParseClientList parses CLIENT LIST's raw text: one client per line,
// space-separated key=value fields, e.g.
//
//	id=9 addr=127.0.0.1:35358 ... name= age=0 idle=0 flags=N ...
//
// A line that doesn't parse cleanly is skipped rather than failing the
// whole list — one malformed or unexpected line (e.g. from a future
// Redis version adding new fields) shouldn't hide every other client.
func ParseClientList(raw string) []ClientRecord {
	var records []ClientRecord

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := make(map[string]string)
		for _, token := range strings.Fields(line) {
			key, value, found := strings.Cut(token, "=")
			if !found {
				continue
			}
			fields[key] = value
		}

		if _, ok := fields["id"]; !ok {
			// Not a recognizable client line — skip defensively.
			continue
		}

		records = append(records, ClientRecord{
			ID:          fields["id"],
			Addr:        fields["addr"],
			Name:        fields["name"],
			IdleSeconds: parseInt(fields["idle"]),
		})
	}

	return records
}

// LongIdleThresholdSeconds is the idle time above which a client
// connection is flagged in the panel. 300 seconds (5 minutes) is a
// pragmatic default — long enough that normal request/response gaps
// don't trigger it, short enough to catch connections that are
// plausibly forgotten/leaked rather than actively idle-but-pooled.
// Like the fragmentation ratio threshold, this is a heuristic starting
// point to argue about, not a value derived from a formal study.
const LongIdleThresholdSeconds int64 = 300

// LongIdleClients filters records to those at or above
// LongIdleThresholdSeconds of idle time.
func LongIdleClients(records []ClientRecord) []ClientRecord {
	var out []ClientRecord
	for _, r := range records {
		if r.IdleSeconds >= LongIdleThresholdSeconds {
			out = append(out, r)
		}
	}
	return out
}

// Clients holds the connection-count fields Rekon's Clients panel
// needs, sourced from INFO rather than by counting CLIENT LIST lines —
// INFO's connected_clients/blocked_clients are Redis's own authoritative
// counters, simpler and more reliable than re-deriving counts from a
// separately-fetched client list that could theoretically be
// momentarily inconsistent with INFO's snapshot.
type Clients struct {
	Connected int64
	Blocked   int64
}

// ParseClients extracts connection counts from an already-parsed INFO
// map (see redis.ParseInfo).
func ParseClients(info map[string]string) Clients {
	return Clients{
		Connected: parseInt(info["connected_clients"]),
		Blocked:   parseInt(info["blocked_clients"]),
	}
}
