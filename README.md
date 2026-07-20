# D&D Duet AI

這個專案包含兩種本機 D&D 介面：

- 瀏覽器網站：1–4 人遊戲桌、2024 角色規則、骰盤、戰役紀錄與 CLI Dungeon Master
- VS Code 擴充套件：較精簡的側邊欄版本，使用 VS Code Language Model API

瀏覽器網站前後端分離：

- `frontend/`：React／Vite 前端（UI 未變動）
- `backend/`：Go 後端（`net/http` + chi），封裝 Codex CLI、DM 裁定、圖片生成，並以 SQLite（`campaign-data/dnd-duet.db`）保存生成的圖片
- 後端提供 `/api/status`、`/api/dm`、`/api/scene-image`、`/api/character-image` 與 `/generated/<檔名>`，並在正式模式下同時服務已建置的前端

需求：Go 1.24+、Node.js 20+，以及 Codex CLI。

## 本機網站

### 功能

- 1–4 名玩家分別鎖定行動，全隊提交後才推進場景
- 2024 第五版／SRD 5.2.1 的 12 個核心職業與 3 級子職業
- 完整能力值、技能、豁免、HP、AC、攻擊、裝備與職業資源
- 分環法術位、戲法、準備法術、法師法術書、專注與儀式施法
- 短休自動擲生命骰、長休完整恢復，以及施法和職業資源消耗
- 可按角色與技能自動套用加值的公開多面骰
- 瀏覽器自動存檔及戰役紀錄頁
- 本機 `codex exec` 結構化地城主，不需要 API Key
- Codex `$imagegen` 場景插圖，玩家手動觸發
- 完整戰鬥追蹤：先攻排序、命中 AC、自然 20 重擊、暫時生命、傷害與生命同步
- 治療／傷害／狀態法術選擇目標後自動結算；DM 劇情效果經白名單驗證後同步角色卡
- 玩家不能任意加減 HP；職業資源只能使用，並依短休／長休規則恢復
- 等級 1–20、升級、多職業，以及種族、背景、能力值與法術配置
- DM 公開敘事與逐玩家私密訊息視角
- 多戰役 vault、JSON 匯出／匯入；舊 `dnd-duet-web-v1` 存檔採非破壞遷移
- 每個戰役可選擇 Codex 預設、GPT-5.6 Sol／Terra／Luna 或 GPT-5.6
- 沒有模型時可切換示範 DM，測試完整遊戲流程
- 桌面、平板與手機響應式布局

### 啟動開發模式

```bash
codex login
./scripts/dev.sh
```

`dev.sh` 會同時啟動 Go 後端（`127.0.0.1:4318`）與 Vite 前端（`127.0.0.1:4317`，`/api` 與 `/generated` 代理到後端）。瀏覽器開啟 `http://127.0.0.1:4317`。

### 啟動正式版本

```bash
./scripts/run.sh        # 建置前端與後端，再由後端服務整個網站
```

瀏覽器開啟 `http://127.0.0.1:4318`。改用其他埠：`PORT=8080 ./scripts/run.sh`。重新啟動（停止舊行程並重建）：`./scripts/restart.sh`。

### Codex 連線模式

DM 回合預設走**長連線**（`CODEX_MODE=app-server`）：後端啟動時保持一個 `codex app-server` 常駐行程，DM 回合透過其持久 JSON-RPC/stdio 連線以 `turn/start` + `outputSchema` 取得結構化裁定，**不再每回合重新 spawn**（單一常駐行程服務所有回合）。圖片生成仍走 `codex exec`。`CODEX_MODEL` 需為目前 Codex CLI 支援的模型。

如需回到每次請求 spawn 的模式：

```bash
CODEX_MODE=exec ./scripts/run.sh
```

### 自動測試

```bash
./scripts/test.sh                    # 後端 go test 全部 + 前端 vitest 全部
# 或分別執行：
cd backend && go test ./...
cd frontend && npm test
```

