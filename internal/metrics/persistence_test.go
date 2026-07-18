package metrics

import "testing"

func TestParsePersistence_RealBaselineCapture(t *testing.T) {
	// Real captured INFO persistence section from a fresh standalone
	// Redis instance: RDB present, AOF disabled, no errors, nothing
	// in progress.
	info := map[string]string{
		"rdb_changes_since_last_save": "0",
		"rdb_bgsave_in_progress":      "0",
		"rdb_last_save_time":          "1784308233",
		"rdb_last_bgsave_status":      "ok",
		"aof_enabled":                 "0",
		"aof_rewrite_in_progress":     "0",
		"aof_last_bgrewrite_status":   "ok",
		"aof_last_write_status":       "ok",
	}

	p := ParsePersistence(info)

	if p.RDBLastSaveTime != 1784308233 {
		t.Errorf("RDBLastSaveTime: got %d, want 1784308233", p.RDBLastSaveTime)
	}
	if p.RDBLastBGSaveStatus != "ok" {
		t.Errorf("RDBLastBGSaveStatus: got %q, want ok", p.RDBLastBGSaveStatus)
	}
	if p.AOFEnabled {
		t.Error("AOFEnabled: got true, want false for this capture")
	}
	if p.Status() != StatusOK {
		t.Errorf("Status: got %v, want StatusOK for a healthy baseline", p.Status())
	}
}

func TestPersistenceStatus_RDBSaveFailure(t *testing.T) {
	p := Persistence{RDBLastBGSaveStatus: "err"}
	if p.Status() != StatusCritical {
		t.Errorf("got %v, want StatusCritical for a failed RDB save", p.Status())
	}
}

func TestPersistenceStatus_AOFWriteFailureOnlyMattersIfEnabled(t *testing.T) {
	// AOF disabled: a stale/leftover "err" in aof_last_write_status
	// shouldn't be treated as a current problem.
	disabled := Persistence{AOFEnabled: false, AOFLastWriteStatus: "err"}
	if disabled.Status() != StatusOK {
		t.Errorf("disabled AOF with stale err status: got %v, want StatusOK", disabled.Status())
	}

	// AOF enabled: the same "err" now genuinely matters.
	enabled := Persistence{AOFEnabled: true, AOFLastWriteStatus: "err"}
	if enabled.Status() != StatusCritical {
		t.Errorf("enabled AOF with err status: got %v, want StatusCritical", enabled.Status())
	}
}

func TestPersistenceStatus_InProgressIsWarnNotCritical(t *testing.T) {
	p := Persistence{RDBLastBGSaveStatus: "ok", RDBBGSaveInProgress: true}
	if p.Status() != StatusWarn {
		t.Errorf("got %v, want StatusWarn for an in-progress save (not broken, just active)", p.Status())
	}
}
