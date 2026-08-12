# codeusage 🔥

Track your **Claude Code** token & $ usage across all your machines — on one iOS widget and a live dashboard. Self-hosted, tiny, no account.

![codeusage widget](docs/widget.png)

## Features

- **All your machines in one place** — VPSes + Macs, each with a **last-seen** time (green = active, red = inactive).
- **today / 7d / 30d** — dollars, tokens, and trend vs the previous period.
- **iOS Large widget** (via Scriptable) + a **live dashboard** in any browser.
- **One-line agent install**, no admin. Self-hosted, MIT.

## Setup

See **[INSTALL.md](INSTALL.md)** — deploy the server, add machines with a one-liner, add the widget.

## How it works

Each machine runs [`ccusage`](https://github.com/ccusage/ccusage) and pushes its daily totals to a tiny Go + SQLite server (every 20 min); your widget and dashboard read one endpoint. Dollars are cache-weighted (API-equivalent); the token count shown is input + output.

## License

MIT — see [LICENSE](LICENSE).
