# AI Dungeon Master

本機 D&D 遊戲桌：React 前端 + Go 後端，AI 地城主走本機 Codex CLI（ChatGPT 登入）或 Grok CLI，不需要 API key。1–4 名玩家、SRD 5.2.1 的 6 個職業、完整戰鬥與法術結算、劇本／自由兩種模式、場景插圖。

## 需求

- Go 1.24+
- Node.js 20+
- Codex CLI（`codex login`）或 Grok CLI；都沒有可切「示範 DM」試玩
- Windows 用 git-bash 跑腳本

完整安裝盤點見 [`REQUIREMENTS.md`](REQUIREMENTS.md)。

## 啟動

```bash
codex login

# 開發模式：後端 :4318 + Vite 熱更新 :4317，開 http://127.0.0.1:4317
./scripts/dev.sh

# 正式模式：build 前後端，單一 binary 服務 http://127.0.0.1:4318
./scripts/run.sh          # 自動依 OS 選 run-mac.sh / run-windows.sh
PORT=8080 ./scripts/run.sh   # 換埠
./scripts/restart.sh         # 停舊行程並重建重啟
```

所有腳本說明見 [`scripts/README.md`](scripts/README.md)。

## 怎麼玩

1. 首次進入走開團設定：選故事模板（內建模板可選**劇本模式**＝預寫分支、選項按鈕、零 AI 延遲；或**自由模式**＝AI DM 即興(會等比較久) 目前有機率幻覺）、建 1–4 名角色
2. 右上「連線 Codex／Grok」讓 DM 上線 (包含生圖)
3. 每位玩家輸入行動並鎖定，**全員鎖定後**故事推進；DM 要求檢定時按骰盤擲 d20（加值與 DC 由伺服器算）
4. 戰鬥自動排先攻；輪到你時攻擊／施法／用資源，敵方回合 AI 自動結算
5. 進度存本機 SQLite（`campaign-data/`），重開瀏覽器續玩；設定頁可多戰役、匯入遊戲紀錄、匯出小說 `.txt`

## 選用功能

| 功能 | 啟用方式 |
|---|---|
| 場景／角色圖 | 需 `codex login`（固定 GPT／Codex `$imagegen`） |
| Grok 當 DM | `grok login` 或 `XAI_API_KEY`，設定頁切換（僅故事，不含生圖） |
| 介面／DM 語言 | 設定頁切換繁體中文／English |

後端設定全部在 `backend/.env`（範本：`backend/.env.example`，含各選項註解）。

## 測試

```bash
./scripts/test.sh   # 後端 vet+test、前端 typecheck+vitest、擴充 typecheck
```

## 選用：VS Code 擴充套件

瀏覽器版不需要安裝 VS Code。`vscode-extension/` 是獨立的精簡擴充套件，僅供 VS Code 使用者選用；它使用 VS Code Language Model API，需先在 VS Code Chat 設定可用模型。

```bash
cd vscode-extension
npm ci
npm run dev:extension
```

`dev:extension` 會先編譯，再啟動新的 VS Code Extension Development Host。側欄點 d20 圖示，執行「D&D Duet: 建立新戰役」。

## 授權

程式 MIT（見 `LICENSE`）。規則資料基於 SRD 5.2.1（CC-BY-4.0），歸屬聲明見 `NOTICE.md`。
