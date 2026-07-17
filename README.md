# D&D Duet AI

這個專案包含兩種本機 D&D 介面：

- 瀏覽器網站：1–4 人遊戲桌、2024 角色規則、骰盤、戰役紀錄與 CLI Dungeon Master
- VS Code 擴充套件：較精簡的側邊欄版本，使用 VS Code Language Model API

瀏覽器網站前後端分離：

- `frontend/`：React／Vite 前端（UI 未變動）
- `backend/`：Go 後端（`net/http` + chi），封裝 Codex CLI、DM 裁定、圖片生成；生成圖存於本機 `generated-images/`（使用者刪除前會保留），敘事記憶在 SQLite（`campaign-data/dnd-duet.db`）
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
- 三幕劇本節奏（前／中／後期各 20 回合）與限時達標 XP 獎勵
- 金幣經濟：寶箱掉落、裝備商店買賣、鍛造商強化武器與護甲（上限 +3）
- 輕型武器每動作攻擊 2 次；任務掉落武器可鑑定成可用攻擊
- 倒地救援機制與全隊倒地時「結束故事／戰鬥重來」選擇
- DM 資料源可選 Codex（ChatGPT 登入）或 Grok（`grok login`／`XAI_API_KEY`）
- 3D 地城主桌：說話、思考、擲骰成功／失敗動畫
- 桌面、平板與手機響應式布局

### 啟動開發模式

```bash
codex login
./scripts/dev.sh
```

`dev.sh` 會同時啟動 Go 後端（`127.0.0.1:4318`）與 Vite 前端（`127.0.0.1:4317`，`/api` 與 `/generated` 代理到後端）。瀏覽器開啟 `http://127.0.0.1:4317`。

### 啟動正式版本

```bash
./scripts/run-mac.sh        # macOS / Linux
./scripts/run-windows.sh    # Windows（git-bash）
# 或由 run.sh 依 OS 自動選擇：
./scripts/run.sh
```

建置前端與後端後，由後端服務整個網站。瀏覽器開啟 `http://127.0.0.1:4318`。改用其他埠：`PORT=8080 ./scripts/run-mac.sh`。重新啟動（停止舊行程並重建）：`./scripts/restart.sh`。

## 怎麼玩（玩家指南）

### 開團

1. 啟動網站（見上方），首次進入會出現開團設定
2. 選擇故事模板（或自訂標題、場景、目標），建立 1–4 名角色：12 個核心職業、種族、背景、能力值都可調整
3. 完成後進入遊戲桌；每位角色開局帶 100 gp
4. 右上若顯示「需要連線」，按「連線 Codex／Grok」讓 DM 上線（每個故事各自一條連線；切換故事後要重新連線）

### 一回合怎麼進行

1. 每位玩家在自己的操作區輸入行動（或點選 DM 給的建議），按「鎖定行動」
2. **全員鎖定後**故事才會推進；最後一人送出前都可以解鎖修改
3. DM 回覆公開敘事，有時附帶只有單一玩家看得到的私密訊息（左上「訊息視角」切換）
4. 行動不合理時 DM 會駁回並說明理由，修改後重新鎖定即可
5. 對上一則敘事不滿意，可按「修正上一則 DM 敘事」用自然語言要求重寫

### 擲骰

- DM 要求檢定時，畫面會出現**必要骰盤**：按「擲 d20 並自動加值」，加值與 DC 由伺服器依角色卡計算，結果自動寫入故事
- 3D 地城主桌上的骰子會跟著你的擲骰播放成功／失敗動畫

### 戰鬥

- DM 宣布戰鬥開始時自動擲先攻並排序；也可在戰鬥區手動建立敵人開打
- 輪到你時可以：
  - **攻擊**：下拉切換武器（顯示強化 +N 與每動作攻擊次數；輕型武器每動作打 2 次）
  - **施放法術**：消耗法術位，命中／豁免／傷害由伺服器結算
  - **職業資源**：回氣（附贈動作回 1d10+等級）、動作如潮（本回合多一個動作）等一鍵使用
  - **消耗品**：治療藥水（附贈動作回 2d4+2）、解毒劑
  - **救援**：花一個動作扶起倒地（0 HP）的隊友，倒地者花生命骰回血站起
