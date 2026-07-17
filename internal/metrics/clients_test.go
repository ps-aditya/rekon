package metrics

import "testing"

// realClientListSample is a real line captured from CLIENT LIST against
// a live local Redis instance.
const realClientListSample = "id=9 addr=127.0.0.1:35358 laddr=127.0.0.1:6379 fd=7 name= age=0 idle=0 flags=N db=0 sub=0 psub=0 ssub=0 multi=-1 qbuf=26 qbuf-free=20448 argv-mem=10 multi-mem=0 rbs=16384 rbp=16384 obl=0 oll=0 omem=0 tot-mem=37658 events=r cmd=client|list user=default redir=-1 resp=2\n"

func TestParseClientList_RealSample(t *testing.T) {
	records := ParseClientList(realClientListSample)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	if r.ID != "9" {
		t.Errorf("ID: got %q, want 9", r.ID)
	}
	if r.Addr != "127.0.0.1:35358" {
		t.Errorf("Addr: got %q, want 127.0.0.1:35358", r.Addr)
	}
	if r.Name != "" {
		t.Errorf("Name: got %q, want empty", r.Name)
	}
	if r.IdleSeconds != 0 {
		t.Errorf("IdleSeconds: got %d, want 0", r.IdleSeconds)
	}
}

func TestParseClientList_MultipleClients(t *testing.T) {
	raw := "id=1 addr=127.0.0.1:1 name=alpha idle=0\n" +
		"id=2 addr=127.0.0.1:2 name= idle=500\n"

	records := ParseClientList(raw)
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].Name != "alpha" {
		t.Errorf("records[0].Name: got %q, want alpha", records[0].Name)
	}
	if records[1].IdleSeconds != 500 {
		t.Errorf("records[1].IdleSeconds: got %d, want 500", records[1].IdleSeconds)
	}
}

func TestParseClientList_EmptyInput(t *testing.T) {
	records := ParseClientList("")
	if len(records) != 0 {
		t.Errorf("got %d records for empty input, want 0", len(records))
	}
}

func TestLongIdleClients_ThresholdBoundary(t *testing.T) {
	records := []ClientRecord{
		{ID: "1", IdleSeconds: LongIdleThresholdSeconds - 1}, // just under
		{ID: "2", IdleSeconds: LongIdleThresholdSeconds},     // exactly at
		{ID: "3", IdleSeconds: LongIdleThresholdSeconds + 1}, // just over
		{ID: "4", IdleSeconds: 0},                            // active
	}

	got := LongIdleClients(records)
	if len(got) != 2 {
		t.Fatalf("got %d long-idle clients, want 2", len(got))
	}
	if got[0].ID != "2" || got[1].ID != "3" {
		t.Errorf("got IDs %s, %s — want 2, 3 (threshold is inclusive)", got[0].ID, got[1].ID)
	}
}

func TestParseClients_ExtractsCounts(t *testing.T) {
	info := map[string]string{
		"connected_clients": "5",
		"blocked_clients":   "1",
	}
	c := ParseClients(info)
	if c.Connected != 5 {
		t.Errorf("Connected: got %d, want 5", c.Connected)
	}
	if c.Blocked != 1 {
		t.Errorf("Blocked: got %d, want 1", c.Blocked)
	}
}
