import { describe, expect, it } from 'vitest';
import { createLevel3Character } from './characters';
import { applyDamage, applyDmEffects, resolveShortRest, resolveSpellEffect } from './effects';

describe('automatic effect settlement', () => {
  it('uses temporary hp before regular hp', () => {
    const result = applyDamage({ id: 'x', name: 'x', side: 'enemy', initiativeBonus: 0, initiative: 0, ac: 10, hp: 10, maxHp: 10, temporaryHp: 5, attackBonus: 0, damage: '1d4', damageType: '鈍擊' }, 8);
    expect(result).toMatchObject({ temporaryHp: 0, hp: 7 });
  });

  it('automatically spends hit dice on a short rest until full', () => {
    const player = { ...createLevel3Character('player1', '甲', '戰士'), hp: 1 };
    const result = resolveShortRest(player, () => 0.99);
    expect(result.character.hp).toBe(player.maxHp);
    expect(result.diceSpent).toBeGreaterThan(0);
  });

  it('settles healing spells and bounds DM damage', () => {
    const caster = createLevel3Character('player1', '牧者', '牧師');
    const wounded = { ...createLevel3Character('player2', '傷者', '戰士'), hp: 1 };
    const healed = resolveSpellEffect([caster, wounded], undefined, caster.id, wounded.id, { kind: 'healing', target: 'ally', dice: '2d4', flat: 2 }, () => 0);
    expect(healed.players[1].hp).toBe(5);
    const dm = applyDmEffects(healed.players, [{ targetId: 'player2', kind: 'damage', amount: 9999, reason: '落石' }]);
    expect(dm.players[1].hp).toBe(0);
    expect(dm.logs[0]).toMatch(/落石/);
  });

  it('keeps combat hp synchronized after healing and consumes temporary hp for DM damage', () => {
    const caster = createLevel3Character('player1', '牧者', '牧師');
    const wounded = { ...createLevel3Character('player2', '傷者', '戰士'), hp: 1, temporaryHp: 3 };
    const combat = {
      active: true, round: 1, turnIndex: 0,
      combatants: [
        { id: caster.id, playerId: caster.id, name: caster.name, side: 'party' as const, initiativeBonus: 0, initiative: 20, ac: caster.ac, hp: caster.hp, maxHp: caster.maxHp, attackBonus: 0, damage: '1d4', damageType: '鈍擊' },
        { id: wounded.id, playerId: wounded.id, name: wounded.name, side: 'party' as const, initiativeBonus: 0, initiative: 10, ac: wounded.ac, hp: wounded.hp, maxHp: wounded.maxHp, temporaryHp: 3, attackBonus: 0, damage: '1d4', damageType: '鈍擊' },
      ],
    };
    const healed = resolveSpellEffect([caster, wounded], combat, caster.id, wounded.id, { kind: 'healing', target: 'ally', flat: 4 }, () => 0);
    expect(healed.combat?.combatants[1].hp).toBe(5);
    const dm = applyDmEffects(healed.players, [{ targetId: 'player2', kind: 'damage', amount: 5, reason: '落石' }]);
    expect(dm.players[1]).toMatchObject({ temporaryHp: 0, hp: 3 });
  });

  it('uses the visible spell attack total instead of rolling a hidden d20', () => {
    const caster = createLevel3Character('player1', '術者', '術士');
    const enemy = { id: 'enemy', name: '敵人', side: 'enemy' as const, initiativeBonus: 0, initiative: 10, ac: 15, hp: 10, maxHp: 10, attackBonus: 0, damage: '1d4', damageType: '鈍擊' };
    const combat = { active: true, round: 1, turnIndex: 0, combatants: [{ ...enemy }] };
    const missed = resolveSpellEffect([caster], combat, caster.id, enemy.id, { kind: 'damage', target: 'creature', flat: 4, attackRoll: true }, () => .99, { attackTotal: 14 });
    expect(missed.amount).toBe(0);
    expect(missed.combat?.combatants[0].hp).toBe(10);
    const hit = resolveSpellEffect([caster], combat, caster.id, enemy.id, { kind: 'damage', target: 'creature', flat: 4, attackRoll: true }, () => 0, { attackTotal: 15 });
    expect(hit.combat?.combatants[0].hp).toBe(6);
  });
});