- 敵方回合由 AI 自動選擇目標結算
- **全隊倒地**時跳出選擇：「戰鬥重來」回到本場戰鬥開始的狀態重打；「結束故事」由 DM 寫下終章
- 戰鬥結束按「結束戰鬥並敘述」，勝利自動發 XP，DM 接著敘述後果

### 劇本節奏（三幕）

- 每個戰役分**前期／中期／後期**三階段，各約 20 回合，任務摘要上方有進度條
- 在期限內完成階段目標，每人獲得 250／400／600 XP 限時獎勵；超過期限 DM 會主動把關鍵事件推到你面前
- 三階段完成後故事收尾

### 成長

- 升級門檻很低（2 級 100 XP、4 級 500 XP…），幾場遭遇就能升級
- 「角色」頁升級：每級 +5 能力點自由分配（單項上限 20）；能力值即時反映到命中、傷害、AC、HP、先攻與法術 DC
- 短休（1 點行動時間）自動花生命骰回血；長休（4 點）完整恢復

### 金錢與裝備

- 寶箱、任務報酬會發金幣（全隊平分）與物品
- 場景列的「裝備商店」（非戰鬥時可用）：
  - **購買**：武器買了就能用（自動成為攻擊選項）、護甲、藥水、雜項
  - **鍛造商**：武器強化 +1 命中/+1 傷害（100 gp × 等級）、護甲 +1 AC（150 gp × 等級），上限 +3
  - **賣出**：目錄品半價、故事道具 5 gp
- 任務掉落的武器：宣告「鑑定〈物品名〉」請 DM 給數值，之後就會出現在攻擊選單

### 場景插圖（選用）

- 對話上方是每一幕的圖片列：已生成的顯示縮圖、未生成的按「生成」補畫該幕（會先跳確認）
- 每張圖記錄當時的 prompt（滑鼠停留可見）；生成在背景進行，不會卡住劇情
- 設定頁可開「每回合自動生成」；圖片引擎可選 Codex、Grok 或本地 SD Forge

### 存檔

- 進度全部保存在本機伺服器（SQLite），瀏覽器重開即續玩
- 設定頁可建立多個戰役、複製、匯出／匯入 JSON、刪除

### Codex 連線模式

DM 回合預設走**長連線**（`CODEX_MODE=app-server`）：後端啟動時保持一個 `codex app-server` 常駐行程，DM 回合透過其持久 JSON-RPC/stdio 連線以 `turn/start` + `outputSchema` 取得結構化裁定，**不再每回合重新 spawn**（單一常駐行程服務所有回合）。圖片生成仍走 `codex exec`。`CODEX_MODEL` 需為目前 Codex CLI 支援的模型。

如需回到每次請求 spawn 的模式：

```bash
CODEX_MODE=exec ./scripts/run-mac.sh   # 或 run-windows.sh / run.sh
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

DM 流程使用 `codex exec --ephemeral --sandbox read-only --output-schema`，回傳經 JSON Schema 約束的裁定結果。場景插圖明確呼叫 `$imagegen` 的內建工具；原圖先由 Codex 保存於個人 `generated_images`，後端驗證工作識別碼後把圖片位元組寫入本機 `generated-images/`（可用 `DND_IMAGE_DIR` 覆寫），再由 `/generated/<檔名>` 服務。圖片會持久保存，直到使用者在 UI 刪除（`DELETE /api/generated/<檔名>`）。

目前後端只監聽 `127.0.0.1`，不會直接暴露到區域網路。Codex 未安裝或未登入時，可在首頁切換示範 DM。

圖片不會每回合自動生成（可在設定開啟自動生成）。玩家按下「生成場景」後會產生一張 3:2 場景圖並寫入 `generated-images/`（不進 Git）。文字與圖片都會計入目前 ChatGPT 方案的 Codex 使用限制，但不需要 OpenAI Platform API Key。

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
