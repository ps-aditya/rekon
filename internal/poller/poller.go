// Package poller runs a Redis client on a timer in its own goroutine and
// delivers results over a channel, decoupled from whatever consumes them.
//
// This is Sprint 1's core proof: a slow or stalled poll must never block
// anything else in the program. See ARCHITECTURE.md section 2 for the
// full reasoning (blocking I/O in a single sequential loop would freeze
// a UI; a dedicated goroutine + channel keeps polling and consuming
// independent of each other).
package poller

import (
	"sync"
	"time"

	"github.com/rekon/rekon/internal/redis"
)

// slowlogFetchCount is how many recent slowlog entries to request per
// poll. 25 is comfortably more than one poll interval is likely to
// produce under normal conditions, while staying small enough that a
// burst of slow commands doesn't make each poll's payload huge.
const slowlogFetchCount = 25

// Snapshot is one poll's results. Each of the three commands Rekon
// polls (INFO, CLIENT LIST, SLOWLOG GET) has its own independent error
// field rather than one shared error for the whole Snapshot — an
// ACL-restricted SLOWLOG command, for example, shouldn't take down
// the Memory/Ops panels that only need INFO.
type Snapshot struct {
	Info    string
	InfoErr error

	ClientListRaw string
	ClientListErr error

	SlowlogEntries []redis.SlowlogEntry
	SlowlogErr     error

	Timestamp time.Time
}

// Poller owns a Redis connection and polls it on Interval, sending each
// result to Results. Callers read from Results; they never call the
// Redis client directly.
type Poller struct {
	client   *redis.Client
	interval time.Duration
	Results  chan Snapshot
	stop     chan struct{}
	stopOnce sync.Once
}

// New creates a Poller against an already-connected client. Connection
// setup stays the caller's responsibility (Sprint 0's job) — Poller's
// only job is scheduling repeated polls, not connection lifecycle.
func New(client *redis.Client, interval time.Duration) *Poller {
	return &Poller{
		client:   client,
		interval: interval,
		Results:  make(chan Snapshot),
		stop:     make(chan struct{}),
	}
}

// Start begins polling in a new goroutine. It returns immediately;
// results arrive asynchronously on p.Results. This is the actual
// mechanism proven in Sprint 1: this goroutine can block on a slow
// network call and nothing else in the program is affected.
func (p *Poller) Start() {
	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-p.stop:
				close(p.Results)
				return
			case <-ticker.C:
				p.Results <- p.pollOnce()
			}
		}
	}()
}

// pollOnce issues all three commands sequentially over the same
// connection (Rekon's Client isn't concurrent-safe, and one Redis
// connection can only have one command in flight at a time regardless)
// and returns a Snapshot with each command's result and error tracked
// independently.
//
// If INFO fails — the strongest signal something is wrong with the
// connection itself, as opposed to e.g. an ACL restricting one specific
// command — pollOnce attempts a reconnect before returning. This is a
// best-effort recovery for the *next* poll, not a retry of the current
// one: the current Snapshot still reports the failure that happened,
// so the UI shows an honest "this poll failed" rather than silently
// hiding it behind a retry.
func (p *Poller) pollOnce() Snapshot {
	snap := Snapshot{Timestamp: time.Now()}
	snap.Info, snap.InfoErr = p.client.Info()

	if snap.InfoErr != nil {
		// Best-effort; if this also fails, the next poll's InfoErr
		// will simply report the same underlying problem again — no
		// special handling needed here beyond not crashing.
		_ = p.client.Reconnect()
	}

	snap.ClientListRaw, snap.ClientListErr = p.client.ClientList()
	snap.SlowlogEntries, snap.SlowlogErr = p.client.SlowlogGet(slowlogFetchCount)
	return snap
}

// Stop signals the polling goroutine to exit and close Results.
// Guarded with sync.Once so a second call is a harmless no-op instead
// of panicking on an already-closed channel — see TECHNICAL_DEBT.md's
// Sprint 1 entry: this was accepted debt while there was only one call
// site, and Sprint 6 adds more (e.g. shutdown on connection-closed vs.
// user-quit), so it's fixed now rather than left as a latent bug.
func (p *Poller) Stop() {
	p.stopOnce.Do(func() {
		close(p.stop)
	})
}
