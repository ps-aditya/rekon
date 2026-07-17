// Package metrics converts Redis's raw INFO key-value strings into typed,
// domain-meaningful values — and where a threshold judgment is made
// (e.g. "this fragmentation ratio is concerning"), that reasoning is
// documented here explicitly rather than left as a magic number.
//
// These thresholds are commonly-cited operational heuristics from Redis
// operations experience, not universal laws or something Redis itself
// enforces. They're a reasonable default starting point for coloring a
// dashboard, not a claim that crossing them always means something is
// actually wrong for every workload.
package metrics

import "strconv"

// Status represents a simple traffic-light judgment about a metric,
// used to decide panel coloring in the View layer.
type Status int

const (
	StatusOK Status = iota
	StatusWarn
	StatusCritical
	StatusUnknown          // field was missing or unparseable from INFO
	StatusInsufficientData // field is present and valid, but the instance
	// doesn't hold enough data yet for this particular judgment to be
	// meaningful (distinct from StatusUnknown: the number exists, it's
	// just not trustworthy at this scale — see FragmentationStatus).
)

// MinMeaningfulMemoryBytes is the used_memory floor below which
// mem_fragmentation_ratio is treated as not-yet-meaningful rather than
// judged against the normal thresholds.
//
// This exists because of a real finding from manual testing (Sprint 3):
// a fresh, near-empty local Redis instance (used_memory around 650KB)
// reported a fragmentation ratio above 11 — correctly parsed, but not
// representative of an actual problem. At very low memory usage,
// allocator/RSS bookkeeping overhead dominates the ratio's denominator
// relative to the (tiny) amount of real data, producing large numbers
// that don't mean what they'd mean on an instance with real data
// volume.
//
// 5 MiB is a pragmatic default, not a value derived from a formal
// study — it's comfortably above the ~650KB that produced the
// misleading reading in testing, while still being small enough that
// most real Redis deployments cross it almost immediately. Revisit if
// real-world use shows it's set too high or too low.
const MinMeaningfulMemoryBytes int64 = 5 * 1024 * 1024

// Memory holds the memory-related fields Rekon's Memory panel needs.
type Memory struct {
	UsedMemoryBytes    int64
	FragmentationRatio float64
	MaxMemoryBytes     int64
	MaxMemoryPolicy    string
	EvictedKeys        int64
	parseErr           error
}

// ParseMemory extracts memory fields from an already-parsed INFO map
// (see redis.ParseInfo). Missing fields don't fail the whole parse —
// each field independently falls back to zero/unknown, since a single
// absent field (e.g. under an ACL-restricted INFO) shouldn't take down
// every other metric on the panel.
func ParseMemory(info map[string]string) Memory {
	m := Memory{}
	m.UsedMemoryBytes = parseInt(info["used_memory"])
	m.FragmentationRatio = parseFloat(info["mem_fragmentation_ratio"])
	m.MaxMemoryBytes = parseInt(info["maxmemory"])
	m.MaxMemoryPolicy = info["maxmemory_policy"]
	m.EvictedKeys = parseInt(info["evicted_keys"])
	return m
}

// FragmentationStatus judges mem_fragmentation_ratio using widely-cited
// Redis operational guidance:
//   - ratio < 1.0 means Redis's resident memory (RSS) is *less* than
//     what Redis believes it has allocated — a strong sign the OS has
//     swapped some of Redis's memory to disk, which is generally worse
//     than fragmentation itself (swapped Redis memory is drastically
//     slower to access).
//   - ratio > 1.5 is commonly flagged as significant fragmentation —
//     the allocator is holding much more resident memory than Redis's
//     logical data actually needs.
//   - Between 1.0 and 1.5 is the commonly-cited "normal" range.
//
// These are heuristic defaults for this dashboard's coloring, not a
// claim that every value outside this range is an active incident —
// documented here so the reasoning is inspectable and arguable, not a
// silent magic number.
func (m Memory) FragmentationStatus() Status {
	if m.FragmentationRatio == 0 {
		return StatusUnknown
	}
	if m.UsedMemoryBytes < MinMeaningfulMemoryBytes {
		return StatusInsufficientData
	}
	switch {
	case m.FragmentationRatio < 1.0:
		return StatusCritical
	case m.FragmentationRatio > 1.5:
		return StatusWarn
	default:
		return StatusOK
	}
}

// Ops holds the throughput/cache-effectiveness fields Rekon's Ops panel
// needs.
type Ops struct {
	OpsPerSec      int64
	KeyspaceHits   int64
	KeyspaceMisses int64
}

// ParseOps extracts ops fields from an already-parsed INFO map.
func ParseOps(info map[string]string) Ops {
	return Ops{
		OpsPerSec:      parseInt(info["instantaneous_ops_per_sec"]),
		KeyspaceHits:   parseInt(info["keyspace_hits"]),
		KeyspaceMisses: parseInt(info["keyspace_misses"]),
	}
}

// HitRatio returns the keyspace hit ratio (hits / (hits + misses)) as a
// fraction from 0.0 to 1.0. Returns 0 with ok=false if there's no data
// yet to compute a ratio from (both hits and misses are zero) — this is
// distinct from an actual 0% hit ratio, and callers should treat it as
// "not enough data" rather than "cache is failing."
func (o Ops) HitRatio() (ratio float64, ok bool) {
	total := o.KeyspaceHits + o.KeyspaceMisses
	if total == 0 {
		return 0, false
	}
	return float64(o.KeyspaceHits) / float64(total), true
}

// HitRatioStatus judges the hit ratio against a commonly-cited (but
// genuinely workload-dependent) rule of thumb: below 80% is often worth
// a second look. This is explicitly softer/less confident guidance than
// the fragmentation thresholds above — a low hit ratio can be entirely
// expected for some access patterns (e.g. mostly-write workloads), so
// this status is informational, not alarming, by design.
func (o Ops) HitRatioStatus() Status {
	ratio, ok := o.HitRatio()
	if !ok {
		return StatusUnknown
	}
	if ratio < 0.80 {
		return StatusWarn
	}
	return StatusOK
}

func parseInt(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
