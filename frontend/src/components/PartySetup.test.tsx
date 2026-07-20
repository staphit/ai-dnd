import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PartySetup } from './PartySetup';
import { jsonResponse, makeCampaign, makePlayer } from '../test/fixtures';

const catalog = {
  classNames: ['吟遊詩人', '牧師', '戰士', '聖武士', '盜賊', '法師'],
  abilityLabels: { str: '力量', dex: '敏捷', con: '體質', int: '智力', wis: '感知', cha: '魅力' },
  spells: [],
  scriptedStoryIds: ['ashen-crown'],
};

type FetchArgs = [input: RequestInfo | URL, init?: RequestInit];

function stubFetch() {
  const calls: Array<{ url: string; method: string; body?: unknown }> = [];
  const mock = vi.fn(async (...args: FetchArgs) => {
    const url = String(args[0]);
    const init = args[1] || {};
    const method = (init.method || 'GET').toUpperCase();
    const body = typeof init.body === 'string' ? JSON.parse(init.body) : undefined;
    calls.push({ url, method, body });
    if (url === '/api/rules/catalog') return jsonResponse(catalog);
    if (url === '/api/campaigns' && method === 'POST') {
      const players = (body as { players: Array<{ name: string; className: string }> }).players;
      return jsonResponse(makeCampaign({
        id: 'campaign-created',
        title: (body as { title: string }).title,
        players: players.map((seed, index) => makePlayer(`player${index + 1}`, seed.name, { className: seed.className })),
      }));
    }
    return jsonResponse({ error: `unexpected ${method} ${url}` }, 500);
  });
  vi.stubGlobal('fetch', mock);
  return calls;
}

