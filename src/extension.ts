import * as vscode from 'vscode';
import { askDungeonMaster } from './dm';
import { CampaignStore } from './storage';
import { CampaignState, ChatEntry } from './types';

export function activate(context: vscode.ExtensionContext): void {
  const root = vscode.workspace.workspaceFolders?.[0]?.uri ?? context.globalStorageUri;
  const store = new CampaignStore(root);
  const provider = new GameViewProvider(context.extensionUri, store);

  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider('dndDuet.gameView', provider),
    vscode.commands.registerCommand('dndDuet.newCampaign', () => provider.newCampaign()),
    vscode.commands.registerCommand('dndDuet.rollDice', async () => {
      const sidesText = await vscode.window.showInputBox({ prompt: '骰子面數', value: '20' });
      if (!sidesText) { return; }
      const sides = Number.parseInt(sidesText, 10);
      if (!Number.isInteger(sides) || sides < 2 || sides > 1000) {
        void vscode.window.showErrorMessage('骰子面數必須是 2 到 1000 的整數。');
        return;
      }
      void vscode.window.showInformationMessage(`d${sides} 擲骰結果：${roll(sides)}`);
    })
  );
}

class GameViewProvider implements vscode.WebviewViewProvider {
  private view?: vscode.WebviewView;
  private state?: CampaignState;
  private busy = false;

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly store: CampaignStore
  ) {}

  async resolveWebviewView(view: vscode.WebviewView): Promise<void> {
    this.view = view;
    view.webview.options = { enableScripts: true, localResourceRoots: [this.extensionUri] };
    view.webview.html = getHtml(view.webview);
    this.state = await this.store.load();

    view.webview.onDidReceiveMessage(async message => {
      if (message.type === 'ready') { await this.publish(); }
      if (message.type === 'submit') { await this.submit(message.player, message.text); }
      if (message.type === 'roll') { await this.addRoll(message.player, message.sides); }
      if (message.type === 'newCampaign') { await this.newCampaign(); }
    });
    await this.publish();
  }

  async newCampaign(): Promise<void> {
    const title = await vscode.window.showInputBox({
      prompt: '新戰役名稱',
      value: '雙月酒館之謎',
      validateInput: value => value.trim() ? undefined : '請輸入戰役名稱'
    });
    if (!title) { return; }
    this.state = await this.store.reset(title.trim());
    await this.publish();
  }

  private async submit(player: 'player1' | 'player2', rawText: unknown): Promise<void> {
    if (
      this.busy ||
      !this.state ||
      (player !== 'player1' && player !== 'player2') ||
      typeof rawText !== 'string'
    ) { return; }
    const text = rawText.trim().slice(0, 2000);
    if (!text || this.state.pendingActions[player]) { return; }

    this.state.pendingActions[player] = text;
    this.state.history.push(entry(player, text));
    await this.store.save(this.state);
    await this.publish();

    const { player1, player2 } = this.state.pendingActions;
    if (!player1 || !player2) { return; }

    this.busy = true;
    await this.publish();
    try {
      const answer = await vscode.window.withProgress(
        { location: { viewId: 'dndDuet.gameView' }, title: 'AI DM 正在思考…' },
        (_progress, token) => askDungeonMaster(this.state!, player1, player2, token)
      );
      this.state.history.push(entry('dm', answer));
      this.state.round += 1;
      this.state.pendingActions = {};
      await this.store.appendLog(`## 回合 ${this.state.round - 1}\n\n**玩家一：** ${player1}\n\n**玩家二：** ${player2}\n\n**DM：** ${answer}`);
      await this.store.save(this.state);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.state.history.push(entry('system', `AI DM 錯誤：${message}`));
      this.state.pendingActions = {};
      await this.store.save(this.state);
      void vscode.window.showErrorMessage(message);
    } finally {
      this.busy = false;
      await this.publish();
    }
  }

  private async addRoll(player: 'player1' | 'player2', value: unknown): Promise<void> {
    if (!this.state || (player !== 'player1' && player !== 'player2')) { return; }
    const sides = Number(value);
    if (!Number.isInteger(sides) || sides < 2 || sides > 1000) { return; }
    const result = roll(sides);
    this.state.history.push(entry(player, `擲 d${sides}：${result}`));
    await this.store.save(this.state);
    await this.publish();
  }

  private async publish(): Promise<void> {
    if (!this.view || !this.state) { return; }
    await this.view.webview.postMessage({ type: 'state', state: this.state, busy: this.busy });
  }
}

function entry(speaker: ChatEntry['speaker'], text: string): ChatEntry {
  return { id: `${Date.now()}-${Math.random()}`, speaker, text, createdAt: new Date().toISOString() };
}

function roll(sides: number): number {
  return Math.floor(Math.random() * sides) + 1;
}

