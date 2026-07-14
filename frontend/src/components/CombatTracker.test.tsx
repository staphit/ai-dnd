import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { createLevel3Character } from '../rules/characters';
import { partyCombatants } from '../rules/combat';
import type { CombatState } from '../types';
import { CombatTracker } from './CombatTracker';

describe('CombatTracker conclusion flow', () => {
  it('hands the final combat state to the DM continuation instead of ending silently', () => {
    const player = createLevel3Character('player1', '黎恩', '戰士');
    const combat: CombatState = {
      active: true,
      round: 2,
      turnIndex: 0,
      combatants: [
        ...partyCombatants([player]),
        { id: 'enemy-1', name: '灰牙', side: 'enemy', initiativeBonus: 2, initiative: 8, ac: 13, hp: 0, maxHp: 9, attackBonus: 3, damage: '1d6+1', damageType: '穿刺', defeated: true },
      ],
    };
    const onEnd = vi.fn();
    render(<CombatTracker players={[player]} combat={combat} onChange={vi.fn()} onEnd={onEnd} onLog={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /結束戰鬥並敘述/ }));
    expect(onEnd).toHaveBeenCalledWith(combat);
  });
});
