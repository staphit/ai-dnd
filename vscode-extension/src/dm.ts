import * as vscode from 'vscode';
import { CampaignState } from './types';

export async function askDungeonMaster(
  state: CampaignState,
  player1: string,
  player2: string,
  token: vscode.CancellationToken
): Promise<string> {
  const models = await vscode.lm.selectChatModels();
  if (models.length === 0) {
    throw new Error('找不到可用的 VS Code 語言模型。請先在 VS Code Chat 中設定模型或登入 GitHub Copilot。');
  }

  const config = vscode.workspace.getConfiguration('dndDuet');
  const language = config.get<string>('language', '繁體中文');
  const ruleset = config.get<string>('ruleset', 'D&D 5e');
  const recent = state.history.slice(-12).map(item => `${item.speaker}: ${item.text}`).join('\n');

  const system = [
    `你是兩名本地玩家的 Dungeon Master，使用 ${language} 與 ${ruleset}。`,
    '維持玩家能動性，不替玩家決定行動。',
    '公平描述後果；需要檢定時說明能力、技能與 DC，但不要捏造骰子結果。',
    '一次推進一個有意義的場景，結尾清楚詢問兩位玩家接下來做什麼。',
    '輸出適合遊戲桌朗讀的文字，最多 500 字。'
  ].join('\n');
  const prompt = [
    `戰役：${state.title}`,
    `場景：${state.scene}，回合：${state.round}`,
    '最近紀錄：',
    recent || '尚無紀錄',
    '',
    `玩家一：${player1}`,
    `玩家二：${player2}`,
    '',
    '請以 DM 身分處理兩個行動，指出衝突或檢定需求並推進故事。'
  ].join('\n');

  const messages = [
    vscode.LanguageModelChatMessage.User(system),
    vscode.LanguageModelChatMessage.User(prompt)
  ];
  const response = await models[0].sendRequest(messages, {}, token);
  let result = '';
  for await (const fragment of response.text) {
    result += fragment;
  }
  return result.trim();
}
