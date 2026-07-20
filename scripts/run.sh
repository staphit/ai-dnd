#!/usr/bin/env bash
# OS dispatcher: picks run-mac.sh or run-windows.sh.
# Prefer calling the platform script directly if you know your OS.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

case "$(uname -s 2>/dev/null || echo unknown)" in
  Darwin|Linux)
    exec "$ROOT/scripts/run-mac.sh" "$@"
    ;;
  MINGW*|MSYS*|CYGWIN*|Windows_NT)
    exec "$ROOT/scripts/run-windows.sh" "$@"
    ;;
  *)
    # git-bash sometimes reports a generic uname; fall back on Windows env.
    if [ -n "${WINDIR:-}" ] || [ -n "${SYSTEMROOT:-}" ]; then
      exec "$ROOT/scripts/run-windows.sh" "$@"
    fi
    echo "error: unsupported OS '$(uname -s 2>/dev/null || echo unknown)'." >&2
    echo "Use scripts/run-mac.sh (macOS/Linux) or scripts/run-windows.sh (Windows git-bash)." >&2
    exit 1
    ;;
esac
