#!/usr/bin/env bash
# Dev mode: run the Go backend (API, port 4318) and the Vite dev server
# (frontend with hot reload, port 4317) together. Vite proxies /api and
# /generated to the backend. Open http://127.0.0.1:4317.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-4318}"
CODEX_MODE="${CODEX_MODE:-app-server}"

if [ ! -d "$ROOT/frontend/node_modules" ]; then
  ( cd "$ROOT/frontend" && npm ci )
fi

cleanup() { kill 0 2>/dev/null || true; }
trap cleanup EXIT INT TERM

echo "==> backend  http://127.0.0.1:$PORT  (CODEX_MODE=$CODEX_MODE)"
( cd "$ROOT/backend" && exec env PORT="$PORT" CODEX_MODE="$CODEX_MODE" go run ./cmd/server ) &

echo "==> frontend http://127.0.0.1:4317  (proxies /api to backend)"
( cd "$ROOT/frontend" && exec npm run dev ) &

wait
