# codeusage 🔥

Track how much you burn — **tokens and $** — across **every AI coding agent** (Claude Code, Codex, GitHub Copilot CLI, Gemini CLI, Goose, Amp, and more) and **every machine** you use, on one iOS widget and a live dashboard.

Self-hosted, tiny, no account. Each machine sums its own usage locally and pushes the totals to your server; your phone reads one endpoint.

<!-- Add a screenshot here: docs/widget.png -->

## Features

- **Every agent, automatically** — powered by [`ccusage`](https://github.com/ccusage/ccusage), which detects all supported coding-agent CLIs on the machine.
- **Every machine** — VPSes + laptops, aggregated. Each shows its own spend, token count, and **last-seen** time (green = active, red = inactive).
- **Ranges** — today / 7 days / 30 days, each with $, tokens burnt, and trend vs the previous period.
- **iOS Large widget** (via Scriptable) + a **live dashboard** you open in any browser.
- **One-line agent install** — `curl … | bash`, no admin, no Homebrew.
- **Honest numbers** — `$` is API-equivalent (cache-weighted, real for API users / notional on a subscription). The token count shown is **input + output** ("prompted"), not the cache-inflated total.

## How it works

```
each machine:  ccusage  ──►  POST /push  ──►  Go server + SQLite  ──►  GET /stats  ──►  iOS widget / dashboard
 (cron/launchd, every 20 min)                (dedup by machine+day)
```

- The agent pushes the **full per-day array** each run, so it's idempotent and self-healing — duplicates are impossible (primary key is `(machine, day)`), and a missed push backfills on the next one.
- Ranges are just date-window sums over the per-day rows, so any range (today / 7d / 30d / …) is a query, not stored state.
- `$` uses ccusage's cost (cache reads priced at ~10%, writes at a premium) — so the dollar figure stays sane even when cache-read tokens dominate the raw count.

## Repo layout

```
server/    Go single-binary API + SQLite + Docker. Serves /push, /stats, /install.sh, dashboard at /
widget/    codeusage.js — the iOS Large widget (Scriptable)
docker-compose.yml
```

## Quick start

See **[INSTALL.md](INSTALL.md)** — deploy the server, add machines with a one-liner, add the widget.

## Notes

- The token count is **prompted tokens (input+output)**; cache tokens are excluded from the count (they'd be ~98% of it) but are still reflected in the `$`.
- `BURN_TZ` sets the day boundary; the agent must bucket in the same timezone (the installer handles this).

## License

MIT — see [LICENSE](LICENSE).
