import type { CombatState, Combatant, PlayerCharacter, PlayerId, SpellEffect } from '../types';
import { abilityModifier } from './characters';
import { rollExpression } from './combat';

type RandomSource = () => number;

export function applyHealing(character: PlayerCharacter, amount: number): PlayerCharacter {
  const hp = Math.min(character.maxHp, character.hp + Math.max(0, amount));
  return { ...character, hp, condition: hp > 0 && character.condition === '倒地' ? '正常' : character.condition };
}

export function applyDamage(combatant: Combatant, amount: number): Combatant {
  const damage = Math.max(0, amount);
  const temporaryHp = Math.max(0, combatant.temporaryHp || 0);
  const absorbed = Math.min(temporaryHp, damage);
  const hp = Math.max(0, combatant.hp - (damage - absorbed));
  return { ...combatant, temporaryHp: temporaryHp - absorbed, hp, defeated: hp === 0 };
}

export function resolveShortRest(character: PlayerCharacter, random: RandomSource = Math.random) {
  let hp = character.hp;
  let hitDice = character.hitDice;
  let diceSpent = 0;
  const constitution = abilityModifier(character.abilities.con);
  while (hp < character.maxHp && hitDice > 0) {
    hp = Math.min(character.maxHp, hp + Math.max(0, Math.floor(random() * character.hitDie) + 1 + constitution));
    hitDice -= 1;
    diceSpent += 1;
  }
  return { character: { ...character, hp, hitDice, condition: hp > 0 && character.condition === '倒地' ? '正常' : character.condition }, healed: hp - character.hp, diceSpent };
}

function amountFor(effect: SpellEffect, caster: PlayerCharacter, random: RandomSource) {
  const dice = effect.dice ? rollExpression(effect.dice, random) : 0;
  const modifier = effect.addAbilityModifier && caster.spellcasting ? abilityModifier(caster.abilities[caster.spellcasting.ability]) : 0;
  return Math.max(0, dice + (effect.flat || 0) + modifier);
}

export function resolveSpellEffect(players: PlayerCharacter[], combat: CombatState | undefined, casterId: PlayerId, targetId: string, effect: SpellEffect, random: RandomSource = Math.random) {
  const caster = players.find((entry) => entry.id === casterId);
  const targetPlayer = players.find((entry) => entry.id === targetId);
  const targetCombatant = combat?.combatants.find((entry) => entry.id === targetId || entry.playerId === targetId);
  if (!caster) throw new Error('找不到施法者');
  if (!targetPlayer && !targetCombatant) throw new Error('找不到法術目標');
  let amount = amountFor(effect, caster, random);
  let outcome = '';
  if (effect.attackRoll && targetCombatant) {
    const attack = Math.floor(random() * 20) + 1 + (caster.spellcasting?.attackBonus || 0);
    if (attack < targetCombatant.ac) { amount = 0; outcome = '法術攻擊未命中'; }
  }
  if (effect.saveAbility && targetCombatant && !outcome) {
    const save = Math.floor(random() * 20) + 1 + Number(targetCombatant.savingThrows?.[effect.saveAbility] || 0);
    if (save >= (caster.spellcasting?.saveDc || 10)) {
      amount = effect.halfOnSave ? Math.floor(amount / 2) : 0;
      outcome = amount ? `豁免成功，承受 ${amount} 點` : '豁免成功，未受影響';
    }
  }
  let nextPlayers = players;
  let nextCombat = combat;
  if (effect.kind === 'healing' && targetPlayer) nextPlayers = players.map((entry) => entry.id === targetPlayer.id ? applyHealing(entry, amount) : entry);
  if (effect.kind === 'temporaryHp' && targetPlayer) nextPlayers = players.map((entry) => entry.id === targetPlayer.id ? { ...entry, temporaryHp: Math.max(entry.temporaryHp || 0, amount) } : entry);
  if (effect.kind === 'condition' && targetPlayer) nextPlayers = players.map((entry) => entry.id === targetPlayer.id ? { ...entry, condition: effect.condition || '正常' } : entry);
  if (effect.kind === 'damage' && targetCombatant && combat) {
    nextCombat = { ...combat, combatants: combat.combatants.map((entry) => entry.id === targetCombatant.id ? applyDamage(entry, amount) : entry) };
    const updated = nextCombat.combatants.find((entry) => entry.id === targetCombatant.id)!;
    if (targetCombatant.playerId) nextPlayers = players.map((entry) => entry.id === targetCombatant.playerId ? { ...entry, hp: updated.hp, temporaryHp: updated.temporaryHp, condition: updated.hp === 0 ? '倒地' : entry.condition } : entry);
  }
  if (targetPlayer && nextCombat && effect.kind !== 'damage') {
    const updatedPlayer = nextPlayers.find((entry) => entry.id === targetPlayer.id)!;
    nextCombat = { ...nextCombat, combatants: nextCombat.combatants.map((entry) => entry.playerId === targetPlayer.id ? { ...entry, hp: updatedPlayer.hp, temporaryHp: updatedPlayer.temporaryHp, defeated: updatedPlayer.hp === 0 } : entry) };
  }
  const targetName = targetCombatant?.name || targetPlayer?.name || '目標';
  if (!outcome) outcome = effect.kind === 'damage' ? `受到 ${amount} 點${effect.damageType || ''}傷害` : effect.kind === 'healing' ? `恢復 ${amount} 點生命` : effect.kind === 'temporaryHp' ? `獲得 ${amount} 點暫時生命` : `狀態變為「${effect.condition}」`;
  return { players: nextPlayers, combat: nextCombat, amount, text: `${targetName}${outcome}。` };
}

export interface DmEffect { targetId: PlayerId; kind: 'damage' | 'healing' | 'condition'; amount?: number; condition?: string; reason: string }

export function applyDmEffects(players: PlayerCharacter[], effects: DmEffect[]) {
  let next = players;
  const logs: string[] = [];
  for (const effect of effects.slice(0, 8)) {
    const target = next.find((entry) => entry.id === effect.targetId);
    if (!target) continue;
    const amount = Math.max(0, Math.min(500, Math.floor(Number(effect.amount || 0))));
    if (effect.kind === 'damage') next = next.map((entry) => {
      if (entry.id !== target.id) return entry;
      const temporaryHp = Math.max(0, entry.temporaryHp || 0);
      const absorbed = Math.min(temporaryHp, amount);
      const hp = Math.max(0, entry.hp - (amount - absorbed));
      return { ...entry, temporaryHp: temporaryHp - absorbed, hp, condition: hp === 0 ? '倒地' : entry.condition };
    });
    if (effect.kind === 'healing') next = next.map((entry) => entry.id === target.id ? applyHealing(entry, amount) : entry);
    if (effect.kind === 'condition') next = next.map((entry) => entry.id === target.id ? { ...entry, condition: String(effect.condition || '正常').slice(0, 40) } : entry);
    logs.push(`${target.name}：${effect.reason}（${effect.kind === 'damage' ? `-${amount} HP` : effect.kind === 'healing' ? `+${amount} HP` : `狀態：${String(effect.condition || '正常').slice(0, 40)}`}）`);
  }
  return { players: next, logs };
}


