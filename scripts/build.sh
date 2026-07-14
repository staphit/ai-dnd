#!/usr/bin/env bash
# Full build: frontend bundle into web-dist/ and the backend binary into
# backend/bin/dnd-server.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "==> Building frontend -> web-dist/"
( cd "$ROOT/frontend" && npm install && npm run build )

echo "==> Building backend -> backend/bin/dnd-server"
( cd "$ROOT/backend" && go build -o "$ROOT/backend/bin/dnd-server" ./cmd/server )

echo "Done."
