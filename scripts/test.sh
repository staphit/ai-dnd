#!/usr/bin/env bash
# Run every static check and test suite in the repository.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "==> Backend (vet + test)"
( cd "$ROOT/backend" && go vet ./... && go test ./... )

echo "==> Frontend (typecheck + vitest)"
( cd "$ROOT/frontend" && if [ ! -d node_modules ]; then npm ci; fi && npm run check && npm test )

echo "==> VS Code extension (typecheck)"
( cd "$ROOT" && if [ ! -d node_modules ]; then npm ci; fi && npm run check )

echo "All tests passed."
