#!/usr/bin/env bash
# Basic run: build the frontend (if needed) and the Go backend, then serve the
# whole app (API + built SPA + images) from the backend on http://127.0.0.1:PORT.
set -euo pipefail
# Keep the window open if the build fails, so a double-click launch doesn't
# just vanish before you can read the error. Once the server starts below
# via exec, this trap no longer applies — the server owns the window, and
# Ctrl+C reaches it directly (a foreground non-exec child often doesn't get
# Ctrl+C forwarded reliably from git-bash to a native Windows exe).
trap 'read -n 1 -s -r -p "Press any key to close..."; echo' EXIT
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-4318}"
CODEX_MODE="${CODEX_MODE:-app-server}"

# Fail fast with an actionable message if something already holds the port,
# instead of a bare "bind: Only one usage..." from the Go server.
existing_pid="$(netstat -ano 2>/dev/null | { grep -E "[[:space:]]127\.0\.0\.1:$PORT[[:space:]].*LISTENING" || true; } | awk '{print $NF}' | head -n1)"
if [ -n "$existing_pid" ]; then
  echo "error: port $PORT is already in use by PID $existing_pid." >&2
  tasklist //FI "PID eq $existing_pid" 2>/dev/null >&2 || true
  echo "Stop it first: taskkill //F //PID $existing_pid" >&2
  exit 1
fi

echo "==> Building frontend"
( cd "$ROOT/frontend" && npm install && npm run build )

echo "==> Building backend"
( cd "$ROOT/backend" && go build -o "$ROOT/backend/bin/dnd-server" ./cmd/server )

echo "==> Serving on http://127.0.0.1:$PORT  (CODEX_MODE=$CODEX_MODE)"
exec env PORT="$PORT" CODEX_MODE="$CODEX_MODE" "$ROOT/backend/bin/dnd-server"
