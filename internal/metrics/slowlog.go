package metrics

import "github.com/rekon/rekon/internal/redis"

// FilterOutSelf removes slowlog entries whose ClientAddr matches
// selfAddr — Rekon's own connection's local address, as reported by
// redis.Client.LocalAddr(). Redis records the connecting side's
// address for both CLIENT LIST and SLOWLOG entries, so Rekon's own
// polling commands (INFO, CLIENT LIST, SLOWLOG GET) show up under its
// own connection's address exactly like any other client's commands
// would.
//
// This is the fix for the Sprint 4 finding: under an aggressive
// slowlog-log-slower-than setting, Rekon's own polling was appearing
// in the Slowlog panel meant to show the *watched* instance's
// activity. Filtering by address (not by command name) is deliberate —
// filtering by name would also hide a real other client legitimately
// issuing the same commands, which is signal a user might actually
// want to see.
func FilterOutSelf(entries []redis.SlowlogEntry, selfAddr string) []redis.SlowlogEntry {
	if selfAddr == "" {
		// No self address known (e.g. LocalAddr unavailable) — don't
		// filter anything rather than risk hiding real entries on an
		// empty-string coincidental match.
		return entries
	}

	filtered := make([]redis.SlowlogEntry, 0, len(entries))
	for _, e := range entries {
		if e.ClientAddr == selfAddr {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

// NewEntriesSince returns the entries from a fresh SLOWLOG GET poll
// whose ID is greater than lastSeenID, plus the highest ID seen in this
// batch (for the caller to remember as the new lastSeenID going into
// the next poll).
//
// This relies on Redis's own guarantee that slowlog entry IDs are
// assigned as a monotonically increasing counter — a higher ID always
// means a more recent entry, regardless of polling timing.
//
// Deliberately a pure function taking/returning plain values rather
// than a stateful type: the actual "remember lastSeenID across polls"
// state belongs to whatever owns program lifecycle (the Model), not to
// this parsing layer. This keeps the interesting logic (which entries
// count as new) unit-testable without any bubbletea/UI machinery.
func NewEntriesSince(entries []redis.SlowlogEntry, lastSeenID int64) (newEntries []redis.SlowlogEntry, maxID int64) {
	maxID = lastSeenID
	for _, e := range entries {
		if e.ID > lastSeenID {
			newEntries = append(newEntries, e)
		}
		if e.ID > maxID {
			maxID = e.ID
		}
	}
	return newEntries, maxID
}
