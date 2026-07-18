package metrics

import "testing"

func TestParseReplication_StandaloneMaster(t *testing.T) {
	// Real captured INFO replication section from a fresh standalone
	// Redis instance (no replicas connected).
	info := map[string]string{
		"role":               "master",
		"connected_slaves":   "0",
		"master_repl_offset": "0",
	}

	r := ParseReplication(info)

	if r.Role != "master" {
		t.Errorf("Role: got %q, want master", r.Role)
	}
	if r.IsReplica() {
		t.Error("a standalone master should not report IsReplica() true")
	}
	if r.ConnectedSlaves != 0 {
		t.Errorf("ConnectedSlaves: got %d, want 0", r.ConnectedSlaves)
	}
	if len(r.Slaves) != 0 {
		t.Errorf("Slaves: got %d entries, want 0", len(r.Slaves))
	}
	if r.ReplicaLinkStatus() != StatusUnknown {
		t.Errorf("ReplicaLinkStatus on a non-replica: got %v, want StatusUnknown", r.ReplicaLinkStatus())
	}
}

func TestParseReplication_MasterWithConnectedReplica(t *testing.T) {
	// Real captured INFO replication section from a master with one
	// real connected replica.
	info := map[string]string{
		"role":             "master",
		"connected_slaves": "1",
		"slave0":           "ip=127.0.0.1,port=6380,state=online,offset=1,lag=0",
	}

	r := ParseReplication(info)

	if r.ConnectedSlaves != 1 {
		t.Fatalf("ConnectedSlaves: got %d, want 1", r.ConnectedSlaves)
	}
	if len(r.Slaves) != 1 {
		t.Fatalf("Slaves: got %d entries, want 1", len(r.Slaves))
	}

	s := r.Slaves[0]
	if s.IP != "127.0.0.1" {
		t.Errorf("Slaves[0].IP: got %q, want 127.0.0.1", s.IP)
	}
	if s.Port != "6380" {
		t.Errorf("Slaves[0].Port: got %q, want 6380", s.Port)
	}
	if s.State != "online" {
		t.Errorf("Slaves[0].State: got %q, want online", s.State)
	}
	if s.Lag != 0 {
		t.Errorf("Slaves[0].Lag: got %d, want 0", s.Lag)
	}
}

func TestParseReplication_ReplicaRoleLinkUp(t *testing.T) {
	// Real captured INFO replication section from the replica side,
	// after the link came up and data actually replicated.
	info := map[string]string{
		"role":               "slave",
		"master_host":        "127.0.0.1",
		"master_port":        "6379",
		"master_link_status": "up",
		"connected_slaves":   "0",
	}

	r := ParseReplication(info)

	if !r.IsReplica() {
		t.Error("role:slave should report IsReplica() true")
	}
	if r.MasterHost != "127.0.0.1" {
		t.Errorf("MasterHost: got %q, want 127.0.0.1", r.MasterHost)
	}
	if r.ReplicaLinkStatus() != StatusOK {
		t.Errorf("ReplicaLinkStatus with link up: got %v, want StatusOK", r.ReplicaLinkStatus())
	}
}

func TestParseReplication_ReplicaRoleLinkDown(t *testing.T) {
	// Real captured transient state: immediately after starting a
	// replica, before the initial sync completed.
	info := map[string]string{
		"role":               "slave",
		"master_host":        "127.0.0.1",
		"master_port":        "6379",
		"master_link_status": "down",
	}

	r := ParseReplication(info)

	if r.ReplicaLinkStatus() != StatusCritical {
		t.Errorf("ReplicaLinkStatus with link down: got %v, want StatusCritical", r.ReplicaLinkStatus())
	}
}
