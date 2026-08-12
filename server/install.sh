#!/usr/bin/env bash
# codeusage agent installer — no admin/sudo, no Homebrew. Needs only Node.js.
#   curl -fsSL https://burn.iamdsv.dev/install.sh | BURN_TOKEN=<token> BURN_MACHINE=<name> bash
set -euo pipefail

BURN_URL="${BURN_URL:-https://burn.iamdsv.dev}"
: "${BURN_TOKEN:?Set BURN_TOKEN=... before running}"
MACHINE="${BURN_MACHINE:-$(hostname -s 2>/dev/null || hostname)}"
TZONE="${BURN_TZ:-Asia/Kolkata}"
DIR="$HOME/.config/codeusage"; mkdir -p "$DIR"

echo "codeusage: setting up '$MACHINE' -> $BURN_URL"
command -v curl >/dev/null 2>&1 || { echo "need curl"; exit 1; }
command -v node >/dev/null 2>&1 || { echo "Please install Node.js from nodejs.org, then re-run (no admin needed)."; exit 1; }
command -v npm  >/dev/null 2>&1 || { echo "Please install npm (ships with Node.js), then re-run."; exit 1; }

echo "codeusage: installing ccusage into $DIR (no admin)…"
npm install --prefix "$DIR" ccusage@latest >/dev/null 2>&1 || { echo "ccusage install failed"; exit 1; }

umask 077
cat > "$DIR/env" <<EOF
BURN_URL=$BURN_URL
BURN_TOKEN=$BURN_TOKEN
BURN_MACHINE=$MACHINE
BURN_TZ=$TZONE
EOF
# make sure cron can find node (covers nvm/homebrew installs)
echo "PATH=$(dirname "$(command -v node)"):\$PATH" >> "$DIR/env"

cat > "$DIR/push.sh" <<'PUSH'
#!/usr/bin/env bash
set -euo pipefail
DIR="$HOME/.config/codeusage"
. "$DIR/env"
export BURN_URL BURN_TOKEN BURN_MACHINE BURN_TZ PATH
CC="$DIR/node_modules/.bin/ccusage"; [ -x "$CC" ] || CC="npx -y ccusage@latest"
$CC daily --json --timezone "${BURN_TZ:-UTC}" | node -e '
const fs=require("fs");
let d={}; try{ d=JSON.parse(fs.readFileSync(0,"utf8")); }catch(e){ process.exit(1); }
const M=process.env.BURN_MACHINE||require("os").hostname();
const days=(d.daily||[]).map(x=>({date:x.period,input_tokens:x.inputTokens||0,output_tokens:x.outputTokens||0,cache_creation_tokens:x.cacheCreationTokens||0,cache_read_tokens:x.cacheReadTokens||0,cost_usd:x.totalCost||0}));
fetch(process.env.BURN_URL.replace(/\/$/,"")+"/push",{method:"POST",headers:{Authorization:"Bearer "+process.env.BURN_TOKEN,"Content-Type":"application/json"},body:JSON.stringify({machine:M,days})})
 .then(r=>{ if(!r.ok){ console.error("push HTTP "+r.status); process.exit(1);} console.log("codeusage: pushed "+M+" ("+days.length+" days)"); })
 .catch(e=>{ console.error(e.message); process.exit(1); });
'
PUSH
chmod +x "$DIR/push.sh"

echo "codeusage: test push…"
bash "$DIR/push.sh"

if [ "$(uname)" = "Darwin" ]; then
  # macOS: launchd LaunchAgent (per-user, no admin, no crontab permission prompt)
  mkdir -p "$HOME/Library/LaunchAgents"
  PLIST="$HOME/Library/LaunchAgents/dev.codeusage.push.plist"
  cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>dev.codeusage.push</string>
  <key>ProgramArguments</key><array><string>/bin/bash</string><string>$DIR/push.sh</string></array>
  <key>StartInterval</key><integer>1200</integer>
  <key>RunAtLoad</key><true/>
  <key>StandardOutPath</key><string>$DIR/push.log</string>
  <key>StandardErrorPath</key><string>$DIR/push.log</string>
</dict></plist>
EOF
  launchctl bootout "gui/$(id -u)/dev.codeusage.push" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "$PLIST" 2>/dev/null || launchctl load "$PLIST" 2>/dev/null || true
  echo "codeusage: scheduled every 20 min via launchd (no admin). ✅"
else
  # Linux: cron
  LINE="*/20 * * * * $DIR/push.sh >> $DIR/push.log 2>&1 # codeusage"
  ( crontab -l 2>/dev/null | grep -v '# codeusage$' || true; echo "$LINE" ) | crontab -
  echo "codeusage: scheduled every 20 min via cron. ✅"
fi
echo "codeusage: done — '$MACHINE' is set."
