#!/usr/bin/env bash
# One-time setup for the local image backend: clones Stable Diffusion WebUI
# Forge into vendor/ and (optionally) downloads an SDXL checkpoint.
#
#   scripts/forge-setup.sh                  # clone only, pick a model later
#   scripts/forge-setup.sh --model juggernaut   # + JuggernautXL v9 (quality, ~7 GB)
#   scripts/forge-setup.sh --model turbo        # + SD Turbo (low VRAM, ~5 GB)
#   scripts/forge-setup.sh --model hyper        # + Hyper-SD15 LoRA (epiCRealism from Civitai, lowest VRAM)
#
# Windows: this script (clone + checkpoint download) works fine from
# git-bash. Only scripts/forge.sh (the launcher) does not — Forge's own
# webui.sh can't activate a native Windows venv. Start the server by
# double-clicking vendor/stable-diffusion-webui-forge/webui-user.bat instead
# (add `set COMMANDLINE_ARGS=--api` in it first).
set -euo pipefail
# Keep the window open when double-clicked from a file manager so errors are
# readable instead of flashing shut.
trap 'read -n 1 -s -r -p "Press any key to close..."; echo' EXIT
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
LORA_DIR="$FORGE_DIR/models/Lora"
mkdir -p "$MODELS_DIR" "$LORA_DIR"

download() { # url, filename
  local url="$1" file="$MODELS_DIR/$2"
  if [ -f "$file" ]; then
    echo "==> Checkpoint already present: $2"
  else
    echo "==> Downloading $2 (several GB, resumable)"
    curl -L --fail --retry 3 -C - -o "$file" "$url"
  fi
}

download_lora() { # url, filename
  local url="$1" file="$LORA_DIR/$2"
  if [ -f "$file" ]; then
    echo "==> LoRA already present: $2"
  else
    echo "==> Downloading LoRA $2 (resumable)"
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
  turbo)
    download \
      "https://huggingface.co/stabilityai/sd-turbo/resolve/main/sd_turbo.safetensors" \
      "sd_turbo.safetensors"
    echo "==> backend/.env suggestion (low VRAM, e.g. running alongside TTS):"
    echo "    FORGE_CHECKPOINT=sd_turbo.safetensors"
    echo "    FORGE_PRESET=turbo"
    ;;
  hyper)
    # Hyper-SD 8-step LoRA (ByteDance, reliable on HF); run it at 6–8 steps.
    download_lora \
      "https://huggingface.co/ByteDance/Hyper-SD/resolve/main/Hyper-SD15-8steps-lora.safetensors" \
      "Hyper-SD15-8steps-lora.safetensors"
    # ADetailer extension for face fixing (downloads its YOLO models on first run).
    ADETAILER_DIR="$FORGE_DIR/extensions/adetailer"
    if [ -d "$ADETAILER_DIR/.git" ]; then
      echo "==> ADetailer extension already installed."
    else
      echo "==> Installing ADetailer extension"
      git clone --depth 1 "https://github.com/Bing-su/adetailer.git" "$ADETAILER_DIR"
    fi
    echo "==> LoRA + ADetailer installed. Two files you must fetch manually from"
    echo "    Civitai (login required) and drop in place:"
    echo "    1. epiCRealism (SD1.5) -> $MODELS_DIR"
    echo "       https://civitai.com/models/25694"
    echo "    2. add_detail / Detail Tweaker LoRA -> $LORA_DIR (save as add_detail.safetensors)"
    echo "       https://civitai.com/models/58390"
    echo "==> backend/.env suggestion (finer detail, uses spare VRAM):"
    echo "    FORGE_CHECKPOINT=<the epiCRealism file name you saved>"
    echo "    FORGE_PRESET=hyper"
    echo "    FORGE_LORA=<lora:Hyper-SD15-8steps-lora:1>, <lora:add_detail:0.6>"
    echo "    FORGE_ADETAILER=face_yolov8n.pt"
    ;;
  *)
    echo "unknown --model $MODEL (use juggernaut | turbo | hyper | none)" >&2
    exit 1
    ;;
esac

echo "==> Done. Start the server with scripts/forge.sh (first start installs"
echo "    Forge's own Python venv and takes a while)."
