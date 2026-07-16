# Architecture.md — Rekon v1

## 1. What we're building

Rekon is a live, terminal-native TUI that monitors a single Redis instance's
internals in real time — memory, ops throughput, client connections, slowlog,
replication, and persistence state — refreshed on a short polling interval.

It is explicitly **not**:
- A general OS resource monitor (that's btop/htop's job)
- An AI-powered diagnostic tool (no LLM, no inference, no "root cause" claims)
- A Redis admin/write tool (Rekon never issues `CONFIG SET`, `FLUSHALL`, or
  any mutating command — read-only, always)
- A multi-service or cluster-aware tool in v1 (single standalone instance only)

The one-sentence pitch: **btop, but it understands Redis.**

## 2. Why this architecture makes sense

Redis already exposes nearly everything Rekon needs through existing,
well-documented commands (`INFO`, `SLOWLOG GET`, `CLIENT LIST`,
`CONFIG GET maxmemory-policy`). This means v1 requires **no new
instrumentation, no Redis modules, no log parsing** — just a polling loop
issuing known commands on an interval and rendering the results. That keeps
the actual engineering surface area of v1 realistic for a one-week build,
while still being genuinely useful (this is the same "no AI in the hot path,
deterministic, auditable" philosophy as inode-cli — just applied to reading
live state instead of matching command patterns).

The core technical problem worth building carefully is concurrency: the UI
must stay responsive (accept keypresses, resize, scroll) while a network
call to Redis is potentially slow or stalled. A single sequential loop would
freeze the entire UI for the duration of any slow poll. This is solved with:

- **A dedicated polling goroutine** that issues Redis commands on a timer,
  independent of the render loop.
- **A channel** (`chan RedisSnapshot`) as the only handoff between the
  polling goroutine and the main UI loop — no shared mutable state, no
  manual locking.
- **`bubbletea`'s Model–Update–View loop**, where an incoming channel value
  is just another kind of message the `Update` function reacts to (same
  category as a keypress), producing a new `Model` that `View` renders.

This directly prevents the specific failure mode we walked through: blocking
I/O in one loop freezing the whole UI. The polling goroutine can wait on the
network by itself; the UI keeps running regardless of how long that wait is.

## 3. Constraints

- **Read-only.** Rekon must never send a command capable of mutating Redis
  state or configuration. This is a hard rule, not a style preference — it's
  the same "side-tool, not a gatekeeper" trust model as inode-cli.
- **Single instance, local or remote via connection string.** No cluster
  topology awareness in v1.
- **Terminal-only.** No web UI, no daemon, no background service — one
  binary, one process, runs in the foreground in a terminal.
- **Polling, not event-driven.** Redis doesn't push metric changes to
  clients; Rekon must poll. Poll interval is configurable but defaults to
  something reasonable (proposal: 1s) balancing responsiveness against load
  on the Redis instance being watched.
- **No persistence between runs in v1.** The rolling metrics buffer lives in
  memory only; export/replay-to-file is an explicitly deferred v2 feature
  (see Roadmap.md).

## 4. Assumptions

- The user has network access to a Redis instance they're allowed to run
  `INFO`/`SLOWLOG`/`CLIENT LIST` against (these are safe, standard,
  non-privileged commands on most Redis setups, but ACL-restricted
  environments may block one or more — Rekon should degrade gracefully,
  showing "unavailable" for a metric rather than crashing).
- The terminal supports a reasonably modern feature set (true color not
  required, but a standard ANSI-capable terminal is assumed).
- Users are comfortable installing a single compiled binary (Go's
  distribution model) rather than requiring a runtime (Node, Python, JVM).

## 5. Alternatives considered and rejected

| Alternative | Why rejected |
|---|---|
| **TypeScript + `blessed`/`ink`** | Fastest for the author personally (proven via inode-cli), but requires Node installed, and Node-based TUI rendering is a known weaker point for a fast, live-updating dashboard. Distribution friction (npm install before a visitor can even see the demo) directly works against the "get users to try it" goal. |
| **Rust + `ratatui`** | Technically excellent TUI ecosystem and single-binary distribution like Go, but steepest learning curve of the three options for a first attempt in the language — higher risk of the week being consumed by language-learning rather than the actual system. |
| **General resource monitor (btop replacement)** | Rejected as a *direction*, not a language choice — see prior discussion. No clear wedge against a mature, loved incumbent; high effort just to reach parity. |
| **AI-based root cause analysis (Redis copilot)** | Rejected because it invites the "just an LLM wrapper" critique common on Show HN/HN, and because most of the interesting Redis failure modes are actually well-known deterministic signatures (fragmentation ratio, eviction thrashing, etc.) — an LLM adds cost and non-determinism without adding real diagnostic value over explicit rules. |
| **Point-in-time snapshot export (v2 feature) instead of rolling window** | Rejected because a single point-in-time capture is trivially replaced by `redis-cli INFO > file.txt` — no real reason to install a tool for that. A rolling window captures the lead-up to an incident automatically, which a manual snapshot structurally cannot do. (Deferred to v2 regardless — noted here for the record of why rolling-window is the target design when that feature is built.) |

## 6. Success criteria for v1 (ties to CONSTITUTION.md's Success Metric)

- Can explain, unprompted: why goroutines + channels fit this problem: done,
  demonstrated in project chat log before this doc was written.
- Can explain every panel's data source (which Redis command backs it) and
  what a bad value in each one actually means operationally.
- Can rebuild the polling+channel+render mechanism from scratch without
  looking at the generated code, even if slower than a first pass.
- Ships: installable, runs against a real local Redis instance, all six v1
  panels render live data correctly.
