# Technical Debt List — Rekon

Per CONSTITUTION.md's required artifacts. This file is a living log, not a
speculative one — entries get added the moment a shortcut is actually taken
during a sprint, not invented in advance or backfilled from memory later.
An empty section below just means that category hasn't incurred debt yet,
not that it's been reviewed and found clean.

Each entry should include: what was shortcut, why (the pressure that caused
it), and what the "correct" version would look like — so a future session
(or a contributor) can decide if/when to pay it down without re-deriving
the reasoning from scratch.

---

## Format for new entries

```
### [Sprint N] Short title
- **What:** the actual shortcut taken
- **Why:** the real pressure/reason (time, unclear requirement, etc.)
- **Correct version:** what doing it properly would look like
- **Risk if unpaid:** what breaks or gets harder if this is never fixed
- **Status:** open / paid down (date + how)
```

---

## Architecture / concurrency

### [Sprint 1] Quit requires "q" + Enter, not a single raw keypress
- **What:** The Sprint 1 proof reads stdin line-by-line (`bufio.Reader.
  ReadString('\n')`), so quitting needs "q" followed by Enter rather than
  a single raw keypress.
- **Why:** True raw-mode terminal input (no Enter required) needs either
  a third-party terminal library or manual `termios`/syscall handling.
  Both are unnecessary complexity for a throwaway proof sprint whose only
  goal is showing the polling and input goroutines run independently —
  and `bubbletea` (Sprint 2) handles raw input properly as part of its
  own architecture anyway, making it wasted effort to solve twice.
- **Correct version:** Sprint 2's `bubbletea` model replaces this
  input-handling goroutine entirely; this file (`cmd/rekon/main.go`)
  itself will be substantially rewritten, not patched.
- **Risk if unpaid:** None in practice — this code is explicitly
  temporary and superseded by Sprint 2, not part of the shipped v1.
- **Status:** paid down (Sprint 2) — `cmd/rekon/main.go` now runs
  through `tea.NewProgram(...).Run()`, which owns raw terminal input.
  `q` alone quits, no Enter required. Verified under a faked pty
  (this sandbox has no real TTY) showing a single 'q' keypress
  triggering `tea.Quit` cleanly.

### [Sprint 1] Poller.Stop() panics if called twice
- **What:** `Stop()` closes an internal channel with no guard against
  being called more than once; a second call panics (closing an
  already-closed channel).
- **Why:** Out of scope for a proof-of-concept sprint whose caller
  (`main.go`) only ever calls `Stop()` once, on the single quit path.
- **Correct version:** Guard with a `sync.Once` or a boolean-plus-mutex
  before this type is relied on by anything beyond Sprint 1's proof.
- **Risk if unpaid:** Low today (single call site), but would become a
  real bug source once more call paths exist (e.g. Sprint 2's UI adding
  its own shutdown/error paths that might also call `Stop()`).
- **Status:** open.

## Redis command handling
*(none yet — log here if e.g. ACL-restricted command failures are
initially just swallowed rather than surfaced distinctly per-metric)*

## Parsing / data modeling
*(none yet — log here if e.g. `INFO` field parsing is done with fragile
string splitting instead of a proper structured parser, "just to get
something rendering")*

## UI / rendering
*(none yet — log here if e.g. panel layout is hardcoded for one terminal
size before responsive sizing is handled properly)*

## Testing
*(none yet — log here if e.g. Sprint 3-5 panels ship without real unit
tests because manual verification against a live Redis felt "good enough"
at the time — name this explicitly if it happens, don't let it go unlogged)*

## Packaging / distribution
*(none yet — log here if e.g. only one OS/architecture is actually tested
before claiming "single static binary" support broadly)*

## Documentation drift
*(none yet — log here if code changes and the README/Architecture.md
aren't updated in the same commit — this is genuine debt, not just a
chore, because it breaks the "documentation matches reality" principle
the whole project is built on)*
