// Static display constants and tiny pure helpers shared by leaf components.
// The rules engine lives server-side; only presentation math stays here.
import type { AbilityKey, PlayerCharacter } from './types';

export const abilityLabels: Record<AbilityKey, string> = {
  str: '力量', dex: '敏捷', con: '體質', int: '智力', wis: '感知', cha: '魅力',
};

export function abilityModifier(score: number) {
  return Math.floor((score - 10) / 2);
}

// Display fallback mirroring the server's rules.GetCheckBonus: used only when
// a required check arrives without a server-computed modifier.
export function getCheckBonus(character: PlayerCharacter, check: string): number {
  if (check === '先攻') return character.initiative;
  const skill = character.skills.find((entry) => entry.name === check);
  if (skill) return skill.bonus;
  const ability = (Object.entries(abilityLabels).find(([, label]) => label === check)?.[0] || '') as AbilityKey | '';
  return ability ? abilityModifier(character.abilities[ability]) : 0;
}
