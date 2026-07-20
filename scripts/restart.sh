#!/usr/bin/env bash
# Restart: stop whatever is serving on PORT, then run again (rebuilds the
# backend so code changes take effect). Uses lsof on macOS/Linux and falls
# back to netstat/taskkill on Windows git-bash.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-4318}"

port_pids() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -ti "tcp:$PORT" 2>/dev/null || true
  else
    netstat -ano 2>/dev/null | { grep -E "[[:space:]]127\.0\.0\.1:$PORT[[:space:]].*LISTENING" || true; } | awk '{print $NF}' | sort -u
  fi
}

kill_pid() { # pid, force(0|1)
  if command -v lsof >/dev/null 2>&1; then
    if [ "$2" = "1" ]; then kill -9 "$1" 2>/dev/null || true
    else kill "$1" 2>/dev/null || true; fi
  else
    # git-bash: kill only reaches MSYS processes; the Go server is a native
    # Windows exe, so use taskkill. It has no graceful option that works
    # cross-session, so both passes force-terminate.
    taskkill //F //PID "$1" >/dev/null 2>&1 || true
  fi
}

PIDS="$(port_pids)"
if [ -n "$PIDS" ]; then
  echo "==> Stopping server on port $PORT (pid: $PIDS)"
  for pid in $PIDS; do kill_pid "$pid" 0; done
  for _ in 1 2 3 4 5; do
    sleep 1
    [ -n "$(port_pids)" ] || break
  done
  for pid in $(port_pids); do kill_pid "$pid" 1; done
else
  echo "==> No server running on port $PORT"
fi

exec env PORT="$PORT" "$ROOT/scripts/run.sh"
