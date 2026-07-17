package metrics

import "testing"

func TestParseMemory_ExtractsFields(t *testing.T) {
	info := map[string]string{
		"used_memory":             "635840",
		"mem_fragmentation_ratio": "1.07",
		"maxmemory":               "0",
		"maxmemory_policy":        "noeviction",
		"evicted_keys":            "3",
	}
	m := ParseMemory(info)

	if m.UsedMemoryBytes != 635840 {
		t.Errorf("UsedMemoryBytes: got %d, want 635840", m.UsedMemoryBytes)
	}
	if m.FragmentationRatio != 1.07 {
		t.Errorf("FragmentationRatio: got %f, want 1.07", m.FragmentationRatio)
	}
	if m.MaxMemoryPolicy != "noeviction" {
		t.Errorf("MaxMemoryPolicy: got %q, want noeviction", m.MaxMemoryPolicy)
	}
	if m.EvictedKeys != 3 {
		t.Errorf("EvictedKeys: got %d, want 3", m.EvictedKeys)
	}
}

func TestParseMemory_MissingFieldsDefaultToZero(t *testing.T) {
	// An ACL-restricted or partial INFO reply shouldn't panic or error —
	// missing fields should just come back as zero values.
	m := ParseMemory(map[string]string{})

	if m.UsedMemoryBytes != 0 || m.FragmentationRatio != 0 {
		t.Errorf("expected zero values for missing fields, got %+v", m)
	}
}

func TestFragmentationStatus_Boundaries(t *testing.T) {
	// All cases here use UsedMemoryBytes above MinMeaningfulMemoryBytes,
	// so the insufficient-data gate doesn't interfere with testing the
	// ratio thresholds themselves.
	const wellAboveFloor = MinMeaningfulMemoryBytes * 2

	cases := []struct {
		name  string
		ratio float64
		want  Status
	}{
		{"zero (unparseable/missing)", 0, StatusUnknown},
		{"below 1.0 -> likely swapping -> critical", 0.95, StatusCritical},
		{"exactly 1.0 -> ok", 1.0, StatusOK},
		{"normal mid-range -> ok", 1.2, StatusOK},
		{"exactly 1.5 -> still ok (boundary is exclusive)", 1.5, StatusOK},
		{"above 1.5 -> warn", 1.6, StatusWarn},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Memory{FragmentationRatio: c.ratio, UsedMemoryBytes: wellAboveFloor}
			got := m.FragmentationStatus()
			if got != c.want {
				t.Errorf("ratio %.2f: got status %v, want %v", c.ratio, got, c.want)
			}
		})
	}
}

func TestFragmentationStatus_InsufficientDataOnRealCapturedScenario(t *testing.T) {
	// This is the exact scenario from Sprint 3 manual testing that
	// surfaced the issue: a fresh, near-empty Redis instance reported
	// used_memory ~650KB and a fragmentation ratio above 11. Before this
	// fix, that produced StatusWarn — a misleading signal on an instance
	// with no real problem. It must now return StatusInsufficientData.
	m := Memory{FragmentationRatio: 11.78, UsedMemoryBytes: 650840}
	got := m.FragmentationStatus()
	if got != StatusInsufficientData {
		t.Errorf("got %v, want StatusInsufficientData for a near-idle instance", got)
	}
}

func TestFragmentationStatus_FloorBoundary(t *testing.T) {
	cases := []struct {
		name        string
		usedMemory  int64
		wantInsuff  bool
		description string
	}{
		{"just below floor", MinMeaningfulMemoryBytes - 1, true, "one byte under the floor is still insufficient"},
		{"exactly at floor", MinMeaningfulMemoryBytes, false, "at the floor, normal thresholds apply"},
		{"well above floor", MinMeaningfulMemoryBytes * 10, false, "comfortably above, normal thresholds apply"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Use a ratio that would be StatusWarn under normal
			// thresholds, so the test actually distinguishes "gated as
			// insufficient" from "happens to also be OK".
			m := Memory{FragmentationRatio: 1.6, UsedMemoryBytes: c.usedMemory}
			got := m.FragmentationStatus()
			isInsuff := got == StatusInsufficientData
			if isInsuff != c.wantInsuff {
				t.Errorf("%s: got status %v (insufficient=%v), want insufficient=%v",
					c.description, got, isInsuff, c.wantInsuff)
			}
		})
	}
}

func TestHitRatio_NoDataYet(t *testing.T) {
	o := Ops{KeyspaceHits: 0, KeyspaceMisses: 0}
	_, ok := o.HitRatio()
	if ok {
		t.Error("expected ok=false when there's no hit/miss data yet")
	}
	if o.HitRatioStatus() != StatusUnknown {
		t.Errorf("expected StatusUnknown with no data, got %v", o.HitRatioStatus())
	}
}

func TestHitRatio_ComputesCorrectly(t *testing.T) {
	o := Ops{KeyspaceHits: 80, KeyspaceMisses: 20}
	ratio, ok := o.HitRatio()
	if !ok {
		t.Fatal("expected ok=true with real hit/miss data")
	}
	if ratio != 0.8 {
		t.Errorf("got ratio %.2f, want 0.80", ratio)
	}
	// Exactly 0.80 is the boundary — documented as still StatusOK
	// (threshold is "below 80%", not "80% or below").
	if o.HitRatioStatus() != StatusOK {
		t.Errorf("expected StatusOK at exactly the 80%% boundary, got %v", o.HitRatioStatus())
	}
}

func TestHitRatio_BelowThresholdWarns(t *testing.T) {
	o := Ops{KeyspaceHits: 70, KeyspaceMisses: 30}
	if o.HitRatioStatus() != StatusWarn {
		t.Errorf("expected StatusWarn at 70%% hit ratio, got %v", o.HitRatioStatus())
	}
}
