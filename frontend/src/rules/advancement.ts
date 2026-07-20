import type { AbilityScores, CharacterClassLevel, PlayerCharacter, PlayerId } from '../types';
import { abilityModifier, classDefinitions, createLevel3Character, type ClassName } from './characters';
import { makeSpell, spellCatalog } from './spells';

const fullCasters = new Set<ClassName>(['吟遊詩人', '牧師', '德魯伊', '術士', '法師']);
const halfCasters = new Set<ClassName>(['聖武士', '遊俠']);
export const experienceThresholds = [0, 0, 300, 900, 2700, 6500, 14000, 23000, 34000, 48000, 64000, 85000, 100000, 120000, 140000, 165000, 195000, 225000, 265000, 305000, 355000] as const;
const abilityImprovementLevels = new Set([4, 8, 12, 16, 19]);
const slotTable: number[][] = [
  [], [2], [3], [4, 2], [4, 3], [4, 3, 2], [4, 3, 3], [4, 3, 3, 1], [4, 3, 3, 2], [4, 3, 3, 3, 1],
  [4, 3, 3, 3, 2], [4, 3, 3, 3, 2, 1], [4, 3, 3, 3, 2, 1], [4, 3, 3, 3, 2, 1, 1], [4, 3, 3, 3, 2, 1, 1],
  [4, 3, 3, 3, 2, 1, 1, 1], [4, 3, 3, 3, 2, 1, 1, 1], [4, 3, 3, 3, 2, 1, 1, 1, 1],
  [4, 3, 3, 3, 3, 1, 1, 1, 1], [4, 3, 3, 3, 3, 2, 1, 1, 1], [4, 3, 3, 3, 3, 2, 2, 1, 1],
];

function proficiencyForLevel(level: number) {
  return 2 + Math.floor((Math.max(1, level) - 1) / 4);
}

export function experienceForLevel(level: number) {
  return experienceThresholds[Math.min(20, Math.max(1, level))];
}

export function experienceToNextLevel(character: PlayerCharacter) {
  if (character.level >= 20) return { current: character.experience, required: character.experience, remaining: 0, ready: false, progress: 1 };
  const floor = experienceForLevel(character.level);
  const required = experienceForLevel(character.level + 1);
  const current = Math.max(floor, character.experience || 0);
  return { current, required, remaining: Math.max(0, required - current), ready: current >= required, progress: Math.max(0, Math.min(1, (current - floor) / (required - floor))) };
}

export function grantExperience(character: PlayerCharacter, amount: number) {
  return { ...character, experience: Math.max(0, Math.min(9_999_999, (character.experience || experienceForLevel(character.level)) + Math.max(0, Math.floor(amount)))) };
}

function normalizedClasses(character: PlayerCharacter): CharacterClassLevel[] {
  return character.classLevels?.length
    ? character.classLevels
    : [{ className: character.className, level: character.level, subclass: character.subclass }];
}

function casterLevel(classes: CharacterClassLevel[]) {
  return Math.min(20, classes.reduce((total, entry) => {
    const name = entry.className as ClassName;
    return total + (fullCasters.has(name) ? entry.level : halfCasters.has(name) ? Math.ceil(entry.level / 2) : 0);
  }, 0));
}

function recalculate(character: PlayerCharacter): PlayerCharacter {
  const proficiencyBonus = proficiencyForLevel(character.level);
  const con = abilityModifier(character.abilities.con);
  const primary = normalizedClasses(character)[0];
  const hitDie = classDefinitions[primary.className as ClassName]?.hitDie || character.hitDie;
  const average = Math.floor(hitDie / 2) + 1;
  const maxHp = Math.max(character.level, hitDie + con + Math.max(0, character.level - 1) * Math.max(1, average + con));
  const skills = character.skills.map((skill) => ({
    ...skill,
    bonus: abilityModifier(character.abilities[skill.ability]) + (skill.expertise ? proficiencyBonus * 2 : skill.proficient ? proficiencyBonus : 0),
  }));
  const attacks = character.attacks.map((attack) => {
    const dexterity = attack.properties.some((value) => /靈巧|彈藥|遠程/.test(value));
    const modifier = abilityModifier(character.abilities[dexterity ? 'dex' : 'str']);
    const diePart = attack.damage.match(/^\d+d\d+/)?.[0] || attack.damage;
    return { ...attack, attackBonus: proficiencyBonus + modifier, damage: `${diePart}${modifier >= 0 ? '+' : ''}${modifier}` };
  });
  const perception = skills.find((skill) => skill.name === '察覺')?.bonus || abilityModifier(character.abilities.wis);
  const casting = character.spellcasting;
  const level = casterLevel(normalizedClasses(character));
  const slotRow = slotTable[level] || [];
  const spellcasting = casting ? {
    ...casting,
    attackBonus: proficiencyBonus + abilityModifier(character.abilities[casting.ability]),
    saveDc: 8 + proficiencyBonus + abilityModifier(character.abilities[casting.ability]),
    slots: casting.mode === 'pact'
      ? [{ level: Math.min(5, Math.max(1, Math.ceil(character.level / 2))), max: Math.min(4, 1 + Math.floor(character.level / 5)), current: Math.min(casting.slots[0]?.current || 0, Math.min(4, 1 + Math.floor(character.level / 5))) }]
      : slotRow.map((max, index) => ({ level: index + 1, max, current: Math.min(casting.slots.find((slot) => slot.level === index + 1)?.current ?? max, max) })),
  } : undefined;
  return {
    ...character,
    className: normalizedClasses(character).map((entry) => entry.className).join('／'),
    subclass: normalizedClasses(character).map((entry) => entry.subclass).filter(Boolean).join('／'),
    proficiencyBonus,
    hitDie,
    maxHp,
    hp: Math.min(character.hp, maxHp),
    hitDice: Math.min(character.hitDice, character.level),
    maxHitDice: character.level,
    passive: 10 + perception,
    initiative: abilityModifier(character.abilities.dex),
    skills,
    attacks,
    spellcasting,
  };
}

