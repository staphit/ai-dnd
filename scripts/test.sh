#!/usr/bin/env bash
# Run every test suite: Go backend and the frontend (Vitest).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "==> Backend (go test)"
( cd "$ROOT/backend" && go test ./... )

echo "==> Frontend (vitest)"
( cd "$ROOT/frontend" && npm install && npm test )

echo "All tests passed."
