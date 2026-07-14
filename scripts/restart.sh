#!/usr/bin/env bash
# Restart: stop whatever is serving on PORT, then run again (rebuilds the
# backend so code changes take effect).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-4318}"

PIDS="$(lsof -ti "tcp:$PORT" 2>/dev/null || true)"
if [ -n "$PIDS" ]; then
  echo "==> Stopping server on port $PORT (pid: $PIDS)"
  # shellcheck disable=SC2086
  kill $PIDS 2>/dev/null || true
  for _ in 1 2 3 4 5; do
    sleep 1
    lsof -ti "tcp:$PORT" >/dev/null 2>&1 || break
  done
  # shellcheck disable=SC2086
  kill -9 $(lsof -ti "tcp:$PORT" 2>/dev/null) 2>/dev/null || true
else
  echo "==> No server running on port $PORT"
fi

exec env PORT="$PORT" "$ROOT/scripts/run.sh"
