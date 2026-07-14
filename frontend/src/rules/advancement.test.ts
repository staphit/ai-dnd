import { describe, expect, it } from 'vitest';
import { createConfiguredCharacter, experienceToNextLevel, grantExperience, levelUpCharacter, setPreparedSpells, spendAbilityPoint } from './advancement';

describe('character advancement', () => {
  it('creates characters from level 1 through 20 with custom identity and scores', () => {
    const character = createConfiguredCharacter('player1', '黎恩', '戰士', { level: 10, species: '自訂星裔', background: '鐘塔守望者', abilities: { str: 18, dex: 12, con: 16, int: 10, wis: 13, cha: 8 } });
    expect(character.level).toBe(10);
    expect(character.proficiencyBonus).toBe(4);
    expect(character.species).toBe('自訂星裔');
    expect(character.maxHitDice).toBe(10);
  });

  it('adds a multiclass level without replacing the existing class', () => {
    const fighter = { ...createConfiguredCharacter('player1', '黎恩', '戰士'), experience: 2700 };
    const multiclass = levelUpCharacter(fighter, '法師');
    expect(multiclass.level).toBe(4);
    expect(multiclass.classLevels).toEqual(expect.arrayContaining([expect.objectContaining({ className: '戰士', level: 3 }), expect.objectContaining({ className: '法師', level: 1 })]));
    expect(multiclass.spellcasting).toBeDefined();
  });

  it('allows explicit spell configuration', () => {
    const wizard = createConfiguredCharacter('player1', '米拉', '法師');
    const configured = setPreparedSpells(wizard, ['light', 'shield', 'misty_step']);
    expect(configured.spellcasting?.spells.map((spell) => spell.id)).toEqual(expect.arrayContaining(['light', 'shield', 'misty_step']));
  });

  it('requires XP to level and grants ability points at level 4', () => {
    const fighter = createConfiguredCharacter('player1', '黎恩', '戰士');
    expect(experienceToNextLevel(fighter).ready).toBe(false);
    expect(() => levelUpCharacter(fighter, '戰士')).toThrow(/XP/);
    const ready = grantExperience(fighter, 1800);
    const leveled = levelUpCharacter(ready, '戰士');
    expect(leveled.level).toBe(4);
    expect(leveled.abilityPoints).toBe(2);
    const improved = spendAbilityPoint(leveled, 'str');
    expect(improved.abilities.str).toBe(leveled.abilities.str + 1);
    expect(improved.abilityPoints).toBe(1);
  });
});
