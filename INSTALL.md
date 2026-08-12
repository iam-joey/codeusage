# Installing codeusage

Three parts: **server** (once), **agents** (one line per machine), **iOS widget**.

Replace `burn.example.com` with your domain and `<TOKEN>` with your chosen secret throughout.

---

## Prerequisites

- **Server:** a box with Docker + Docker Compose, a domain/subdomain you control, and nginx + certbot (or any reverse proxy with TLS).
- **Each machine you track:** Node.js (for `ccusage`). That's it — no admin, no Homebrew.

---

## 1. Deploy the server

```bash
git clone https://github.com/iam-joey/codeusage.git
cd codeusage
cp .env.example .env
```

Edit `.env`:

```ini
BURN_TOKEN=<TOKEN>          # a long random secret — gates every endpoint. Generate: openssl rand -hex 24
BURN_TZ=Asia/Kolkata        # your timezone (IANA) — sets the day boundary
BURN_STALE_MIN=180          # a machine is "inactive" (red) after this many minutes without a push
```

Start it (binds to `127.0.0.1:8091`):

```bash
docker compose up -d --build
```

Point your reverse proxy at it and add TLS. Example nginx vhost, then certbot:

```nginx
server {
    server_name burn.example.com;
    location / {
        proxy_pass http://127.0.0.1:8091;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/burn.example.com /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d burn.example.com
```

Verify:

```bash
curl https://burn.example.com/healthz          # {"ok":true}
```

---

## 2. Add a machine

Run once per machine (VPS or Mac). Pick a **unique name** each time:

```bash
curl -fsSL https://burn.example.com/install.sh | BURN_TOKEN=<TOKEN> BURN_MACHINE=my-vps bash
```

It installs `ccusage` locally, does a test push, and schedules a push **every 20 minutes** (cron on Linux, launchd on macOS — no admin, and it only runs while the Mac is awake).

Different name per box → its own row: `BURN_MACHINE=macbook`, `BURN_MACHINE=work-mac`, etc. Re-running with a new name just updates it.

Confirm it landed:

```bash
curl -s https://burn.example.com/stats -H "Authorization: Bearer <TOKEN>" | jq '.machines[].machine'
```

Logs: `~/.config/codeusage/push.log`

---

## 3. iOS widget (Scriptable)

1. Install **Scriptable** from the App Store.
2. New script named `codeusage` → paste `widget/codeusage.js`.
3. At the top, set `BURN_URL` to your domain and `BURN_TOKEN` to your token.
4. Tap ▶ to preview.
5. Home screen → long-press → **+** → Scriptable → **Large** → Add → **Edit Widget** → pick `codeusage`.

---

## Config reference

| Var | Where | Meaning |
|---|---|---|
| `BURN_TOKEN` | server `.env` + each agent + widget | shared secret; must match everywhere |
| `BURN_TZ` | server `.env` | day-boundary timezone (IANA). Agents inherit it via the installer |
| `BURN_STALE_MIN` | server `.env` | minutes without a push before a machine shows red (default 180) |
| `BURN_MACHINE` | per agent install | that machine's unique display name (default: hostname) |

## Endpoints

- `POST /push` — agent sends `{machine, days:[…]}` (bearer auth)
- `GET /stats` — aggregated ranges + machines + 7-day spark (bearer auth)
- `GET /install.sh` — the agent installer
- `GET /healthz` — health check (no auth)
