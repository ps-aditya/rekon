package metrics

import (
	"fmt"
	"strings"
)

// SlaveInfo is one connected replica, as reported on a master via the
// slaveN:ip=...,port=...,state=...,offset=...,lag=... INFO field.
// Redis's own field name is "slave" (legacy terminology); Rekon keeps
// that name internally to match the wire format exactly, but nothing
// user-facing needs to use it.
type SlaveInfo struct {
	IP     string
	Port   string
	State  string
	Offset int64
	Lag    int64
}

// Replication holds the fields Rekon's Replication panel needs, shaped
// to work for whichever role the watched instance actually has.
//
// Only one of two "modes" is ever meaningful at once: a master instance
// populates ConnectedSlaves/Slaves and leaves the Master* fields zero;
// a replica instance populates MasterHost/MasterLinkStatus/etc and
// leaves Slaves empty. A standalone instance (no replication configured
// at all) is just a master with zero connected slaves — Redis doesn't
// have a separate "standalone" role value; "master" covers both.
type Replication struct {
	Role                   string
	ConnectedSlaves        int64
	Slaves                 []SlaveInfo
	MasterHost             string
	MasterPort             string
	MasterLinkStatus       string
	MasterLastIOSecondsAgo int64
}

// IsReplica reports whether the watched instance is itself a replica
// of another instance (role:slave in Redis's own INFO terminology).
func (r Replication) IsReplica() bool {
	return r.Role == "slave"
}

// ParseReplication extracts replication fields from an already-parsed
// INFO map. The slaveN fields (one per connected replica, N starting
// at 0) are Redis's own comma-separated key=value sub-format nested
// inside a single INFO line — a second, smaller parse step beyond
// ParseInfo's key:value splitting.
func ParseReplication(info map[string]string) Replication {
	r := Replication{
		Role:                   info["role"],
		ConnectedSlaves:        parseInt(info["connected_slaves"]),
		MasterHost:             info["master_host"],
		MasterPort:             info["master_port"],
		MasterLinkStatus:       info["master_link_status"],
		MasterLastIOSecondsAgo: parseInt(info["master_last_io_seconds_ago"]),
	}

	for i := int64(0); i < r.ConnectedSlaves; i++ {
		key := fmt.Sprintf("slave%d", i)
		raw, ok := info[key]
		if !ok {
			continue
		}
		r.Slaves = append(r.Slaves, parseSlaveField(raw))
	}

	return r
}

// parseSlaveField parses one "ip=...,port=...,state=...,offset=...,lag=..."
// value into a SlaveInfo.
func parseSlaveField(raw string) SlaveInfo {
	fields := make(map[string]string)
	for _, token := range strings.Split(raw, ",") {
		key, value, found := strings.Cut(token, "=")
		if !found {
			continue
		}
		fields[key] = value
	}

	return SlaveInfo{
		IP:     fields["ip"],
		Port:   fields["port"],
		State:  fields["state"],
		Offset: parseInt(fields["offset"]),
		Lag:    parseInt(fields["lag"]),
	}
}

// ReplicaLinkStatus judges a replica's link to its master. Only
// meaningful when IsReplica() is true — callers should check that
// first, since this returns StatusUnknown for a non-replica instance
// where the concept doesn't apply, not because data is missing.
//
// "down" is judged StatusCritical rather than StatusWarn: a replica
// not receiving updates from its master means it's serving
// increasingly stale data, which is a genuine operational problem, not
// a borderline one.
func (r Replication) ReplicaLinkStatus() Status {
	if !r.IsReplica() {
		return StatusUnknown
	}
	if r.MasterLinkStatus == "up" {
		return StatusOK
	}
	return StatusCritical
}
