#!/usr/bin/env bash
# Start the local GPT-SoVITS api_v2 server for the backend's /api/tts endpoint.
#
#   scripts/sovits.sh                  # http://127.0.0.1:9880
#   SOVITS_PORT=9881 scripts/sovits.sh
#   SOVITS_LISTEN=1 scripts/sovits.sh  # bind 0.0.0.0 — backend runs on another machine
set -euo pipefail
# Keep the window open if double-clicked and setup hasn't run yet, so the
# error is readable instead of flashing shut. Once the server starts below
# via exec, this trap no longer applies — the server owns the window.
trap 'read -n 1 -s -r -p "Press any key to close..."; echo' EXIT
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOVITS_DIR="$ROOT/vendor/GPT-SoVITS"
PORT="${SOVITS_PORT:-9880}"
ADDR="127.0.0.1"
if [ "${SOVITS_LISTEN:-0}" = "1" ]; then
  ADDR="0.0.0.0"
fi

# python3 -m venv lays out bin/python on Linux/macOS but Scripts/python.exe
# when it runs against a native Windows Python (e.g. from git-bash).
if [ -x "$SOVITS_DIR/.venv/bin/python" ]; then
  PY="$SOVITS_DIR/.venv/bin/python"
elif [ -x "$SOVITS_DIR/.venv/Scripts/python.exe" ]; then
  PY="$SOVITS_DIR/.venv/Scripts/python.exe"
else
  echo "GPT-SoVITS is not installed yet; run scripts/sovits-setup.sh first." >&2
  exit 1
fi

# Fail fast with an actionable message instead of a buried uvicorn bind error
# if a previous run (or something else) is still holding the port.
existing_pid="$(netstat -ano 2>/dev/null | { grep -E "[[:space:]]$ADDR:$PORT[[:space:]].*LISTENING" || true; } | awk '{print $NF}' | head -n1)"
if [ -n "$existing_pid" ]; then
  echo "error: port $PORT is already in use by PID $existing_pid." >&2
  tasklist //FI "PID eq $existing_pid" 2>/dev/null >&2 || true
  echo "Stop it first: taskkill //F //PID $existing_pid" >&2
  exit 1
fi

echo "==> GPT-SoVITS API on http://$ADDR:$PORT  (POST /tts)"
cd "$SOVITS_DIR"
exec "$PY" api_v2.py -a "$ADDR" -p "$PORT" -c GPT_SoVITS/configs/tts_infer.yaml
