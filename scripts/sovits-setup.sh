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
# Keep the window open when double-clicked from a file manager so errors are
# readable instead of flashing shut.
trap 'read -n 1 -s -r -p "Press any key to close..."; echo' EXIT
# Windows consoles default to a legacy codepage (e.g. cp950) that can't
# encode emoji some tools print (huggingface_hub's deprecation warning);
# force UTF-8 so those prints don't crash the process.
export PYTHONUTF8=1
export PYTHONIOENCODING=utf-8
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
# python3 -m venv lays out bin/python on Linux/macOS but Scripts/python.exe
# when it runs against a native Windows Python (e.g. from git-bash).
if [ -x ".venv/bin/python" ]; then
  PY=".venv/bin/python"
  HFCLI=".venv/bin/hf"
else
  PY=".venv/Scripts/python.exe"
  HFCLI=".venv/Scripts/hf.exe"
fi

echo "==> Installing requirements (slow the first time; installs torch)"
"$PY" -m pip install --upgrade pip
# requirements.txt pulls in plain "torchaudio" with no CUDA index, which
# resolves to a CPU-only wheel on Windows and runs GPT-SoVITS on CPU even
# with a GPU present. Install the CUDA build first so pip sees it already
# satisfied and leaves it alone. Do NOT also install torchcodec: newer
# torchaudio only needs it for its own load_with_torchcodec path, which
# additionally requires FFmpeg's "full-shared" build (DLLs) that the
# common winget/static FFmpeg package doesn't ship — GPT-SoVITS's
# torchaudio.load() calls work fine without it via the soundfile backend.
"$PY" -m pip install torch==2.5.1+cu121 torchaudio==2.5.1+cu121 --index-url https://download.pytorch.org/whl/cu121
"$PY" -m pip install -r requirements.txt

# fast_langdetect needs this directory to already exist for its model cache.
mkdir -p GPT_SoVITS/pretrained_models/fast_langdetect

echo "==> Downloading pretrained models (several GB, resumable)"
"$PY" -m pip install "huggingface_hub[cli]"
"$HFCLI" download lj1995/GPT-SoVITS --local-dir GPT_SoVITS/pretrained_models

echo "==> Done. Next steps:"
echo "    1. Put a 3–10 s narrator sample somewhere on this machine (wav/mp3)."
echo "    2. In backend/.env set:"
echo "       SOVITS_REF_AUDIO=/absolute/path/to/sample.wav"
echo "       SOVITS_PROMPT_TEXT=<exact transcript of the sample>"
echo "    3. Start the server with scripts/sovits.sh"
