# Launch Checklist — Rekon v1

Per CONSTITUTION.md: every project must produce a Launch Checklist. This is
the concrete, checkable bar for "v1 is actually done," separate from the
Roadmap's sprint sequencing — Roadmap is "how we get there," this is
"how we know we've arrived."

## Functional correctness
- [ ] Connects to a real local Redis instance via `--url`
- [ ] Connects to a real remote Redis instance (not just localhost)
- [ ] All six v1 panels render live, correct data (Memory, Ops, Clients,
      Slowlog, Replication, Persistence)
- [ ] Replication panel correctly degrades on a standalone instance (no
      broken/empty section)
- [ ] Slowlog panel correctly identifies *new* entries since last poll, not
      just re-showing the same entries every refresh
- [ ] No mutating/write Redis command is ever issued — verify by reviewing
      every command call site, not by assumption
- [ ] Handles Redis disconnecting mid-run without crashing (shows a clear
      "disconnected, retrying" state)
- [ ] Handles a permission-restricted Redis (ACL blocks one of `INFO`/
      `SLOWLOG GET`/`CLIENT LIST`) by showing that one metric as
      unavailable, not crashing the whole program
- [ ] `q` quits cleanly, no orphaned goroutines or hung terminal state

## Performance
- [ ] Poll interval is honored accurately (not drifting significantly over
      a long-running session)
- [ ] UI stays responsive (keypresses, resize) even if a poll is slow —
      this is the core concurrency guarantee from Architecture.md; test it
      deliberately (e.g., artificially slow network) rather than assuming
      it works because the design says it should
- [ ] Memory usage of Rekon itself stays flat over a long session (no
      leak from the polling loop or channel handling)

## Packaging / distribution
- [ ] Builds a single static binary with no runtime dependency
- [ ] Tested by actually installing fresh (not `go run` from inside the
      dev repo) — simulate what a stranger would experience
- [ ] `go install` path works, or a documented binary download works
- [ ] README's install instructions are copy-paste-run accurate, verified
      by literally following them from scratch

## Documentation
- [ ] README accurately describes only what v1 actually does — no
      aspirational claims about export/replay or AI features that don't
      exist yet
- [ ] Demo GIF is a real recording of real output (not a mockup)
- [ ] `--help` output is accurate and matches README's flag table

## Distribution readiness (per the "distribution built in parallel" philosophy)
- [ ] GitHub repo Topics set (e.g., `redis`, `tui`, `cli`, `golang`,
      `monitoring`, `devtools`)
- [ ] GitHub Discussions enabled
- [ ] License file present and correct (Apache 2.0)
- [ ] Considered submission to relevant niche directories (Terminal Trove,
      Console.dev) once README/demo are real, not before
- [ ] Show HN / Product Hunt post drafted in advance, following the pattern
      from the reference posts: one-line problem, one-line mechanism,
      honest limitation, license/stars as social proof — not overselling
      what v1 actually does

## The understanding bar (per CONSTITUTION.md Success Metric — this is not optional)
- [ ] Can explain, unprompted, why the polling goroutine + channel design
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
