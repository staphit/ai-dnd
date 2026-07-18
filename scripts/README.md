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
| `build.sh` | Build frontend to `web-dist/`, backend to `backend/bin/dnd-server`, VS Code extension to `out/` — no serving. |
| `test.sh` | Backend `go vet` + `go test`, frontend typecheck + vitest, extension typecheck. Same suites as CI (`.github/workflows/ci.yml`). |

```bash
# Prefer the platform script directly:
./scripts/run-mac.sh          # macOS / Linux
./scripts/run-windows.sh      # Windows git-bash

# Or let the dispatcher pick:
./scripts/run.sh
```

## Local Stable Diffusion (SD Forge) — optional, `IMAGE_BACKEND=local`

| Script | Purpose | Windows |
|---|---|---|
| `forge-setup.sh` | Clone Forge into `vendor/`, optionally download a checkpoint (`--model juggernaut\|turbo\|hyper`). | OK — plain git clone + curl. |
| `forge.sh` | Start Forge with `--api`. | **Broken** — Forge's own `webui.sh` can't activate a native Windows venv and aborts immediately. Instead, double-click `vendor/stable-diffusion-webui-forge/webui-user.bat` (set `COMMANDLINE_ARGS=--api` in it first; `forge-setup.sh` output reminds you). |

## Double-click behaviour

`forge-setup.sh`, `forge.sh`, `run-mac.sh` and `run-windows.sh` pause on exit ("Press any key to close...") so errors are
readable when double-clicked from a file manager instead of the window
flashing shut. This does **not** catch a failure inside `exec` itself
(e.g. the target binary missing) — bash skips the trap in that case, which is
why `forge.sh` still just closes on Windows instead of showing the venv error.
