#!/usr/bin/env bash
# One-time setup for local TTS: clones GPT-SoVITS into vendor/, creates its
# Python venv, installs requirements and downloads the pretrained models.
#
#   scripts/sovits-setup.sh
#
# Requirements: python3 (3.10+), ffmpeg on PATH. The pretrained download is
# several GB. After setup, record a 3–10 s narrator sample and point
# SOVITS_REF_AUDIO / SOVITS_PROMPT_TEXT in backend/.env at it.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOVITS_DIR="$ROOT/vendor/GPT-SoVITS"
SOVITS_REPO="https://github.com/RVC-Boss/GPT-SoVITS.git"

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "warning: ffmpeg not found on PATH; GPT-SoVITS needs it at runtime." >&2
fi

if [ -d "$SOVITS_DIR/.git" ]; then
  echo "==> GPT-SoVITS already cloned: $SOVITS_DIR"
else
  echo "==> Cloning GPT-SoVITS into $SOVITS_DIR"
  mkdir -p "$ROOT/vendor"
  git clone --depth 1 "$SOVITS_REPO" "$SOVITS_DIR"
fi

cd "$SOVITS_DIR"

if [ ! -d ".venv" ]; then
  echo "==> Creating Python venv"
  python3 -m venv .venv
fi
PY=".venv/bin/python"

echo "==> Installing requirements (slow the first time; installs torch)"
"$PY" -m pip install --upgrade pip
"$PY" -m pip install -r requirements.txt

echo "==> Downloading pretrained models (several GB, resumable)"
"$PY" -m pip install "huggingface_hub[cli]"
".venv/bin/huggingface-cli" download lj1995/GPT-SoVITS --local-dir GPT_SoVITS/pretrained_models

echo "==> Done. Next steps:"
echo "    1. Put a 3–10 s narrator sample somewhere on this machine (wav/mp3)."
echo "    2. In backend/.env set:"
echo "       SOVITS_REF_AUDIO=/absolute/path/to/sample.wav"
echo "       SOVITS_PROMPT_TEXT=<exact transcript of the sample>"
echo "    3. Start the server with scripts/sovits.sh"