後端測試涵蓋 DM 提示組建（與原 Node 版位元組相同）、DM 輸出驗證與強制結構、模型正規化、JS 語意轉換、SQLite 圖片存取（含並發寫入）與 HTTP 端點；測試不會呼叫 Codex。前端測試涵蓋 12 職業角色建立、法術與法術位、資源和休息、動態隊伍回合與開團表單。

### 設定 Codex CLI

網站重用 Codex CLI 的本機 ChatGPT 登入，不需要 `OPENAI_API_KEY`。先安裝 Codex CLI，再確認登入狀態：

```bash
codex login
codex login status
```

可選擇複製 `backend/.env.example` 到 `backend/.env`（或 repo 根目錄 `.env`）設定：

```text
CODEX_CLI_PATH=codex
CODEX_MODEL=
PORT=4318
```

`CODEX_MODEL` 留空時使用 Codex CLI 的預設模型。後端啟動 Codex 子程序時會移除 `OPENAI_API_KEY` 與 `CODEX_API_KEY`，確保走 ChatGPT 登入而不是 API 計費。

DM 流程使用 `codex exec --ephemeral --sandbox read-only --output-schema`，回傳經 JSON Schema 約束的裁定結果。場景插圖明確呼叫 `$imagegen` 的內建工具；原圖先由 Codex 保存於個人 `generated_images`，後端驗證工作識別碼後把圖片位元組寫入 SQLite（`campaign-data/dnd-duet.db`），再由 `/generated/<檔名>` 服務。

目前後端只監聽 `127.0.0.1`，不會直接暴露到區域網路。Codex 未安裝或未登入時，可在首頁切換示範 DM。

圖片不會每回合自動生成。玩家按下「生成場景」後才會產生一張 3:2 場景圖並存入 SQLite；資料庫位於 `campaign-data/`，不會進入 Git 或 VSIX。文字與圖片都會計入目前 ChatGPT 方案的 Codex 使用限制，但不需要 OpenAI Platform API Key。

模型選擇只套用在之後送出的 DM 回合，不會重新生成或修改既有故事。可用模型仍取決於目前帳號方案；未開放的模型會讓該次請求顯示錯誤。多戰役資料保存在 `dnd-duet-web-v2-vault`，第一次啟動只會複製舊存檔，不會刪除 `dnd-duet-web-v1`。匯入 JSON 預設只加入資料庫，不會自動切換目前戰役。

## VS Code 擴充套件

### 功能

- 兩名玩家分別提交行動，避免先提交者替另一人作決定
- 兩人都提交後，由 VS Code Language Model API 呼叫 AI DM
- 內建 d20 擲骰
- 自動保存 `campaign-data/campaign.json`
- 自動產生可閱讀的 `campaign-data/session-log.md`
- 不保存模型 API Key；使用 VS Code 已設定的語言模型

### 執行

需求：Node.js 20+、支援 Language Model API 的新版 VS Code，以及一個已設定的 VS Code Chat 模型。

```bash
npm install
npm run compile
```

在 VS Code 開啟本資料夾，按 `F5`。新的 Extension Development Host 開啟後，點左側的 d20 圖示。

如果尚未設定模型，先在 VS Code Chat 的模型選擇器登入，或加入自己的模型供應商。

### 使用

1. 執行命令 `D&D Duet: 建立新戰役`。
2. 玩家一與玩家二分別輸入行動。
3. 兩位都按下「提交行動」後，AI DM 推進場景。
4. 需要檢定時按「擲 d20」，再於下一次行動中描述結果。

### 資料位置

開啟資料夾時，戰役存在工作區內的 `campaign-data/`。若沒有開啟資料夾，則使用 VS Code 的擴充套件全域儲存空間。

## 後續功能

- 場景／NPC Markdown 知識庫搜尋
- 視需要接入 LangGraph 本機服務

## 規則授權

角色與法術規則以官方 SRD 5.2.1 為基礎，依 CC-BY-4.0 使用。完整歸屬聲明請見 `NOTICE.md`。
