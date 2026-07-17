import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { CombatState } from '../types';
import { jsonResponse, makeCampaign, makeCombat, makeCombatant, makePlayer } from '../test/fixtures';
import { CombatTracker } from './CombatTracker';

const player = makePlayer('player1', '黎恩');

function partyFirstCombat(): CombatState {
  return makeCombat({
    round: 2,
    turnIndex: 0,
    combatants: [
      makeCombatant({ id: 'c-player1', name: '黎恩', side: 'party', playerId: 'player1', hp: 28, maxHp: 28, initiative: 15 }),
      makeCombatant({ id: 'enemy-1', name: '灰牙', side: 'enemy', hp: 9, maxHp: 9, initiative: 8 }),
    ],
  });
}

function enemyFirstCombat(): CombatState {
  const combat = partyFirstCombat();
  return { ...combat, turnIndex: 1 };
}

afterEach(() => vi.unstubAllGlobals());

describe('CombatTracker server flows', () => {
  it('shows the enemy-turn button on an enemy turn and renders the returned intent', async () => {
    const view = makeCampaign({ combat: partyFirstCombat() });
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/campaign/campaign-test-1/combat/enemy-turn') {
        return jsonResponse({
          view,
          resolution: { attackRoll: 12, total: 16, hit: true, critical: false, damage: 5, text: '灰牙攻擊黎恩：命中，造成 5 點傷害。' },
          intent: '灰牙撲向最虛弱的黎恩。',
          fallback: true,
          enemyTurnPending: false,
        });
      }
      return jsonResponse({ error: `unexpected ${url}` }, 500);
    });
    vi.stubGlobal('fetch', fetchMock);
    const onView = vi.fn();
    render(<CombatTracker campaignId="campaign-test-1" players={[player]} combat={enemyFirstCombat()} onView={onView} onEnd={vi.fn()} />);

    const button = screen.getByRole('button', { name: /敵方行動/ });
    fireEvent.click(button);

    await waitFor(() => expect(onView).toHaveBeenCalledWith(view));
    expect(screen.getByText(/【敵方】灰牙撲向最虛弱的黎恩。/)).toBeVisible();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('resolves a player attack through the server and hands back the view', async () => {
    const view = makeCampaign({ combat: partyFirstCombat() });
    let attackBody: unknown;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/campaign/campaign-test-1/combat/attack') {
        attackBody = JSON.parse(String(init?.body));
        return jsonResponse({ view, resolution: { attackRoll: 18, total: 23, hit: true, critical: false, damage: 7, text: '黎恩攻擊灰牙：命中。' } });
      }
      return jsonResponse({ error: `unexpected ${url}` }, 500);
    });
    vi.stubGlobal('fetch', fetchMock);
    const onView = vi.fn();
    render(<CombatTracker campaignId="campaign-test-1" players={[player]} combat={partyFirstCombat()} onView={onView} onEnd={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /攻擊（使用動作）/ }));
    await waitFor(() => expect(onView).toHaveBeenCalledWith(view));
    expect(attackBody).toEqual({ attackId: 'longsword', targetId: 'enemy-1' });
  });

  it('hands combat conclusion to the parent instead of ending silently', () => {
    vi.stubGlobal('fetch', vi.fn());
    const onEnd = vi.fn();
    render(<CombatTracker campaignId="campaign-test-1" players={[player]} combat={partyFirstCombat()} onView={vi.fn()} onEnd={onEnd} />);
    fireEvent.click(screen.getByRole('button', { name: /結束戰鬥並敘述/ }));
    expect(onEnd).toHaveBeenCalledOnce();
  });

  it('starts a built encounter through the server', async () => {
    const view = makeCampaign({ combat: partyFirstCombat() });
    let startBody: unknown;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/campaign/campaign-test-1/combat/start') {
        startBody = JSON.parse(String(init?.body));
        return jsonResponse(view);
      }
      return jsonResponse({ error: `unexpected ${url}` }, 500);
    });
    vi.stubGlobal('fetch', fetchMock);
    const onView = vi.fn();
    render(<CombatTracker campaignId="campaign-test-1" players={[player]} combat={undefined} onView={onView} onEnd={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /加入敵人/ }));
    fireEvent.click(screen.getByRole('button', { name: /擲先攻並開始/ }));
    await waitFor(() => expect(onView).toHaveBeenCalledWith(view));
    expect(startBody).toEqual({ enemies: [{ name: '骸骨守衛', ac: 13, hp: 13, initiativeBonus: 2, attackBonus: 4, damage: '1d6+2', damageType: '穿刺' }] });
  });
});
