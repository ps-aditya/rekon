# Roadmap.md — Rekon

Per CONSTITUTION.md's Code Generation Policy: small modules, one feature, one
file, one review, one iteration. Each sprint below should end with something
that runs and can be explained, not just compiles.

## v1 — Live TUI (this is the whole ship-in-a-week target)

### Sprint 0 — Foundation
- Go module init, project layout (`cmd/`, `internal/redis/`, `internal/ui/`,
  `internal/model/`)
- Redis connection handling: connect via connection string, fail loudly and
  clearly if unreachable (no silent retries masking a real problem)
- A single manual command proven working end-to-end: run `INFO`, print raw
  output to stdout. No TUI yet. Goal: prove the Redis client works before
  any rendering exists.
- **Understanding checkpoint:** can you explain why we prove the Redis
  connection *before* touching `bubbletea`? (Isolating failure sources —
  if the TUI breaks later, you'll know it's not the connection layer.)

### Sprint 1 — Polling goroutine + channel
- Implement the background polling goroutine issuing `INFO` on a timer
- Implement the channel handoff to a (still-minimal) main loop
- Prove it: print each new snapshot to stdout as it arrives, on schedule,
  while a separate goroutine independently accepts a keypress to quit
  cleanly (`q` to exit) — this is the smallest possible proof that both
  goroutines run independently
- **Understanding checkpoint:** what happens if the polling goroutine
  panics — does it take down the whole program? (Yes, by default, unless
  recovered — worth deciding explicitly whether/how to handle this, not
  leaving it as an accident.)

### Sprint 2 — Model–Update–View skeleton (bubbletea)
- Define the `Model` struct: holds latest `RedisSnapshot`, connection
  status, currently focused panel, error state
- Wire the channel's incoming snapshots into `bubbletea`'s `Update` as a
  custom message type
- `View` renders raw text (no panels/styling yet) — just prove data flows:
  connect → poll → channel → Update → View → screen, all working together
- **Understanding checkpoint:** explain in your own words what `Update`
  returning a new `Model` means for how state changes in this program,
  versus how you might have expected a normal imperative loop to work

### Sprint 3 — Memory + Ops panels
- Parse the actual fields needed from `INFO` (used memory, fragmentation
  ratio, maxmemory + policy, ops/sec, keyspace hits/misses)
- Render as a real panel with `lipgloss` styling — first taste of visual
  design decisions (what's alarming vs. normal, color thresholds)
- Decide and document actual threshold logic (e.g., what fragmentation
  ratio counts as "concerning") — this is domain knowledge you should be
  able to defend, not a copied number

### Sprint 4 — Clients + Slowlog panels
- `CLIENT LIST` parsing: connected count, blocked count, flag long-idle
  connections
- `SLOWLOG GET` live tail: detect and highlight entries new since last poll
  (requires tracking last-seen slowlog ID between polls — a small but real
  piece of state design)

### Sprint 5 — Replication + Persistence panels
- Role detection (master/replica/standalone), replica count, lag if
  applicable — must degrade gracefully on a standalone instance (no
  replication section, not a broken/empty one)
- Last save time, RDB/AOF status, save-in-progress flag

### Sprint 6 — Polish + packaging
- Config: connection string via flag/env, configurable poll interval
- Graceful handling of Redis disconnecting mid-run (don't crash — show a
  clear "disconnected, retrying" state)
- Build a single static binary; test the actual install experience someone
  would have (not just `go run` from inside the repo)
- Record the demo GIF (per the "demo quality drives traction" conclusion —
  don't treat this as an afterthought)

## v1 — explicit non-goals (deferred, stated up front)
- Cluster-mode / multi-node view
- Export/replay rolling-window recording format (this is the whole of v2 —
  see below)
- Alerting or notifications
- Any config-writing / mutating capability
- Any non-Redis service

## v2 — Rolling-window recording + replay (deferred, not started until v1 ships)
- In-memory ring buffer already exists informally from the polling loop —
  v2 formalizes it into a fixed-duration window (e.g., last N minutes)
- Define the export file format (structure TBD in its own
  Architecture addendum when this sprint starts — not decided prematurely)
- Replay mode: scrub through a recorded window in the same TUI
- Positioned as "attach this to a bug report / incident channel," not as
  an AI-feeding mechanism (per earlier conversation — the format's reason
  to exist is replay/share, LLM summarization is a bonus feature layered
  on top, not the core justification)

## Explicitly out of scope indefinitely (not just "later")
- Any AI/LLM-based analysis or "root cause" claims
- Any write/mutating Redis operations
- A GUI or web dashboard version
