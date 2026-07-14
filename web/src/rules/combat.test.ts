import { describe, expect, it } from 'vitest';
import { advanceTurn, resolveAttack, rollExpression, startCombat } from './combat';
import type { Combatant } from '../types';

const fighter: Combatant = { id: 'fighter', name: '戰士', side: 'party', initiativeBonus: 3, initiative: 0, ac: 18, hp: 25, maxHp: 25, attackBonus: 6, damage: '1d8+3', damageType: '揮砍' };
const goblin: Combatant = { id: 'goblin', name: '哥布林', side: 'enemy', initiativeBonus: 2, initiative: 0, ac: 13, hp: 12, maxHp: 12, attackBonus: 4, damage: '1d6+2', damageType: '穿刺' };

describe('combat rules', () => {
  it('rolls initiative and wraps rounds in sequence', () => {
    const rolls = [0.9, 0.1];
    const state = startCombat([fighter, goblin], () => rolls.shift() || 0);
    expect(state.combatants[0].name).toBe('戰士');
    expect(advanceTurn(advanceTurn(state)).round).toBe(2);
  });

  it('automatically resolves hit and damage against AC', () => {
    const state = { active: true, round: 1, turnIndex: 0, combatants: [fighter, goblin] };
    const rolls = [0.7, 0.5]; // d20=15, d8=5
    const result = resolveAttack(state, 'fighter', 'goblin', () => rolls.shift() || 0);
    expect(result.resolution.hit).toBe(true);
    expect(result.resolution.damage).toBe(8);
    expect(result.state.combatants[1].hp).toBe(4);
  });

  it('doubles only damage dice on a critical hit', () => {
    const rolls = [0.999, 0, 0.2]; // natural 20; d8 1+2; +3
    const state = { active: true, round: 1, turnIndex: 0, combatants: [fighter, goblin] };
    const result = resolveAttack(state, 'fighter', 'goblin', () => rolls.shift() || 0);
    expect(result.resolution.critical).toBe(true);
    expect(result.resolution.damage).toBe(6);
    expect(rollExpression('2d6+2', () => 0)).toBe(4);
  });
});
