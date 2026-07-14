#!/usr/bin/env bash
# Basic run: build the frontend (if needed) and the Go backend, then serve the
# whole app (API + built SPA + images) from the backend on http://127.0.0.1:PORT.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-4318}"
CODEX_MODE="${CODEX_MODE:-app-server}"

if [ ! -f "$ROOT/web-dist/index.html" ]; then
  echo "==> web-dist missing; building frontend"
  ( cd "$ROOT/frontend" && npm install && npm run build )
fi

echo "==> Building backend"
( cd "$ROOT/backend" && go build -o "$ROOT/backend/bin/dnd-server" ./cmd/server )

echo "==> Serving on http://127.0.0.1:$PORT  (CODEX_MODE=$CODEX_MODE)"
exec env PORT="$PORT" CODEX_MODE="$CODEX_MODE" "$ROOT/backend/bin/dnd-server"
