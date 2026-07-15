#!/usr/bin/env bash
# Start the local GPT-SoVITS api_v2 server for the backend's /api/tts endpoint.
#
#   scripts/sovits.sh                  # http://127.0.0.1:9880
#   SOVITS_PORT=9881 scripts/sovits.sh
#   SOVITS_LISTEN=1 scripts/sovits.sh  # bind 0.0.0.0 — backend runs on another machine
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOVITS_DIR="$ROOT/vendor/GPT-SoVITS"
PORT="${SOVITS_PORT:-9880}"
ADDR="127.0.0.1"
if [ "${SOVITS_LISTEN:-0}" = "1" ]; then
  ADDR="0.0.0.0"
fi

if [ ! -d "$SOVITS_DIR/.venv" ]; then
  echo "GPT-SoVITS is not installed yet; run scripts/sovits-setup.sh first." >&2
  exit 1
fi

echo "==> GPT-SoVITS API on http://$ADDR:$PORT  (POST /tts)"
cd "$SOVITS_DIR"
exec ".venv/bin/python" api_v2.py -a "$ADDR" -p "$PORT" -c GPT_SoVITS/configs/tts_infer.yaml
