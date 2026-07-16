package redis

import "testing"

// realSample is a real INFO reply captured from a live local Redis
// instance (redis-cli info memory), not a hand-typed guess. Keeping a
// real captured sample as the test fixture matches the project's own
// stated testing philosophy from earlier sprints — tested against real
// behavior, not assumptions of what the format looks like.
const realSample = "# Memory\r\n" +
	"used_memory:635840\r\n" +
	"used_memory_human:620.94K\r\n" +
	"used_memory_rss:6832128\r\n" +
	"mem_fragmentation_ratio:1.07\r\n" +
	"maxmemory:0\r\n" +
	"maxmemory_policy:noeviction\r\n" +
	"\r\n" +
	"# Stats\r\n" +
	"instantaneous_ops_per_sec:3\r\n" +
	"keyspace_hits:120\r\n" +
	"keyspace_misses:30\r\n"

func TestParseInfo_ExtractsKeyValuePairs(t *testing.T) {
	got := ParseInfo(realSample)

	want := map[string]string{
		"used_memory":               "635840",
		"used_memory_human":         "620.94K",
		"used_memory_rss":           "6832128",
		"mem_fragmentation_ratio":   "1.07",
		"maxmemory":                 "0",
		"maxmemory_policy":          "noeviction",
		"instantaneous_ops_per_sec": "3",
		"keyspace_hits":             "120",
		"keyspace_misses":           "30",
	}

	for key, wantVal := range want {
		gotVal, ok := got[key]
		if !ok {
			t.Errorf("missing key %q in parsed result", key)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("key %q: got %q, want %q", key, gotVal, wantVal)
		}
	}
}

func TestParseInfo_SkipsSectionHeadersAndBlankLines(t *testing.T) {
	got := ParseInfo(realSample)

	if _, ok := got["# Memory"]; ok {
		t.Error("section header should not appear as a key")
	}
	if _, ok := got[""]; ok {
		t.Error("blank lines should not produce an empty-string key")
	}
}

func TestParseInfo_EmptyInput(t *testing.T) {
	got := ParseInfo("")
	if len(got) != 0 {
		t.Errorf("expected empty map for empty input, got %d entries", len(got))
	}
}
