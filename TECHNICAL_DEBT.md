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
*(none yet — log here if e.g. error handling on a goroutine panic gets
deferred, or channel buffering is left unbounded "for now")*

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
