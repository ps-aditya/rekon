package metrics

import "github.com/rekon/rekon/internal/redis"

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
