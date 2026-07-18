package metrics

import (
	"testing"

	"github.com/rekon/rekon/internal/redis"
)

func TestFilterOutSelf_RemovesMatchingAddr(t *testing.T) {
	entries := []redis.SlowlogEntry{
		{ID: 1, ClientAddr: "127.0.0.1:11111", Args: []string{"info"}},
		{ID: 2, ClientAddr: "127.0.0.1:22222", Args: []string{"set", "foo", "bar"}},
		{ID: 3, ClientAddr: "127.0.0.1:11111", Args: []string{"slowlog", "get", "25"}},
	}

	got := FilterOutSelf(entries, "127.0.0.1:11111")

	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0].ID != 2 {
		t.Errorf("got entry ID %d, want 2 (the non-self entry)", got[0].ID)
	}
}

func TestFilterOutSelf_EmptySelfAddrFiltersNothing(t *testing.T) {
	entries := []redis.SlowlogEntry{
		{ID: 1, ClientAddr: ""},
		{ID: 2, ClientAddr: "127.0.0.1:1"},
	}

	got := FilterOutSelf(entries, "")

	if len(got) != 2 {
		t.Errorf("got %d entries with empty selfAddr, want 2 (nothing filtered)", len(got))
	}
}

func TestFilterOutSelf_NoMatchesLeavesEverything(t *testing.T) {
	entries := []redis.SlowlogEntry{
		{ID: 1, ClientAddr: "127.0.0.1:1"},
		{ID: 2, ClientAddr: "127.0.0.1:2"},
	}

	got := FilterOutSelf(entries, "127.0.0.1:9999")

	if len(got) != 2 {
		t.Errorf("got %d entries, want 2 (selfAddr matched nothing)", len(got))
	}
}

func TestNewEntriesSince_NoPriorEntries(t *testing.T) {
	// lastSeenID=0 with real entries present — this is the "first ever
	// poll" shape from NewEntriesSince's own perspective. Whether the
	// caller (Model) treats this batch as "new" or as a baseline is a
	// decision that belongs to the caller, not this function — see the
	// doc comment. This test just confirms the function reports
	// everything above 0 as newer than 0, which is mechanically correct.
	entries := []redis.SlowlogEntry{{ID: 1}, {ID: 2}, {ID: 3}}
	newEntries, maxID := NewEntriesSince(entries, 0)

	if len(newEntries) != 3 {
		t.Errorf("got %d new entries, want 3", len(newEntries))
	}
	if maxID != 3 {
		t.Errorf("maxID: got %d, want 3", maxID)
	}
}

func TestNewEntriesSince_OnlyReturnsEntriesAboveLastSeen(t *testing.T) {
	entries := []redis.SlowlogEntry{{ID: 5}, {ID: 6}, {ID: 7}}
	newEntries, maxID := NewEntriesSince(entries, 5)

	if len(newEntries) != 2 {
		t.Fatalf("got %d new entries, want 2", len(newEntries))
	}
	if newEntries[0].ID != 6 || newEntries[1].ID != 7 {
		t.Errorf("got IDs %d, %d — want 6, 7", newEntries[0].ID, newEntries[1].ID)
	}
	if maxID != 7 {
		t.Errorf("maxID: got %d, want 7", maxID)
	}
}

func TestNewEntriesSince_NoNewEntries(t *testing.T) {
	entries := []redis.SlowlogEntry{{ID: 3}, {ID: 4}}
	newEntries, maxID := NewEntriesSince(entries, 4)

	if len(newEntries) != 0 {
		t.Errorf("got %d new entries, want 0", len(newEntries))
	}
	// maxID should still reflect the highest ID actually seen, even
	// though nothing was "new" — the caller needs this to stay
	// correctly positioned for the next poll.
	if maxID != 4 {
		t.Errorf("maxID: got %d, want 4 (unchanged from lastSeenID)", maxID)
	}
}

func TestNewEntriesSince_EmptySlowlog(t *testing.T) {
	newEntries, maxID := NewEntriesSince(nil, 10)
	if len(newEntries) != 0 {
		t.Errorf("got %d new entries for an empty slowlog, want 0", len(newEntries))
	}
	if maxID != 10 {
		t.Errorf("maxID: got %d, want unchanged 10", maxID)
	}
}

func TestNewEntriesSince_UnorderedInput(t *testing.T) {
	// Redis's real replies are ID-descending (newest first), not
	// ascending — confirm the function doesn't assume input ordering.
	entries := []redis.SlowlogEntry{{ID: 9}, {ID: 7}, {ID: 8}}
	newEntries, maxID := NewEntriesSince(entries, 7)

	if len(newEntries) != 2 {
		t.Fatalf("got %d new entries, want 2", len(newEntries))
	}
	if maxID != 9 {
		t.Errorf("maxID: got %d, want 9 (max regardless of input order)", maxID)
	}
}
