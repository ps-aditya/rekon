package metrics

// Persistence holds the fields Rekon's Persistence panel needs, from
// INFO's Persistence section, covering both RDB (point-in-time
// snapshot) and AOF (append-only log) mechanisms — a Redis instance
// can have either, both, or neither enabled, and this struct is shaped
// to represent all three cases without assuming one is always active.
type Persistence struct {
	RDBLastSaveTime         int64 // Unix timestamp of the last successful RDB save
	RDBChangesSinceLastSave int64
	RDBBGSaveInProgress     bool
	RDBLastBGSaveStatus     string // "ok" or "err"

	AOFEnabled             bool
	AOFRewriteInProgress   bool
	AOFLastBGRewriteStatus string // "ok" or "err"
	AOFLastWriteStatus     string // "ok" or "err"
}

// ParsePersistence extracts persistence fields from an already-parsed
// INFO map.
func ParsePersistence(info map[string]string) Persistence {
	return Persistence{
		RDBLastSaveTime:         parseInt(info["rdb_last_save_time"]),
		RDBChangesSinceLastSave: parseInt(info["rdb_changes_since_last_save"]),
		RDBBGSaveInProgress:     info["rdb_bgsave_in_progress"] == "1",
		RDBLastBGSaveStatus:     info["rdb_last_bgsave_status"],

		AOFEnabled:             info["aof_enabled"] == "1",
		AOFRewriteInProgress:   info["aof_rewrite_in_progress"] == "1",
		AOFLastBGRewriteStatus: info["aof_last_bgrewrite_status"],
		AOFLastWriteStatus:     info["aof_last_write_status"],
	}
}

// Status judges overall persistence health: a failed RDB save or a
// failed AOF write are both treated as StatusCritical — either one
// means Redis's durability guarantee is currently broken, which is a
// serious operational concern regardless of which mechanism failed.
// AOF fields are only considered when AOFEnabled is true, since a
// disabled AOF's last-write-status is meaningless leftover state, not
// a current problem.
func (p Persistence) Status() Status {
	if p.RDBLastBGSaveStatus == "err" {
		return StatusCritical
	}
	if p.AOFEnabled && (p.AOFLastWriteStatus == "err" || p.AOFLastBGRewriteStatus == "err") {
		return StatusCritical
	}
	if p.RDBBGSaveInProgress || p.AOFRewriteInProgress {
		return StatusWarn // not broken, but worth flagging as "in progress"
	}
	return StatusOK
}
