# 安裝需求盤點（REQUIREMENTS）

本專案沒有單一 `requirements.txt`——各組件自管相依。本文盤點「要跑起來到底要裝多少東西」，按必要／選用分級。

## 總覽

| 組件 | 必要性 | 系統工具 | 套件數 |
|---|---|---|---|
| Go 後端 | 必要 | Go 1.24+ | 2 個直接模組（+12 間接），`go build` 自動下載 |
| React 前端 | 必要 | Node.js 20+、npm | 10 個 runtime + 14 個 dev 套件，`npm ci` 安裝 |
| AI DM | 必要（擇一） | Codex CLI（`codex login`）或 Grok CLI／`XAI_API_KEY`；都沒有可用示範 DM | — |
| VS Code 擴充套件 | 選用 | Node.js 20+、VS Code 1.95+ | 4 個 dev 套件（repo 根目錄 `npm ci`） |
| 本地圖片（SD Forge） | 選用 | git、curl、Python（Forge 首次啟動自建 venv） | Forge 自身相依 + checkpoint 數 GB |

Windows 另需 **git-bash**（所有 `scripts/*.sh` 都是 bash）。

## 必要：核心網站

最小可玩組合 = Go + Node + （Codex CLI 或示範 DM）。

### Go 後端（`backend/`）

- Go **1.24+**
- 直接相依（`backend/go.mod`）：
  - `github.com/go-chi/chi/v5` — HTTP router
  - `modernc.org/sqlite` — 純 Go SQLite（**免 CGO、免安裝 sqlite**）
- 12 個間接模組由 `go.sum` 鎖定；`go build ./cmd/server` 時自動抓取，無須手動安裝。

### React 前端（`frontend/`）

`cd frontend && npm ci` 一次裝完。

- Runtime（10）：`react`、`react-dom`、`three`、`@react-three/fiber`、`@react-three/drei`、`framer-motion`、`@phosphor-icons/react`、3 個字型套件（`@fontsource*`）
- Dev（14）：`vite`、`@vitejs/plugin-react`、`typescript`、`tailwindcss`、`@tailwindcss/vite`、`vitest`、`jsdom`、`@testing-library/*`（3）、`@types/*`（4）

### AI DM（擇一；沒有也能用示範 DM 跑完整流程）

| 選項 | 安裝 | 登入／金鑰 |
|---|---|---|
| Codex CLI（預設） | 安裝 Codex CLI | `codex login`（ChatGPT 登入，免 API key） |
| Grok CLI | `curl -fsSL https://x.ai/cli/install.sh \| bash` | `grok login`（SuperGrok／X Premium）；或設 `XAI_API_KEY` 走 HTTP API |
| 示範 DM | 免安裝 | 首頁切換 |

## 選用組件

### VS Code 擴充套件（repo 根目錄）

- Node.js 20+、VS Code **1.95+**（需 Language Model API 與已設定的 Chat 模型）
- `npm ci`：4 個 dev 套件（`typescript`、`@vscode/vsce`、`@types/node`、`@types/vscode`）
- `npm run compile` 後 F5 啟動

### 本地圖片生成 — Stable Diffusion WebUI Forge（`IMAGE_BACKEND=local`）

- 安裝：`scripts/forge-setup.sh [--model juggernaut|turbo|hyper]`（clone 進 `vendor/` + 下載 checkpoint 5–7 GB）
- 系統：git、curl；Forge 首次啟動自建 Python venv 並安裝 torch（慢一次）
- 硬體：建議 NVIDIA GPU（SDXL quality 需較多 VRAM；`turbo`／`hyper` 為低 VRAM 選項）
- 啟動：`scripts/forge.sh`（macOS／Linux）；**Windows 改用** `vendor/stable-diffusion-webui-forge/webui-user.bat`（先加 `set COMMANDLINE_ARGS=--api`）
- 不裝也能用 Codex（`codex login`）或 Grok（`XAI_API_KEY`）生圖

## 安裝指令速查

```bash
# 必要
cd backend && go build ./cmd/server     # Go 模組自動下載
cd frontend && npm ci                   # 前端套件
codex login                             # AI DM（或 grok login／示範 DM）

# 選用
npm ci && npm run compile               # VS Code 擴充（repo 根目錄）
./scripts/forge-setup.sh --model turbo  # 本地圖片
```

磁碟空間粗估：核心 < 1 GB（node_modules + Go cache）；+Forge 約 10–15 GB。
