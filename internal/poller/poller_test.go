package poller

import (
	"testing"
	"time"

	"github.com/rekon/rekon/internal/redis"
)

// TestStop_SafeToCallTwice proves the sync.Once fix: a second Stop()
// call must not panic on an already-closed channel.
func TestStop_SafeToCallTwice(t *testing.T) {
	// A Poller with a nil client never actually polls in this test
	// (Start isn't called), so this only exercises Stop()'s own
	// guard logic, not real Redis communication.
	p := New(&redis.Client{}, time.Second)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("calling Stop() twice panicked: %v", r)
		}
	}()

	p.Stop()
	p.Stop() // must not panic
}
