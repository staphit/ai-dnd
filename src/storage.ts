import * as vscode from 'vscode';
import { randomUUID } from 'node:crypto';
import { CampaignState } from './types';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isCampaignState(value: unknown): value is CampaignState {
  if (!isRecord(value)
    || typeof value.id !== 'string'
    || typeof value.title !== 'string'
    || typeof value.createdAt !== 'string'
    || typeof value.updatedAt !== 'string'
    || typeof value.scene !== 'string'
    || !Number.isInteger(value.round)
    || (value.round as number) < 1
    || !isRecord(value.pendingActions)
    || !Array.isArray(value.history)) {
    return false;
  }
  const pending = value.pendingActions;
  if ((pending.player1 !== undefined && typeof pending.player1 !== 'string')
    || (pending.player2 !== undefined && typeof pending.player2 !== 'string')) {
    return false;
  }
  const speakers = new Set(['dm', 'player1', 'player2', 'system']);
  return value.history.every((entry) => isRecord(entry)
    && typeof entry.id === 'string'
    && typeof entry.speaker === 'string'
    && speakers.has(entry.speaker)
    && typeof entry.text === 'string'
    && typeof entry.createdAt === 'string');
}

export class CampaignStore {
  private readonly stateFile: vscode.Uri;
  private readonly logFile: vscode.Uri;
  private logWrites: Promise<void> = Promise.resolve();

  constructor(private readonly root: vscode.Uri) {
    this.stateFile = vscode.Uri.joinPath(root, 'campaign-data', 'campaign.json');
    this.logFile = vscode.Uri.joinPath(root, 'campaign-data', 'session-log.md');
  }

  async load(): Promise<CampaignState> {
    try {
      const bytes = await vscode.workspace.fs.readFile(this.stateFile);
      const parsed: unknown = JSON.parse(Buffer.from(bytes).toString('utf8'));
      return isCampaignState(parsed) ? parsed : this.freshState();
    } catch {
      return this.freshState();
    }
  }

  async reset(title = '未命名冒險'): Promise<CampaignState> {
    const state = this.freshState(title);
    await this.save(state);
    await this.enqueueLogWrite(() => this.writeAtomic(
      this.logFile,
      Buffer.from(`# ${title}\n\n建立於 ${state.createdAt}\n`, 'utf8')
    ));
    return state;
  }

  async save(state: CampaignState): Promise<void> {
    const saved = { ...state, updatedAt: new Date().toISOString() };
    if (!isCampaignState(saved)) {
      throw new Error('Refusing to save an invalid campaign state.');
    }
    state.updatedAt = saved.updatedAt;
    await this.writeAtomic(this.stateFile, Buffer.from(JSON.stringify(saved, null, 2), 'utf8'));
  }

  async appendLog(markdown: string): Promise<void> {
    await this.enqueueLogWrite(async () => {
      let previous = '';
      try {
        previous = Buffer.from(await vscode.workspace.fs.readFile(this.logFile)).toString('utf8');
      } catch {
        previous = '# D&D Duet 遊戲紀錄\n';
      }
      await this.writeAtomic(this.logFile, Buffer.from(`${previous}\n${markdown}\n`, 'utf8'));
    });
  }

  private enqueueLogWrite(operation: () => Promise<void>): Promise<void> {
    const result = this.logWrites.then(operation, operation);
    this.logWrites = result.then(() => undefined, () => undefined);
    return result;
  }

  private async writeAtomic(target: vscode.Uri, data: Uint8Array): Promise<void> {
    const dir = vscode.Uri.joinPath(this.root, 'campaign-data');
    await vscode.workspace.fs.createDirectory(dir);
    const name = target.path.split('/').pop() || 'campaign-data';
    const temporary = vscode.Uri.joinPath(dir, `.${name}.${randomUUID()}.tmp`);
    try {
      await vscode.workspace.fs.writeFile(temporary, data);
      await vscode.workspace.fs.rename(temporary, target, { overwrite: true });
    } catch (error) {
      try {
        await vscode.workspace.fs.delete(temporary, { useTrash: false });
      } catch {
        // The rename may already have consumed the temporary file.
      }
      throw error;
    }
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
