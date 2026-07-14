import { describe, expect, it } from 'vitest';
import { classNames, createLevel3Character, restCharacter, spendSpellSlot } from './characters';

describe('2024 level 3 class rules', () => {
  it('builds all 12 classes with a complete common character sheet', () => {
    expect(classNames).toHaveLength(12);
    for (const [index, className] of classNames.entries()) {
      const character = createLevel3Character(`player${index + 1}`, `測試${className}`, className);
      expect(character.className).toBe(className);
      expect(character.subclass).not.toBe('');
      expect(character.level).toBe(3);
      expect(character.proficiencyBonus).toBe(2);
      expect(Object.keys(character.abilities)).toHaveLength(6);
      expect(character.maxHp).toBeGreaterThan(0);
      expect(character.ac).toBeGreaterThanOrEqual(10);
      expect(character.skills).toHaveLength(18);
      expect(character.attacks.length).toBeGreaterThan(0);
      expect(character.features.length).toBeGreaterThan(0);
      expect(character.equipment.length).toBeGreaterThan(0);
    }
  });

  it.each(['吟遊詩人', '牧師', '德魯伊', '術士', '法師'])('%s has 4 first-level and 2 second-level slots', (className) => {
    const character = createLevel3Character('player1', '施法者', className);
    expect(character.spellcasting?.slots).toEqual([
      { level: 1, current: 4, max: 4 },
      { level: 2, current: 2, max: 2 },
    ]);
  });

  it('models half casters and Pact Magic separately', () => {
    for (const className of ['聖武士', '遊俠']) {
      const character = createLevel3Character('player1', '半施法者', className);
      expect(character.spellcasting?.slots).toEqual([{ level: 1, current: 3, max: 3 }]);
      expect(character.spellcasting?.mode).toBe('standard');
    }
    const warlock = createLevel3Character('player1', '契約者', '魔契師');
    expect(warlock.spellcasting?.mode).toBe('pact');
    expect(warlock.spellcasting?.pactSlotLevel).toBe(2);
    expect(warlock.spellcasting?.slots).toEqual([{ level: 2, current: 2, max: 2 }]);
  });

  it('gives the Evoker cantrips, a twelve-spell book, six prepared spells, and Arcane Recovery', () => {
    const wizard = createLevel3Character('player1', '梅林', '法師');
    const spells = wizard.spellcasting?.spells || [];
    expect(spells.filter((spell) => spell.level === 0)).toHaveLength(3);
    expect(spells.filter((spell) => spell.inSpellbook)).toHaveLength(12);
    expect(spells.filter((spell) => spell.level > 0 && spell.prepared)).toHaveLength(6);
    expect(wizard.resources.find((entry) => entry.id === 'arcane_recovery')?.current).toBe(1);
    expect(wizard.spellcasting?.ability).toBe('int');
  });

  it('uses the paladin free Divine Smite before spending a spell slot', () => {
    const paladin = createLevel3Character('player1', '聖騎士', '聖武士');
    const smite = paladin.spellcasting?.spells.find((spell) => spell.id === 'divine_smite');
    const afterFreeSmite = spendSpellSlot(paladin, smite!, false)!;
    expect(afterFreeSmite.resources.find((entry) => entry.id === 'free_divine_smite')?.current).toBe(0);
    expect(afterFreeSmite.spellcasting?.slots[0].current).toBe(3);
    const afterPaidSmite = spendSpellSlot(afterFreeSmite, smite!, false)!;
    expect(afterPaidSmite.spellcasting?.slots[0].current).toBe(2);
  });

  it('spends a slot, tracks concentration, and never allows negative slots', () => {
    const wizard = createLevel3Character('player1', '梅林', '法師');
    const sleep = wizard.spellcasting?.spells.find((spell) => spell.id === 'sleep');
    expect(sleep).toBeDefined();
    const afterCast = spendSpellSlot(wizard, sleep!, false);
    expect(afterCast?.spellcasting?.slots[0].current).toBe(3);
    expect(afterCast?.concentration).toBe('睡眠術');

    let exhausted = afterCast!;
    exhausted = spendSpellSlot(exhausted, sleep!, false)!;
    exhausted = spendSpellSlot(exhausted, sleep!, false)!;
    exhausted = spendSpellSlot(exhausted, sleep!, false)!;
    expect(exhausted.spellcasting?.slots[0].current).toBe(0);
    expect(spendSpellSlot(exhausted, sleep!, false)?.spellcasting?.slots[1].current).toBe(1);
  });

  it('recovers Pact Magic on a short rest and all resources on a long rest', () => {
    const warlock = createLevel3Character('player1', '契約者', '魔契師');
    const hex = warlock.spellcasting?.spells.find((spell) => spell.id === 'hex');
    const spent = spendSpellSlot(warlock, hex!, false)!;
    expect(spent.spellcasting?.slots[0].current).toBe(1);
    expect(restCharacter(spent, 'short').spellcasting?.slots[0].current).toBe(2);

    const wounded = { ...spent, hp: 1, condition: '中毒', hitDice: 0 };
    const rested = restCharacter(wounded, 'long');
    expect(rested.hp).toBe(rested.maxHp);
    expect(rested.hitDice).toBe(rested.maxHitDice);
    expect(rested.condition).toBe('正常');
  });
});
