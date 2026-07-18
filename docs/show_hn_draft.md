# Show HN / Product Hunt draft — Rekon

Written following the pattern observed across the reference posts studied
during planning: one-line problem, one-line mechanism, honest limitation,
license/stars as social proof — not overselling what v1 actually does.

Fill in `<>` placeholders once the repo is live and you have real numbers.
Do not post with placeholder text still in it.

---

## Show HN title

```
Show HN: Rekon – btop, but it understands Redis
```

## Post body

`redis-cli --stat` gives you a scrolling table. RedisInsight gives you a
heavy GUI app you probably don't want SSH'd into a server for. Neither
gives you a live, glanceable dashboard that lives where you already are —
the terminal.

Rekon polls a Redis instance on an interval and renders memory pressure,
fragmentation, ops throughput, client connections, the slowlog, replication
state, and persistence status — live, in one screen, in your terminal.

No AI, no LLM calls, no "root cause analysis" — every number on screen
comes directly from a real Redis command (`INFO`, `SLOWLOG GET`,
`CLIENT LIST`), deterministic and auditable. It's also strictly read-only:
Rekon never issues a mutating or config-changing command, so it's safe to
point at something you care about.

Built in Go with `bubbletea`/`lipgloss`, ships as a single static binary —
no runtime dependency, no `npm install` before you can even see it work.

Honest limitations: v1 targets a single standalone or primary/replica
instance, no cluster-mode awareness yet. It's young — `<N>` commits,
built over about a week, so expect rough edges. Feedback and issues very
welcome, especially on what panel/metric would matter most to you next.

`<X>` stars, Apache-2.0, MIT-compatible dependencies throughout.

<https://github.com/<your-username>/rekon>

---

## Notes for whoever posts this

- Don't post until: the demo GIF is real (see LAUNCH_CHECKLIST.md), the
  README's install instructions have been verified against the actual
  live repo URL (not just locally), and at least a few of the "manual
  step for you" Launch Checklist items are done (Topics, Discussions).
- Fill in real commit count and star count at time of posting — don't
  leave placeholders in a live post.
- Per the "demo quality drives traction" reasoning from planning: lead
  with the GIF/screenshot if the platform supports an image attachment
  (Product Hunt) — for Show HN (text-only), the writing has to do that
  work instead, which is why the post opens with the concrete pain
  (`redis-cli --stat`'s scrolling table) rather than an abstract pitch.
- Be ready to answer fast in the first hour — per the same planning
  notes, personal engagement in comments matters more than the initial
  post text for how a Show HN thread actually performs.
