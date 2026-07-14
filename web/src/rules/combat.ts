import type { CombatState, Combatant, PlayerCharacter } from '../types';

export interface AttackResolution {
  attackRoll: number;
  total: number;
  hit: boolean;
  critical: boolean;
  damage: number;
  text: string;
}

type RandomSource = () => number;

function die(sides: number, random: RandomSource) {
  return Math.floor(random() * sides) + 1;
}

export function rollExpression(expression: string, random: RandomSource = Math.random, critical = false): number {
  const match = expression.trim().match(/^(\d+)d(\d+)(?:\s*([+-])\s*(\d+))?$/i);
  if (!match) throw new Error(`無法辨識傷害骰：${expression}`);
  const count = Number(match[1]) * (critical ? 2 : 1);
  const sides = Number(match[2]);
  const modifier = match[4] ? Number(match[4]) * (match[3] === '-' ? -1 : 1) : 0;
  if (count < 1 || count > 40 || sides < 2 || sides > 100) throw new Error('傷害骰超出允許範圍');
  return Math.max(0, Array.from({ length: count }, () => die(sides, random)).reduce((sum, value) => sum + value, modifier));
}

export function partyCombatants(players: PlayerCharacter[]): Combatant[] {
  return players.map((player) => {
    const attack = player.attacks[0];
    return {
      id: player.id,
      playerId: player.id,
      name: player.name,
      side: 'party',
      initiativeBonus: player.initiative,
      initiative: 0,
      ac: player.ac,
      hp: player.hp,
      temporaryHp: player.temporaryHp || 0,
      maxHp: player.maxHp,
      attackBonus: attack?.attackBonus || 0,
      damage: attack?.damage || '1d4',
      damageType: attack?.damageType || '鈍擊',
      defeated: player.hp <= 0,
    };
  });
}

export function startCombat(combatants: Combatant[], random: RandomSource = Math.random, firstTurn: 'initiative' | 'enemy' = 'initiative'): CombatState {
  const rolled = combatants.map((combatant) => ({
    ...combatant,
    initiative: die(20, random) + combatant.initiativeBonus,
    defeated: combatant.hp <= 0,
  })).sort((a, b) => b.initiative - a.initiative || b.initiativeBonus - a.initiativeBonus || a.name.localeCompare(b.name, 'zh-TW'));
  const enemyIndex = firstTurn === 'enemy' ? rolled.findIndex((entry) => entry.side === 'enemy' && !entry.defeated) : -1;
  return { active: true, round: 1, turnIndex: enemyIndex >= 0 ? enemyIndex : 0, combatants: rolled, turnEconomy: Object.fromEntries(rolled.map((entry) => [entry.id, { actionUsed: false, bonusActionUsed: false, reactionUsed: false }])) };
}

export type CombatResource = 'action' | 'bonusAction' | 'reaction';

export function combatResourceForCastingTime(castingTime: string): CombatResource {
  if (/附贈動作/.test(castingTime)) return 'bonusAction';
  if (/反應/.test(castingTime)) return 'reaction';
  return 'action';
}

export function spendCombatResource(state: CombatState, combatantId: string, resource: CombatResource): CombatState {
  const actor = state.combatants.find((entry) => entry.id === combatantId || entry.playerId === combatantId);
  if (!actor) throw new Error('找不到要消耗行動次數的戰鬥角色。');
  const current = state.combatants[state.turnIndex];
  if (resource !== 'reaction' && current?.id !== actor.id) throw new Error(`現在是 ${current?.name || '其他角色'} 的回合。`);
  const economy = state.turnEconomy || {};
  const usage = economy[actor.id] || { actionUsed: false, bonusActionUsed: false, reactionUsed: false };
  const key = resource === 'action' ? 'actionUsed' : resource === 'bonusAction' ? 'bonusActionUsed' : 'reactionUsed';
  if (usage[key]) throw new Error(`${actor.name}本輪的${resource === 'action' ? '動作' : resource === 'bonusAction' ? '附贈動作' : '反應'}已使用。`);
  return { ...state, turnEconomy: { ...economy, [actor.id]: { ...usage, [key]: true } } };
}

export function advanceTurn(state: CombatState): CombatState {
  if (!state.active || state.combatants.length === 0) return state;
  let next = state.turnIndex;
  let attempts = 0;
  do {
    next = (next + 1) % state.combatants.length;
    attempts += 1;
  } while (state.combatants[next]?.defeated && attempts < state.combatants.length);
  const wrapped = next <= state.turnIndex;
  const nextActor = state.combatants[next];
  return { ...state, turnIndex: next, round: state.round + (wrapped ? 1 : 0), turnEconomy: { ...(state.turnEconomy || {}), ...(nextActor ? { [nextActor.id]: { actionUsed: false, bonusActionUsed: false, reactionUsed: false } } : {}) } };
}

export function resolveAttack(
  state: CombatState,
  attackerId: string,
  targetId: string,
  random: RandomSource = Math.random,
  advantage: 'normal' | 'advantage' | 'disadvantage' = 'normal',
): { state: CombatState; resolution: AttackResolution } {
  const attacker = state.combatants.find((entry) => entry.id === attackerId);
  const target = state.combatants.find((entry) => entry.id === targetId);
  if (!attacker || !target) throw new Error('找不到攻擊者或目標');
  if (attacker.defeated) throw new Error(`${attacker.name} 已失去戰鬥能力`);
  const first = die(20, random);
  const second = advantage === 'normal' ? first : die(20, random);
  const attackRoll = advantage === 'advantage' ? Math.max(first, second) : advantage === 'disadvantage' ? Math.min(first, second) : first;
  const critical = attackRoll === 20;
  const total = attackRoll + attacker.attackBonus;
  const hit = attackRoll !== 1 && (critical || total >= target.ac);
  const damage = hit ? rollExpression(attacker.damage, random, critical) : 0;
  const temporaryHp = Math.max(0, target.temporaryHp || 0);
  const absorbed = Math.min(temporaryHp, damage);
  const nextHp = Math.max(0, target.hp - (damage - absorbed));
  const combatants = state.combatants.map((entry) => entry.id === targetId
    ? { ...entry, temporaryHp: temporaryHp - absorbed, hp: nextHp, defeated: nextHp === 0 }
    : entry);
  const text = hit
    ? `${attacker.name}以 ${total} 命中${critical ? '並造成重擊' : ''}，對 ${target.name} 造成 ${damage} 點${attacker.damageType}傷害。`
    : `${attacker.name}的攻擊結果為 ${total}，未命中 ${target.name}（AC ${target.ac}）。`;
  return { state: { ...state, combatants }, resolution: { attackRoll, total, hit, critical, damage, text } };
}

export function syncPlayersFromCombat(players: PlayerCharacter[], state: CombatState): PlayerCharacter[] {
  return players.map((player) => {
    const combatant = state.combatants.find((entry) => entry.playerId === player.id);
    return combatant ? { ...player, hp: combatant.hp, temporaryHp: combatant.temporaryHp, condition: combatant.hp === 0 ? '倒地' : player.condition } : player;
  });
}
