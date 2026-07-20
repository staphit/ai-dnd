// Shared test fixtures mirroring the server View shapes. The rules engine
// lives in Go, so tests build plain data instead of computing sheets.
import type { Campaign, CombatState, Combatant, PlayerCharacter, PlayerId, ScriptProgress } from '../types';

export function makePlayer(id: PlayerId, name: string, overrides: Partial<PlayerCharacter> = {}): PlayerCharacter {
  return {
    id,
    name,
    className: '戰士',
    subclass: '勇士',
    species: '人類',
    background: '士兵',
    level: 3,
    classLevels: [{ className: '戰士', level: 3, subclass: '勇士' }],
    initials: name.slice(0, 1),
    hp: 28,
    maxHp: 28,
    ac: 16,
    passive: 12,
    speed: 30,
    initiative: 2,
    proficiencyBonus: 2,
    hitDie: 10,
    hitDice: 3,
    maxHitDice: 3,
    abilities: { str: 16, dex: 14, con: 14, int: 10, wis: 12, cha: 10 },
    savingThrowProficiencies: ['str', 'con'],
    skills: [
      { name: '運動', ability: 'str', proficient: true, expertise: false, bonus: 5 },
      { name: '隱匿', ability: 'dex', proficient: false, expertise: false, bonus: 2 },
    ],
    attacks: [{ id: 'longsword', name: '長劍', attackBonus: 5, damage: '1d8+3', damageType: '揮砍', properties: [] }],
    equipment: ['長劍', '鎖子甲'],
    resources: [],
    features: [],
    condition: '正常',
    experience: 900,
    ...overrides,
  };
}

export function makeCombatant(overrides: Partial<Combatant> & Pick<Combatant, 'id' | 'name' | 'side'>): Combatant {
  return {
    initiativeBonus: 2,
    initiative: 10,
    ac: 13,
    hp: 10,
    maxHp: 10,
    attackBonus: 4,
    damage: '1d6+2',
    damageType: '穿刺',
    ...overrides,
  };
}

export function makeCombat(overrides: Partial<CombatState> = {}): CombatState {
  return {
    active: true,
    round: 1,
    turnIndex: 0,
    combatants: [],
    ...overrides,
  };
}

export function makeCampaign(overrides: Partial<Campaign> = {}): Campaign {
  return {
    id: 'campaign-test-1',
    updatedAt: '2026-07-17T00:00:00Z',
    setupComplete: true,
    title: '灰燼王冠',
    chapter: '第一章／沉鐘之夜',
    scene: '下城區・無燈禮拜堂',
    round: 1,
    objective: '在午夜鐘響前找到失蹤的製圖師伊薩克',
    objectiveContext: '祭壇下方傳來不自然的敲擊聲。',
    stakes: '午夜鐘響後線索將被淹沒。',
    players: [makePlayer('player1', '艾拉')],
    story: [{ id: 's-1', speaker: 'dm', text: '禮拜堂的門在隊伍身後闔上。', time: '10:00' }],
    pending: {},
    choices: [],
    requiredCheck: null,
    settings: {},
    xpProgress: { player1: { current: 900, required: 2700, remaining: 1800, ready: false, progress: 0.2 } },
    ...overrides,
  };
}

export function makeScript(overrides: Partial<ScriptProgress> = {}): ScriptProgress {
  return {
    scriptId: 'ashen-crown',
    stage: '前期',
    nodeTitle: '沉鐘塔的低語',
    nodeType: 'scene',
    alignment: 0,
    visitedCount: 1,
    totalNodes: 12,
    ended: false,
    ...overrides,
  };
}

// A Response-shaped JSON body for fetch mocks.
export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}