function getHtml(webview: vscode.Webview): string {
  const nonce = `${Date.now()}${Math.random()}`;
  return `<!DOCTYPE html>
<html lang="zh-Hant">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${webview.cspSource} 'nonce-${nonce}'; script-src 'nonce-${nonce}';">
  <style nonce="${nonce}">
    * { box-sizing: border-box; }
    body { padding: 0 10px 16px; color: var(--vscode-foreground); font-family: var(--vscode-font-family); }
    header { position: sticky; top: 0; padding: 10px 0 8px; background: var(--vscode-sideBar-background); z-index: 2; }
    h2 { margin: 0 0 3px; font-size: 15px; }
    .meta { opacity: .7; font-size: 11px; }
    #history { display: flex; flex-direction: column; gap: 8px; margin: 6px 0 14px; }
    .message { padding: 8px; border-radius: 4px; white-space: pre-wrap; line-height: 1.45; border-left: 3px solid var(--vscode-panel-border); background: var(--vscode-editor-background); }
    .dm { border-left-color: var(--vscode-charts-purple); }
    .player1 { border-left-color: var(--vscode-charts-blue); }
    .player2 { border-left-color: var(--vscode-charts-green); }
    .system { opacity: .75; font-style: italic; }
    .speaker { display: block; margin-bottom: 4px; font-size: 11px; font-weight: 700; opacity: .75; }
    .players { display: grid; gap: 9px; }
    .player { padding: 9px; border: 1px solid var(--vscode-panel-border); border-radius: 5px; }
    label { display: block; font-weight: 600; margin-bottom: 5px; }
    textarea { width: 100%; min-height: 64px; resize: vertical; color: var(--vscode-input-foreground); background: var(--vscode-input-background); border: 1px solid var(--vscode-input-border); padding: 6px; }
    .actions { display: flex; gap: 5px; margin-top: 6px; }
    button { border: 0; padding: 6px 9px; color: var(--vscode-button-foreground); background: var(--vscode-button-background); cursor: pointer; }
    button:hover { background: var(--vscode-button-hoverBackground); }
    button.secondary { color: var(--vscode-button-secondaryForeground); background: var(--vscode-button-secondaryBackground); }
    button:disabled, textarea:disabled { opacity: .5; cursor: default; }
    .waiting { font-size: 11px; color: var(--vscode-testing-iconPassed); margin-top: 5px; }
  </style>
</head>
<body>
  <header><h2 id="title">D&D Duet AI</h2><div class="meta" id="meta"></div></header>
  <main id="history" aria-live="polite"></main>
  <section class="players">
    ${playerForm('player1', '玩家一')}
    ${playerForm('player2', '玩家二')}
  </section>
  <script nonce="${nonce}">
    const vscode = acquireVsCodeApi();
    let current = null;
    const names = { dm: 'AI DM', player1: '玩家一', player2: '玩家二', system: '系統' };
    const history = document.getElementById('history');

    window.addEventListener('message', event => {
      if (event.data.type !== 'state') return;
      current = event.data;
      render();
    });

    function render() {
      const state = current.state;
      document.getElementById('title').textContent = state.title;
      document.getElementById('meta').textContent = state.scene + ' · 回合 ' + state.round + (current.busy ? ' · AI DM 思考中…' : '');
      history.replaceChildren(...state.history.map(item => {
        const node = document.createElement('article');
        node.className = 'message ' + item.speaker;
        const speaker = document.createElement('span');
        speaker.className = 'speaker'; speaker.textContent = names[item.speaker];
        const text = document.createElement('span'); text.textContent = item.text;
        node.append(speaker, text); return node;
      }));
      for (const player of ['player1', 'player2']) {
        const submitted = Boolean(state.pendingActions[player]);
        document.getElementById(player + '-text').disabled = submitted || current.busy;
        document.getElementById(player + '-submit').disabled = submitted || current.busy;
        document.getElementById(player + '-status').textContent = submitted ? '已提交，等待另一位玩家' : '';
        if (!submitted) document.getElementById(player + '-text').value = '';
      }
      history.lastElementChild?.scrollIntoView({ behavior: 'smooth' });
    }

    document.addEventListener('click', event => {
      const button = event.target.closest('button');
      if (!button) return;
      const player = button.dataset.player;
      if (button.dataset.action === 'submit') {
        const input = document.getElementById(player + '-text');
        if (input.value.trim()) vscode.postMessage({ type: 'submit', player, text: input.value });
      }
      if (button.dataset.action === 'roll') vscode.postMessage({ type: 'roll', player, sides: 20 });
    });
    vscode.postMessage({ type: 'ready' });
  </script>
</body>
</html>`;
}

function playerForm(id: string, label: string): string {
  return `<div class="player">
    <label for="${id}-text">${label}的行動</label>
    <textarea id="${id}-text" maxlength="2000" placeholder="描述你要做什麼…"></textarea>
    <div class="actions">
      <button id="${id}-submit" data-action="submit" data-player="${id}">提交行動</button>
      <button class="secondary" data-action="roll" data-player="${id}">擲 d20</button>
    </div>
    <div class="waiting" id="${id}-status"></div>
  </div>`;
}

export function deactivate(): void {}
