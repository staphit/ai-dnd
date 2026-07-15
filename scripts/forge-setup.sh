#!/usr/bin/env bash
# One-time setup for the local image backend: clones Stable Diffusion WebUI
# Forge into vendor/ and (optionally) downloads an SDXL checkpoint.
#
#   scripts/forge-setup.sh                  # clone only, pick a model later
#   scripts/forge-setup.sh --model juggernaut   # + JuggernautXL v9 (quality, ~7 GB)
#   scripts/forge-setup.sh --model lightning    # + DreamShaperXL Lightning (fast, ~7 GB)
#
# Windows: clone the same repo anywhere, put the checkpoint in
# models/Stable-diffusion, and add `set COMMANDLINE_ARGS=--api` to
# webui-user.bat instead of using these scripts.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FORGE_DIR="$ROOT/vendor/stable-diffusion-webui-forge"
FORGE_REPO="https://github.com/lllyasviel/stable-diffusion-webui-forge.git"

MODEL="none"
while [ $# -gt 0 ]; do
  case "$1" in
    --model) MODEL="${2:-none}"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

if [ -d "$FORGE_DIR/.git" ]; then
  echo "==> Forge already cloned: $FORGE_DIR"
else
  echo "==> Cloning Forge into $FORGE_DIR"
  mkdir -p "$ROOT/vendor"
  git clone --depth 1 "$FORGE_REPO" "$FORGE_DIR"
fi

MODELS_DIR="$FORGE_DIR/models/Stable-diffusion"
mkdir -p "$MODELS_DIR"

download() { # url, filename
  local url="$1" file="$MODELS_DIR/$2"
  if [ -f "$file" ]; then
    echo "==> Checkpoint already present: $2"
  else
    echo "==> Downloading $2 (several GB, resumable)"
    curl -L --fail --retry 3 -C - -o "$file" "$url"
  fi
}

case "$MODEL" in
  none)
    echo "==> No checkpoint requested. Put an SDXL .safetensors into:"
    echo "    $MODELS_DIR"
    ;;
  juggernaut)
    download \
      "https://huggingface.co/RunDiffusion/Juggernaut-XL-v9/resolve/main/Juggernaut-XL_v9_RunDiffusionPhoto_v2.safetensors" \
      "Juggernaut-XL_v9_RunDiffusionPhoto_v2.safetensors"
    echo "==> backend/.env suggestion:"
    echo "    FORGE_CHECKPOINT=Juggernaut-XL_v9_RunDiffusionPhoto_v2.safetensors"
    echo "    FORGE_PRESET=quality"
    ;;
  lightning)
    download \
      "https://huggingface.co/Lykon/dreamshaper-xl-lightning/resolve/main/DreamShaperXL_Lightning.safetensors" \
      "DreamShaperXL_Lightning.safetensors"
    echo "==> backend/.env suggestion:"
    echo "    FORGE_CHECKPOINT=DreamShaperXL_Lightning.safetensors"
    echo "    FORGE_PRESET=lightning"
    ;;
  *)
    echo "unknown --model $MODEL (use juggernaut | lightning | none)" >&2
    exit 1
    ;;
esac

echo "==> Done. Start the server with scripts/forge.sh (first start installs"
echo "    Forge's own Python venv and takes a while)."