export interface CharacterBuildOptions {
  level?: number;
  species?: string;
  background?: string;
  abilities?: AbilityScores;
}

export function createConfiguredCharacter(id: PlayerId, name: string, className: ClassName, options: CharacterBuildOptions = {}) {
  const base = createLevel3Character(id, name, className);
  const level = Math.min(20, Math.max(1, options.level || 3));
  return recalculate({
    ...base,
    level,
    classLevels: [{ className, level, subclass: base.subclass }],
    species: options.species?.trim() || base.species,
    background: options.background?.trim() || base.background,
    abilities: options.abilities || base.abilities,
    hitDice: level,
    maxHitDice: level,
    experience: experienceForLevel(level),
    abilityPoints: 0,
  });
}

export function customizeCharacter(character: PlayerCharacter, patch: Pick<CharacterBuildOptions, 'species' | 'background' | 'abilities'>) {
  return recalculate({ ...character, ...patch });
}

export function levelUpCharacter(character: PlayerCharacter, className: ClassName): PlayerCharacter {
  if (character.level >= 20) throw new Error('角色總等級已達 20。');
  if (!experienceToNextLevel(character).ready) throw new Error(`尚缺 ${experienceToNextLevel(character).remaining} XP 才能升級。`);
  const classes = normalizedClasses(character);
  const existing = classes.find((entry) => entry.className === className);
  const starter = createLevel3Character(character.id, character.name, className);
  const classLevels = existing
    ? classes.map((entry) => entry.className === className ? { ...entry, level: entry.level + 1 } : entry)
    : [...classes, { className, level: 1, subclass: starter.subclass }];
  const mergeById = <T extends { id: string }>(left: T[], right: T[]) => [...left, ...right.filter((candidate) => !left.some((entry) => entry.id === candidate.id))];
  const nextLevel = character.level + 1;
  const nextClassLevel = (existing?.level || 0) + 1;
  const progressionFeature = {
    id: `progression-${className}-${nextClassLevel}`,
    name: `${className} ${nextClassLevel} 級進展`,
    description: abilityImprovementLevels.has(nextLevel)
      ? '解鎖 2 點能力值提升，可在角色成長頁分配；生命、熟練與法術進展亦已重新計算。'
      : `解鎖 ${className} 第 ${nextClassLevel} 級進展；生命值、熟練加值、攻擊與法術位依新等級重新計算。`,
  };
  return recalculate({
    ...character,
    level: nextLevel,
    classLevels,
    hp: character.hp + Math.max(1, Math.floor(starter.hitDie / 2) + 1 + abilityModifier(character.abilities.con)),
    hitDice: character.hitDice + 1,
    attacks: mergeById(character.attacks, starter.attacks),
    resources: mergeById(character.resources, starter.resources),
    features: mergeById(mergeById(character.features, starter.features), [progressionFeature]),
    spellcasting: character.spellcasting || starter.spellcasting,
    abilityPoints: (character.abilityPoints || 0) + (abilityImprovementLevels.has(nextLevel) ? 2 : 0),
  });
}

export function spendAbilityPoint(character: PlayerCharacter, ability: keyof AbilityScores) {
  const points = character.abilityPoints || 0;
  if (points < 1) throw new Error('目前沒有可分配的能力值點數。');
  if (character.abilities[ability] >= 20) throw new Error(`${ability} 已達一般上限 20。`);
  return recalculate({ ...character, abilities: { ...character.abilities, [ability]: character.abilities[ability] + 1 }, abilityPoints: points - 1 });
}

export function setPreparedSpells(character: PlayerCharacter, spellIds: string[]): PlayerCharacter {
  if (!character.spellcasting) return character;
  const selected = new Set(spellIds);
  const existing = new Map(character.spellcasting.spells.map((spell) => [spell.id, spell]));
  const spells = Object.keys(spellCatalog)
    .filter((id) => selected.has(id) || existing.get(id)?.alwaysPrepared)
    .map((id) => existing.get(id) || makeSpell(id, { prepared: true, inSpellbook: true }));
  return { ...character, spellcasting: { ...character.spellcasting, spells } };
}

export function getCharacterClasses(character: PlayerCharacter) {
  return normalizedClasses(character);
}

