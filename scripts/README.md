# scripts/

All scripts are bash — run from git-bash on Windows, or a normal shell on
Linux/macOS. Most scripts are cross-platform through git-bash **except**
where noted. Production serve is split by OS (see below). Install
prerequisites are inventoried in [`../REQUIREMENTS.md`](../REQUIREMENTS.md).

## App

| Script | Purpose |
|---|---|
| `dev.sh` | Backend + Vite dev server together, hot reload. |
| `run.sh` | OS dispatcher → `run-mac.sh` or `run-windows.sh`. |
| `run-mac.sh` | **macOS / Linux:** build frontend + backend (`lsof` port check), serve from Go binary. |
| `run-windows.sh` | **Windows (git-bash):** same, with `netstat`/`taskkill` and `.exe` binary name. |
| `restart.sh` | Stop whatever's on `PORT`, rebuild, run again. `lsof` on macOS/Linux, `netstat`/`taskkill` fallback on git-bash. |
| `build.sh` | Build frontend to `web-dist/`, backend to `backend/bin/dnd-server`, VS Code extension to `vscode-extension/out/` — no serving. |
| `test.sh` | Backend `go vet` + `go test`, frontend typecheck + vitest, extension typecheck. Same suites as CI (`.github/workflows/ci.yml`). |

```bash
# Prefer the platform script directly:
./scripts/run-mac.sh          # macOS / Linux
./scripts/run-windows.sh      # Windows git-bash

# Or let the dispatcher pick:
./scripts/run.sh
```

## Image generation

Scene and portrait art use **Codex `$imagegen` (GPT) only**. Local SD Forge and
Grok Imagine backends have been removed from the app. The `forge-setup.sh` /
`forge.sh` scripts under this folder (if still present) are legacy and unused.

## Double-click behaviour

`run-mac.sh` and `run-windows.sh` pause on exit ("Press any key to close...") so
errors are readable when double-clicked from a file manager instead of the
window flashing shut.