describe('PartySetup', () => {
  beforeEach(() => vi.unstubAllGlobals());

  it('creates a three-player party on the server with the selected names and classes', async () => {
    const calls = stubFetch();
    const user = userEvent.setup();
    const onComplete = vi.fn();
    render(<PartySetup onComplete={onComplete} />);

    // Wait for the catalog-backed class options.
    await waitFor(() => expect(screen.getAllByRole('option', { name: '盜賊' }).length).toBeGreaterThan(0));

    await user.click(screen.getByRole('button', { name: '3人' }));
    await user.clear(screen.getByLabelText('玩家 1 角色名稱'));
    await user.type(screen.getByLabelText('玩家 1 角色名稱'), '阿莎');
    await user.selectOptions(screen.getByLabelText('玩家 1 職業'), '法師');
    await user.clear(screen.getByLabelText('玩家 2 角色名稱'));
    await user.type(screen.getByLabelText('玩家 2 角色名稱'), '布蘭');
    await user.selectOptions(screen.getByLabelText('玩家 2 職業'), '盜賊');
    await user.clear(screen.getByLabelText('玩家 3 角色名稱'));
    await user.type(screen.getByLabelText('玩家 3 角色名稱'), '希雅');
    await user.selectOptions(screen.getByLabelText('玩家 3 職業'), '牧師');
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));

    // ashen-crown ships a script module: the story-mode dialog gates creation.
    const dialog = await screen.findByRole('dialog', { name: '要怎麼進行這個故事？' });
    expect(calls.some((entry) => entry.method === 'POST' && entry.url === '/api/campaigns')).toBe(false);
    await user.click(within(dialog).getByRole('button', { name: /既定劇本/ }));

    await waitFor(() => expect(onComplete).toHaveBeenCalledOnce());
    const create = calls.find((entry) => entry.method === 'POST' && entry.url === '/api/campaigns');
    expect(create).toBeDefined();
    const body = create!.body as { storyId: string; opening: string; storyMode: string; players: Array<{ name: string; className: string; level: number }> };
    expect(body.storyId).toBe('ashen-crown');
    expect(body.storyMode).toBe('scripted');
    expect(body.opening).toContain('禮拜堂');
    expect(body.players.map((seed) => [seed.name, seed.className, seed.level])).toEqual([
      ['阿莎', '法師', 3],
      ['布蘭', '盜賊', 3],
      ['希雅', '牧師', 3],
    ]);
    // The parent receives the server view verbatim.
    expect(onComplete.mock.calls[0][0].id).toBe('campaign-created');
  });

  it('uses the selected story preset and skips the story-mode dialog for non-scripted presets', async () => {
    const calls = stubFetch();
    const user = userEvent.setup();
    const onComplete = vi.fn();
    render(<PartySetup onComplete={onComplete} />);

    // Let the catalog land so scriptedStoryIds is known before submitting.
    await waitFor(() => expect(screen.getAllByRole('option', { name: '盜賊' }).length).toBeGreaterThan(0));
    await user.click(screen.getByRole('button', { name: /血月特快車/ }));
    expect(screen.getByDisplayValue('血月特快車')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));

    await waitFor(() => expect(onComplete).toHaveBeenCalledOnce());
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    const create = calls.find((entry) => entry.method === 'POST' && entry.url === '/api/campaigns');
    // Non-scripted presets omit storyMode: the server decides (and only the
    // modal ever sends the explicit 'freeform' opt-out).
    expect(create!.body).toMatchObject({ storyId: 'blood-moon-express', title: '血月特快車' });
    expect((create!.body as Record<string, unknown>).storyMode).toBeUndefined();
  });

  it('sends storyMode freeform when AI 自由走向 is chosen for a scripted preset', async () => {
    const calls = stubFetch();
    const user = userEvent.setup();
    const onComplete = vi.fn();
    render(<PartySetup onComplete={onComplete} />);

    await waitFor(() => expect(screen.getAllByRole('option', { name: '盜賊' }).length).toBeGreaterThan(0));
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));

    const dialog = await screen.findByRole('dialog', { name: '要怎麼進行這個故事？' });
    await user.click(within(dialog).getByRole('button', { name: /AI 自由走向/ }));

    await waitFor(() => expect(onComplete).toHaveBeenCalledOnce());
    const create = calls.find((entry) => entry.method === 'POST' && entry.url === '/api/campaigns');
    expect(create!.body).toMatchObject({ storyId: 'ashen-crown', storyMode: 'freeform' });
  });

  it('creates a custom freeform campaign from the user-written premise', async () => {
    const calls = stubFetch();
    const user = userEvent.setup();
    const onComplete = vi.fn();
    render(<PartySetup onComplete={onComplete} />);

    await user.click(screen.getByRole('button', { name: /自訂劇本/ }));
    expect(screen.getByRole('region', { name: '自訂劇本內容' })).toBeInTheDocument();
    await user.type(screen.getByLabelText('自訂劇本構想'), '港城每逢滿月就有人失去影子，隊伍必須調查封鎖的燈塔。');
    await user.type(screen.getByLabelText('自訂劇本類型'), '哥德懸疑');
    await user.type(screen.getByLabelText('自訂劇本起始場景'), '封鎖的舊燈塔');
    await user.type(screen.getByLabelText('自訂劇本目標'), '在月升前找出偷走影子的儀式');
    await user.type(screen.getByLabelText('自訂劇本風險'), '下一次月升時，全城居民都會失去影子。');
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));

    await waitFor(() => expect(onComplete).toHaveBeenCalledOnce());
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    const create = calls.find((entry) => entry.method === 'POST' && entry.url === '/api/campaigns');
    expect(create!.body).toMatchObject({
      storyId: 'custom',
      storyMode: 'freeform',
      title: '自訂冒險',
      scene: '封鎖的舊燈塔',
      objective: '在月升前找出偷走影子的儀式',
      stakes: '下一次月升時，全城居民都會失去影子。',
    });
    expect((create!.body as { objectiveContext: string }).objectiveContext).toContain('港城每逢滿月');
    expect((create!.body as { opening: string }).opening).toContain('港城每逢滿月');
  });

  it('requires an adventure premise before creating a custom campaign', async () => {
    const calls = stubFetch();
    const user = userEvent.setup();
    render(<PartySetup onComplete={vi.fn()} />);

    await user.click(screen.getByRole('button', { name: /自訂劇本/ }));
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));

    expect(screen.getByRole('alert')).toHaveTextContent('請先寫下自訂劇本的冒險構想');
    expect(calls.some((entry) => entry.method === 'POST' && entry.url === '/api/campaigns')).toBe(false);
  });

  it('rejects duplicate character names without calling the server', async () => {
    const calls = stubFetch();
    const user = userEvent.setup();
    const onComplete = vi.fn();
    render(<PartySetup onComplete={onComplete} />);

    await user.clear(screen.getByLabelText('玩家 2 角色名稱'));
    await user.type(screen.getByLabelText('玩家 2 角色名稱'), '冒險者一號');
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));

    expect(screen.getByRole('alert')).toHaveTextContent('角色名稱不能重複');
    expect(onComplete).not.toHaveBeenCalled();
    expect(calls.some((entry) => entry.method === 'POST' && entry.url === '/api/campaigns')).toBe(false);
  });

});
