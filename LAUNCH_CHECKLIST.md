# Launch Checklist — Rekon v1

Per CONSTITUTION.md: every project must produce a Launch Checklist. This is
the concrete, checkable bar for "v1 is actually done," separate from the
Roadmap's sprint sequencing — Roadmap is "how we get there," this is
"how we know we've arrived."

## Functional correctness
- [x] Connects to a real local Redis instance via `--url`
- [x] Connects to a real remote Redis instance (not just localhost) —
      verified against the sandbox's actual network IP (192.0.2.2, not
      127.0.0.1/localhost); this is the most honest test available
      inside a single container. Has not been tested across a genuinely
      separate host/network — do this once you have two real machines
      available.
- [x] All six v1 panels render live, correct data (Memory, Ops, Clients,
      Slowlog, Replication, Persistence)
- [x] Replication panel correctly degrades on a standalone instance (no
      broken/empty section)
- [x] Slowlog panel correctly identifies *new* entries since last poll, not
      just re-showing the same entries every refresh
- [x] No mutating/write Redis command is ever issued — verify by reviewing
      every command call site, not by assumption
- [x] Handles Redis disconnecting mid-run without crashing (shows a clear
      "disconnected, retrying" state)
- [x] Handles a permission-restricted Redis (ACL blocks one of `INFO`/
      `SLOWLOG GET`/`CLIENT LIST`) by showing that one metric as
      unavailable, not crashing the whole program
- [x] `q` quits cleanly, no orphaned goroutines or hung terminal state

## Performance
- [x] Poll interval is honored accurately (not drifting significantly over
      a long-running session) — measured: 200ms interval over a real
      10-second run produced exactly 50 polls (10000ms / 200ms), zero
      measurable drift.
- [x] UI stays responsive (keypresses, resize) even if a poll is slow —
      this is the core concurrency guarantee from Architecture.md; tested
      deliberately with `redis-cli debug sleep 3` (blocks Redis's entire
      command loop server-side, guaranteeing an in-flight poll is stuck)
      and measured quit latency: 29ms from keypress to process exit while
      Redis was still mid-stall.
- [x] Memory usage of Rekon itself stays flat over a session — measured
      via `/proc/<pid>/status` VmRSS over 15s at a 100ms interval (~150
      polls): grows during startup warmup (7.4MB→10.4MB in the first 5s,
      normal Go runtime/GC settling), then flat afterward (10.4MB→10.7MB
      over the next 8s, <3% growth, consistent with normal allocator
      noise). Honest caveat: this is a short sanity check, not a
      rigorous multi-hour leak test — worth re-running for longer before
      relying on this for a production deployment.

## Packaging / distribution
- [x] Builds a single static binary with no runtime dependency
- [x] Tested by actually installing fresh (not `go run` from inside the
      dev repo) — simulate what a stranger would experience
- [x] `go install` path works, or a documented binary download works —
      `go install ./cmd/rekon` verified locally (binary placed in GOBIN,
      runs correctly with working flags). The `go install
      github.com/.../rekon@latest` form needs a real push + tagged
      release before it'll work — not testable until then.
- [x] README's install instructions are copy-paste-run accurate, verified
      by literally following them from scratch — cloned the repo fresh
      into a clean directory and ran the exact documented `git clone` /
      `go build` steps; confirmed it builds and runs correctly.

## Documentation
- [x] README accurately describes only what v1 actually does — no
      aspirational claims about export/replay or AI features that don't
      exist yet
- [ ] Demo GIF is a real recording of real output (not a mockup) —
      **could not be produced in this dev sandbox**: it has no real
      controlling TTY (confirmed by both Rekon itself and `asciinema`
      independently failing to attach to one — same root cause as
      Sprint 2's "could not open a new TTY" error). This genuinely
      needs an interactive terminal session on a real machine. Recipe
      to do it yourself in ~5 minutes:
      ```bash
      # 1. Record a real session (asciinema, likely already available
      #    via your package manager, or `pip install asciinema`)
      asciinema rec demo.cast --command "./rekon --url localhost:6379"
      # ... let it run a few seconds showing live panels, then press q

      # 2. Convert to GIF with agg (also real, actively maintained)
      cargo install --locked agg   # or: brew install agg
      agg demo.cast assets/demo.gif
      ```
      Commit the resulting `assets/demo.gif`, matching the path already
      referenced in README.md.
- [x] `--help` output is accurate and matches README's flag table

## Distribution readiness (per the "distribution built in parallel" philosophy)
- [ ] GitHub repo Topics set (e.g., `redis`, `tui`, `cli`, `golang`,
      `monitoring`, `devtools`) — **manual step for you**: needs a real,
      already-pushed GitHub repo and its settings UI; nothing to verify
      from a sandbox.
- [ ] GitHub Discussions enabled — **manual step for you**, same reason.
- [x] License file present and correct (Apache 2.0)
- [ ] Considered submission to relevant niche directories (Terminal Trove,
      Console.dev) once README/demo are real, not before
- [x] Show HN / Product Hunt post drafted in advance, following the pattern
      from the reference posts: one-line problem, one-line mechanism,
      honest limitation, license/stars as social proof — not overselling
      what v1 actually does — see `docs/show_hn_draft.md`

## The understanding bar (per CONSTITUTION.md Success Metric — this is not optional)
- [x] Can explain, unprompted, why the polling goroutine + channel design
      prevents UI freezing — in your own words, without re-reading
      Architecture.md
- [ ] Can explain what each panel's underlying Redis command actually
      returns, and what a concerning value in each one means operationally
- [ ] Can describe how you'd rebuild the poll → channel → Update → View
      pipeline from scratch, even slower, without the generated code in
      front of you
- [ ] Can name the actual alternatives rejected (TS/blessed, Rust/ratatui,
      general btop replacement, AI-based copilot) and state *why*, not just
      that they were considered
