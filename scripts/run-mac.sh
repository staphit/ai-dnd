#!/usr/bin/env bash
# macOS / Linux: build frontend + backend, serve from the Go binary.
#   ./scripts/run-mac.sh
#   PORT=8080 ./scripts/run-mac.sh
set -euo pipefail

# Keep the window open if the build fails (e.g. double-clicked from Finder).
# Once the server starts via exec, this trap no longer applies.
trap 'read -n 1 -s -r -p "Press any key to close..."; echo' EXIT

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-4318}"
CODEX_MODE="${CODEX_MODE:-app-server}"

# Fail fast if something already holds the port (lsof is standard on macOS/Linux).
existing_pid="$(lsof -ti "tcp:$PORT" 2>/dev/null | head -n1 || true)"
if [ -n "$existing_pid" ]; then
  echo "error: port $PORT is already in use by PID $existing_pid." >&2
  echo "Stop it first: kill $existing_pid" >&2
  echo "Or: ./scripts/restart.sh" >&2
  exit 1
fi

echo "==> Building frontend"
( cd "$ROOT/frontend" && npm install && npm run build )

echo "==> Building backend"
( cd "$ROOT/backend" && go build -o "$ROOT/backend/bin/dnd-server" ./cmd/server )

echo "==> Serving on http://127.0.0.1:$PORT  (CODEX_MODE=$CODEX_MODE)"
exec env PORT="$PORT" CODEX_MODE="$CODEX_MODE" "$ROOT/backend/bin/dnd-server"
