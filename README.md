# D&D Duet AI

這個專案包含兩種本機 D&D 介面：

- 瀏覽器網站：1–4 人遊戲桌、2024 角色規則、骰盤、戰役紀錄與 OpenAI Dungeon Master Agent
- VS Code 擴充套件：較精簡的側邊欄版本，使用 VS Code Language Model API

## 本機網站

### 功能

- 1–4 名玩家分別鎖定行動，全隊提交後才推進場景
- 2024 第五版／SRD 5.2.1 的 12 個核心職業與 3 級子職業
- 完整能力值、技能、豁免、HP、AC、攻擊、裝備與職業資源
- 分環法術位、戲法、準備法術、法師法術書、專注與儀式施法
- 短休／長休恢復，以及施法和職業資源消耗
- 可按角色與技能自動套用加值的公開多面骰
- 瀏覽器自動存檔及戰役紀錄頁
- 官方 OpenAI Agents SDK 地城主
- `gpt-image-2` 場景插圖，玩家手動觸發
- 沒有模型時可切換示範 DM，測試完整遊戲流程
- 桌面、平板與手機響應式布局

### 啟動開發模式

```bash
npm install
npm run web:dev
```

瀏覽器開啟 `http://127.0.0.1:4317`。

### 啟動正式版本

```bash
npm run web:build
npm run web:start
```

瀏覽器開啟 `http://127.0.0.1:4318`。

### 自動測試

```bash
npm test
npm run web:check
npm run web:server-check
```

`npm test` 會檢查 12 職業角色建立、法術與法術位、資源和休息、動態隊伍回合、開團表單，以及送往 AI 地城主的完整角色快照；測試不會呼叫 OpenAI API。

### 設定 OpenAI Agent

複製環境變數範例，然後只在本機的 `.env` 填入 OpenAI API Key：

```bash
cp .env.example .env
```

```text
OPENAI_API_KEY=你的金鑰
OPENAI_MODEL=gpt-5.6-terra
OPENAI_IMAGE_MODEL=gpt-image-2
```

`.env` 已列入 `.gitignore`，金鑰只由本機 Node 伺服器讀取，不會進入前端 bundle。Agent tracing 預設關閉，避免另外記錄戰役內容；如需 OpenAI Dashboard traces，可設定 `OPENAI_AGENT_TRACING=1`。

目前伺服器只監聽 `127.0.0.1`，不會直接暴露到區域網路。未設定金鑰時可在首頁切換示範 DM。

### 模型選擇

- `gpt-5.6-terra`：預設，適合長篇劇情一致性與成本平衡
- `gpt-5.6-luna`：回應較經濟，適合頻繁短回合
- `gpt-5.6-sol`：品質優先，適合重要劇情或複雜規則裁定
- `gpt-image-2`：場景插圖模型

圖片不會每回合自動生成。玩家按下「生成場景」後才會產生一張 1536×1024 中等品質 JPEG，並保存在 `campaign-data/images/`；該資料夾不會進入 Git 或 VSIX。

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

- 完整戰鬥先攻序列、命中與傷害自動結算
- 更多角色等級、升級流程與多職業
- 自訂種族、背景、能力值與法術配置
- DM 公開訊息與玩家私密訊息
- 場景／NPC Markdown 知識庫搜尋
- 匯出與載入多個戰役
- 選擇指定模型
- 視需要接入 LangGraph 本機服務

## 規則授權

角色與法術規則以官方 SRD 5.2.1 為基礎，依 CC-BY-4.0 使用。完整歸屬聲明請見 `NOTICE.md`。
