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
	"time"

	"github.com/rekon/rekon/internal/redis"
)

// Snapshot is one poll result: either the raw INFO text, or an error if
// that particular poll failed. Errors are delivered on the same channel
// rather than crashing the poller — a single failed poll (e.g. a brief
// network hiccup) shouldn't kill the whole polling loop, only that
// poll's result should reflect the failure.
type Snapshot struct {
	Info      string
	Err       error
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
// mechanism being proven in Sprint 1: this goroutine can block on a
// slow network call and nothing else in the program is affected.
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
				info, err := p.client.Info()
				p.Results <- Snapshot{
					Info:      info,
					Err:       err,
					Timestamp: time.Now(),
				}
			}
		}
	}()
}

// Stop signals the polling goroutine to exit and close Results. Safe to
// call once; calling it twice will panic on the closed stop channel,
// which is acceptable for this sprint's proof-of-concept scope (noted
// in TECHNICAL_DEBT.md rather than guarded against here).
func (p *Poller) Stop() {
	close(p.stop)
}
