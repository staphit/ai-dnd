#!/usr/bin/env bash
# Start the local Stable Diffusion WebUI Forge server with the API enabled, so
# the backend's "local" image backend (FORGE_URL) can reach it.
#
#   scripts/forge.sh                 # http://127.0.0.1:7860 (API + web UI)
#   FORGE_PORT=7861 scripts/forge.sh
#   FORGE_LISTEN=1 scripts/forge.sh  # bind 0.0.0.0 — backend runs on another machine
#   FORGE_ARGS="--medvram-sdxl" scripts/forge.sh   # extra Forge flags
#
# First run installs Forge's own Python venv and torch; that is slow once.
set -euo pipefail
# Keep the window open if double-clicked and setup hasn't run yet, so the
# error is readable instead of flashing shut. Once the server starts below
# via exec, this trap no longer applies — the server owns the window.
trap 'read -n 1 -s -r -p "Press any key to close..."; echo' EXIT
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FORGE_DIR="$ROOT/vendor/stable-diffusion-webui-forge"
PORT="${FORGE_PORT:-7860}"

if [ ! -d "$FORGE_DIR" ]; then
  echo "Forge is not installed yet; run scripts/forge-setup.sh first." >&2
  exit 1
fi

ARGS="--api --port $PORT"
if [ "${FORGE_LISTEN:-0}" = "1" ]; then
  ARGS="$ARGS --listen"
fi
ARGS="$ARGS ${FORGE_ARGS:-}"

echo "==> Forge on http://127.0.0.1:$PORT  (API: /sdapi/v1)"
cd "$FORGE_DIR"
export COMMANDLINE_ARGS="$ARGS"
exec ./webui.sh
