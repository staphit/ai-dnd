#!/usr/bin/env bash
# Windows (git-bash): build frontend + backend, serve from the Go binary.
#   ./scripts/run-windows.sh
#   PORT=8080 ./scripts/run-windows.sh
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

# Fail fast if something already holds the port (Windows netstat -ano).
existing_pid="$(netstat -ano 2>/dev/null | { grep -E "[[:space:]]127\.0\.0\.1:$PORT[[:space:]].*LISTENING" || true; } | awk '{print $NF}' | head -n1)" || true
if [ -n "$existing_pid" ]; then
  echo "error: port $PORT is already in use by PID $existing_pid." >&2
  tasklist //FI "PID eq $existing_pid" 2>/dev/null >&2 || true
  echo "Stop it first: taskkill //F //PID $existing_pid" >&2
  exit 1
fi

echo "==> Building frontend"
( cd "$ROOT/frontend" && if [ ! -d node_modules ]; then npm ci; fi && npm run build )

echo "==> Building backend"
( cd "$ROOT/backend" && go build -o "$ROOT/backend/bin/dnd-server.exe" ./cmd/server )

SERVER_BIN="$ROOT/backend/bin/dnd-server.exe"
if [ ! -x "$SERVER_BIN" ] && [ -x "$ROOT/backend/bin/dnd-server" ]; then
  # go on some git-bash setups still emits a no-.exe name
  SERVER_BIN="$ROOT/backend/bin/dnd-server"
fi

echo "==> Serving on http://127.0.0.1:$PORT  (CODEX_MODE=$CODEX_MODE)"
exec env PORT="$PORT" CODEX_MODE="$CODEX_MODE" "$SERVER_BIN"
