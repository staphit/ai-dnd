import * as vscode from 'vscode';
import { CampaignState } from './types';

export class CampaignStore {
  private readonly stateFile: vscode.Uri;
  private readonly logFile: vscode.Uri;

  constructor(private readonly root: vscode.Uri) {
    this.stateFile = vscode.Uri.joinPath(root, 'campaign-data', 'campaign.json');
    this.logFile = vscode.Uri.joinPath(root, 'campaign-data', 'session-log.md');
  }

  async load(): Promise<CampaignState> {
    try {
      const bytes = await vscode.workspace.fs.readFile(this.stateFile);
      return JSON.parse(Buffer.from(bytes).toString('utf8')) as CampaignState;
    } catch {
      return this.freshState();
    }
  }

  async reset(title = '未命名冒險'): Promise<CampaignState> {
    const state = this.freshState(title);
    await this.save(state);
    await vscode.workspace.fs.writeFile(
      this.logFile,
      Buffer.from(`# ${title}\n\n建立於 ${state.createdAt}\n`, 'utf8')
    );
    return state;
  }

  async save(state: CampaignState): Promise<void> {
    const dir = vscode.Uri.joinPath(this.root, 'campaign-data');
    await vscode.workspace.fs.createDirectory(dir);
    state.updatedAt = new Date().toISOString();
    await vscode.workspace.fs.writeFile(
      this.stateFile,
      Buffer.from(JSON.stringify(state, null, 2), 'utf8')
    );
  }

  async appendLog(markdown: string): Promise<void> {
    let previous = '';
    try {
      previous = Buffer.from(await vscode.workspace.fs.readFile(this.logFile)).toString('utf8');
    } catch {
      previous = '# D&D Duet 遊戲紀錄\n';
    }
    await vscode.workspace.fs.writeFile(
      this.logFile,
      Buffer.from(`${previous}\n${markdown}\n`, 'utf8')
    );
  }

  private freshState(title = '未命名冒險'): CampaignState {
    const now = new Date().toISOString();
    return {
      id: `${Date.now()}`,
      title,
      createdAt: now,
      updatedAt: now,
      scene: '序章',
      round: 1,
      pendingActions: {},
      history: [{
        id: `${Date.now()}-welcome`,
        speaker: 'system',
        text: '請讓兩位玩家分別輸入行動；兩人都提交後，AI DM 才會推進故事。',
        createdAt: now
      }]
    };
  }
}
