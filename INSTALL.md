# Installing codeusage

End to end: a **server** (once), your **machines** (one line each), and the **iOS widget**.

Throughout, replace `burn.example.com` with your own domain and `<TOKEN>` with a secret you generate.

---

## Prerequisites

- A **Linux server with a public IP** — any VPS (Hetzner, Contabo, DigitalOcean, etc.).
- A **domain or subdomain** you can add a DNS record to.
- On **each machine you want to track:** Node.js (for `ccusage`). No admin needed.

---

## 1. Prepare the server

SSH into your server and install Docker, nginx, and certbot (Ubuntu/Debian shown):

```bash
curl -fsSL https://get.docker.com | sh
sudo apt-get update
sudo apt-get install -y nginx certbot python3-certbot-nginx
```

---

## 2. Point your domain at the server

In your DNS provider (Namecheap, Cloudflare, etc.), add an **A record**:

| Type | Host | Value |
|------|------|-------|
| `A`  | `burn` (→ `burn.example.com`) | your server's public IP |

Wait for it to resolve before continuing — this must work **before** you request the TLS cert:

```bash
dig +short burn.example.com     # should print your server's IP
```

---

## 3. Deploy the server

```bash
git clone https://github.com/iam-joey/codeusage.git
cd codeusage
cp .env.example .env
```

Generate a token and put it in `.env`:

```bash
openssl rand -hex 24            # copy the output into BURN_TOKEN
```

```ini
# .env
BURN_TOKEN=<TOKEN>              # gates every endpoint — same value used everywhere
BURN_TZ=Asia/Kolkata           # your timezone (IANA) — sets the day boundary
BURN_STALE_MIN=180             # minutes without a push before a machine shows red
```

Start it (listens on `127.0.0.1:8091`, only local — nginx will expose it):

```bash
docker compose up -d --build
curl http://127.0.0.1:8091/healthz     # {"ok":true}
```

---

## 4. nginx + HTTPS

Create the reverse-proxy vhost:

```bash
sudo tee /etc/nginx/sites-available/burn.example.com >/dev/null <<'EOF'
server {
    server_name burn.example.com;
    location / {
        proxy_pass http://127.0.0.1:8091;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF

sudo ln -s /etc/nginx/sites-available/burn.example.com /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

Get the certificate (this also adds the HTTPS redirect):

```bash
sudo certbot --nginx -d burn.example.com
```

Verify it's live over HTTPS:

```bash
curl https://burn.example.com/healthz     # {"ok":true}
```

---

## 5. Add a machine

Run once per machine (server, VPS, or Mac). Pick a **unique name** each time:

```bash
curl -fsSL https://burn.example.com/install.sh | BURN_TOKEN=<TOKEN> BURN_MACHINE=my-vps bash
```

It installs `ccusage` locally, does a test push, and schedules a push **every 20 minutes** (cron on Linux, launchd on macOS — no admin; on a Mac it only runs while awake).

Unique name per box → its own row: `BURN_MACHINE=macbook`, `BURN_MACHINE=work-mac`, etc. Re-running with a new name just updates it.

Confirm it landed:

```bash
curl -s https://burn.example.com/stats -H "Authorization: Bearer <TOKEN>" | jq '.machines[].machine'
```

Logs: `~/.config/codeusage/push.log`

---

## 6. iOS widget (Scriptable)

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
