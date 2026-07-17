# scripts/

All scripts are bash — run from git-bash on Windows, or a normal shell on
Linux/macOS. Most scripts are cross-platform through git-bash **except**
where noted. Production serve is split by OS (see below).

## App

| Script | Purpose |
|---|---|
| `dev.sh` | Backend + Vite dev server together, hot reload. |
| `run.sh` | OS dispatcher → `run-mac.sh` or `run-windows.sh`. |
| `run-mac.sh` | **macOS / Linux:** build frontend + backend (`lsof` port check), serve from Go binary. |
| `run-windows.sh` | **Windows (git-bash):** same, with `netstat`/`taskkill` and `.exe` binary name. |
| `restart.sh` | Stop whatever's on `PORT`, rebuild, run again. Needs `lsof` — not present on git-bash by default. |
| `build.sh` | Build frontend to `web-dist/` and backend to `backend/bin/dnd-server`, no serving. |
| `test.sh` | `go test ./...` + `npm test`. |

```bash
# Prefer the platform script directly:
./scripts/run-mac.sh          # macOS / Linux
./scripts/run-windows.sh      # Windows git-bash

# Or let the dispatcher pick:
./scripts/run.sh
```


## Local Stable Diffusion (SD Forge)

| Script | Purpose | Windows |
|---|---|---|
| `forge-setup.sh` | Clone Forge into `vendor/`, optionally download a checkpoint (`--model juggernaut\|lightning\|turbo`). | OK — plain git clone + curl. |
| `forge.sh` | Start Forge with `--api`. | **Broken** — Forge's own `webui.sh` can't activate a native Windows venv and aborts immediately. Instead, double-click `vendor/stable-diffusion-webui-forge/webui-user.bat` (set `COMMANDLINE_ARGS=--api` in it first; `forge-setup.sh` output reminds you). |

## Local TTS (GPT-SoVITS)

| Script | Purpose | Windows |
|---|---|---|
| `sovits-setup.sh` | Clone GPT-SoVITS into `vendor/`, create its venv, install requirements, download pretrained models. | OK. |
| `sovits.sh` | Start the `api_v2` server. | OK — auto-detects `.venv/bin/python` (Linux/macOS) vs `.venv/Scripts/python.exe` (Windows). |

Both `*-setup.sh` and the two server scripts pause on exit ("Press any key to
close...") so errors are readable when double-clicked from a file manager
instead of the window flashing shut. This does **not** catch a failure
inside `exec` itself (e.g. the target binary missing) — bash skips the trap
in that case, which is why `forge.sh` still just closes on Windows instead
of showing the venv error.
