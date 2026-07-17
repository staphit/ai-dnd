import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import type { Campaign } from './types';
import { jsonResponse, makeCampaign, makePlayer } from './test/fixtures';

interface Call {
  url: string;
  method: string;
  body?: unknown;
}

// Routes fetch by "METHOD url"; unmatched requests fail the test loudly.
function stubFetchRouter(routes: Record<string, (call: Call) => Response>) {
  const calls: Call[] = [];
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = (init?.method || 'GET').toUpperCase();
    const call: Call = { url, method, body: typeof init?.body === 'string' ? JSON.parse(init.body) : undefined };
    calls.push(call);
    const handler = routes[`${method} ${url}`];
    if (!handler) return jsonResponse({ error: `unexpected ${method} ${url}` }, 500);
    return handler(call);
  }));
  return calls;
}

const baseRoutes = {
  'GET /api/status': () => jsonResponse({ connected: true, provider: 'Codex CLI', model: 'gpt-5' }),
  'GET /api/codex/connection': () => jsonResponse({ alive: true, storyId: 'campaign-test-1' }),
};

beforeEach(() => localStorage.clear());
afterEach(() => vi.unstubAllGlobals());

describe('legacy vault import banner', () => {
  it('offers a one-time upload of local campaigns and loads the imported view', async () => {
    const legacy = makeCampaign({ id: 'campaign-legacy-1', title: '正在玩的舊戰役', round: 9 });
    localStorage.setItem('dnd-duet-web-v2-vault', JSON.stringify({ version: 2, activeId: legacy.id, campaigns: { [legacy.id!]: legacy } }));

    const imported = makeCampaign({ id: 'campaign-legacy-1', title: '正在玩的舊戰役', round: 9 });
    const calls = stubFetchRouter({
      ...baseRoutes,
      'GET /api/codex/connection': () => jsonResponse({ alive: false, storyId: '' }),
      'GET /api/campaigns': () => jsonResponse({ campaigns: [] }),
      'POST /api/campaigns/import': () => jsonResponse(imported),
    });

    render(<App />);

    const uploadButton = await screen.findByRole('button', { name: '將本機戰役上傳到伺服器' });
    fireEvent.click(uploadButton);

    // The imported campaign becomes the active table view.
    await waitFor(() => expect(screen.getByRole('heading', { name: '正在玩的舊戰役' })).toBeVisible());
    const importCall = calls.find((call) => call.method === 'POST' && call.url === '/api/campaigns/import');
    expect(importCall).toBeDefined();
    expect((importCall!.body as Campaign).title).toBe('正在玩的舊戰役');
    expect(localStorage.getItem('dnd-duet-active-id')).toBe('campaign-legacy-1');
  });
});

describe('dm turn rejection', () => {
  it('renders 422 action issues and releases the rejected server locks', async () => {
    localStorage.setItem('dnd-duet-active-id', 'campaign-test-1');
    const view = makeCampaign({ players: [makePlayer('player1', '艾拉')] });
    const lockedView: Campaign = { ...view, pending: { player1: '施放「火球術」' } };
    const unlockedView: Campaign = { ...view, pending: {} };

    const calls = stubFetchRouter({
      ...baseRoutes,
      'GET /api/campaign/campaign-test-1': () => jsonResponse(view),
      'GET /api/campaigns': () => jsonResponse({ campaigns: [{ id: 'campaign-test-1', title: view.title, scene: view.scene, round: 1, updatedAt: view.updatedAt }] }),
      'POST /api/campaign/campaign-test-1/players/player1/action': () => jsonResponse(lockedView),
      'DELETE /api/campaign/campaign-test-1/players/player1/action': () => jsonResponse(unlockedView),
      'POST /api/dm': () => jsonResponse({
        error: '有行動未通過規則驗證，故事尚未推進。',
        actionIssues: [{ playerId: 'player1', message: '施放「火球術」需要 3 環以上的法術位，目前已耗盡。' }],
      }, 422),
    });

    render(<App />);
    await screen.findByRole('heading', { name: view.title });

    // Locking the only player's action completes the round and triggers /api/dm.
    fireEvent.click(screen.getByRole('button', { name: /鎖定行動/ }));

    const banner = await screen.findByRole('alert');
    await waitFor(() => expect(banner).toHaveTextContent('【行動駁回】艾拉：施放「火球術」需要 3 環以上的法術位，目前已耗盡。'));
    expect(calls.some((call) => call.method === 'POST' && call.url === '/api/dm')).toBe(true);
    expect(calls.some((call) => call.method === 'DELETE' && call.url === '/api/campaign/campaign-test-1/players/player1/action')).toBe(true);
  });
});
