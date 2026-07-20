export type PlayerId = `player${number}`;
export type Speaker = 'dm' | 'system' | PlayerId;
export type MessageAudience = 'public' | PlayerId;
export type AbilityKey = 'str' | 'dex' | 'con' | 'int' | 'wis' | 'cha';
export type RestType = 'short' | 'long';

export interface AbilityScores {
  str: number;
  dex: number;
  con: number;
  int: number;
  wis: number;
  cha: number;
}

export interface CharacterSkill {
  name: string;
  ability: AbilityKey;
  proficient: boolean;
  expertise: boolean;
  bonus: number;
}

export interface CharacterAttack {
  id: string;
  name: string;
  attackBonus: number;
  damage: string;
  damageType: string;
  properties: string[];
  /** Blacksmith enhancement level (+1 hit / +1 damage each). */
  upgradeLevel?: number;
  /** Light weapons strike twice per action; others once. */
  attacksPerAction?: number;
}

export interface CharacterResource {
  id: string;
  name: string;
  current: number;
  max: number;
  die?: string;
  description: string;
  shortRestRecovery: number | 'all';
}

export interface ClassFeature {
  id: string;
  name: string;
  description: string;
}

export interface CharacterSpell {
  id: string;
  name: string;
  englishName: string;
  level: number;
  school: string;
  castingTime: string;
  range: string;
  duration: string;
  description: string;
  concentration: boolean;
  ritual: boolean;
  prepared: boolean;
  alwaysPrepared: boolean;
  inSpellbook: boolean;
  freeUseResourceId?: string;
  effect?: SpellEffect;
}

export interface SpellEffect {
  kind: 'damage' | 'healing' | 'temporaryHp' | 'condition';
  target: 'self' | 'ally' | 'creature';
  dice?: string;
  flat?: number;
  addAbilityModifier?: boolean;
  attackRoll?: boolean;
  automaticHit?: boolean;
  saveAbility?: AbilityKey;
  halfOnSave?: boolean;
  condition?: string;
  damageType?: string;
}

export interface SpellSlotPool {
  level: number;
  current: number;
  max: number;
}

export interface CharacterSpellcasting {
  ability: AbilityKey;
  attackBonus: number;
  saveDc: number;
  focus: string;
  mode: 'standard' | 'pact';
  pactSlotLevel?: number;
  slots: SpellSlotPool[];
  spells: CharacterSpell[];
}

export interface CharacterClassLevel {
  className: string;
  level: number;
  subclass?: string;
}

export interface StoryEntry {
  id: string;
  speaker: Speaker;
  text: string;
  time: string;
  audience?: MessageAudience;
}

export interface RequiredCheck {
  character: string;
  ability: string;
  skill: string;
  dc: number;
  reason: string;
  modifier?: number;
  playerId?: string;
  // Server bookkeeping: the scripted-module choice this check gates.
  scriptChoiceId?: string;
}

// A DM-suggested next action. playerId ties it to the character it suits;
// an absent playerId means it applies to the whole party.
export interface Choice {
  text: string;
  playerId?: PlayerId;
}

export interface PlayerCharacter {
  id: PlayerId;
  name: string;
  className: string;
  subclass: string;
  species: string;
  background: string;
  level: number;
  classLevels?: CharacterClassLevel[];
  initials: string;
  hp: number;
  temporaryHp?: number;
  maxHp: number;
  ac: number;
  passive: number;
  speed: number;
  initiative: number;
  proficiencyBonus: number;
  hitDie: number;
  hitDice: number;
  maxHitDice: number;
  abilities: AbilityScores;
  savingThrowProficiencies: AbilityKey[];
  skills: CharacterSkill[];
  attacks: CharacterAttack[];
  equipment: string[];
  /** Carried gold (gp); chests, quest rewards, and the merchant move it. */
  gold?: number;
  /** Blacksmith armor enhancement already folded into AC. */
  armorUpgrade?: number;
  resources: CharacterResource[];
  features: ClassFeature[];
  spellcasting?: CharacterSpellcasting;
  concentration?: string;
  condition: string;
  experience: number;
  abilityPoints?: number;
  appearance?: string;
  portraitUrl?: string;
}

export interface Combatant {
  id: string;
  name: string;
  side: 'party' | 'enemy' | 'neutral';
  playerId?: PlayerId;
  initiativeBonus: number;
  initiative: number;
  ac: number;
  hp: number;
  temporaryHp?: number;
  maxHp: number;
  attackBonus: number;
  damage: string;
  damageType: string;
  savingThrows?: Partial<Record<AbilityKey, number>>;
  defeated?: boolean;
}

export interface CombatState {
  active: boolean;
  round: number;
  turnIndex: number;
  combatants: Combatant[];
  turnEconomy?: Record<string, { actionUsed: boolean; bonusActionUsed: boolean; reactionUsed: boolean }>;
}

// Per-player experience progress, computed server-side (view.xpProgress).
export interface XpProgress {
  current: number;
  required: number;
  remaining: number;
  ready: boolean;
  progress: number;
}

export interface SceneImage {
  url: string;
  scene: string;
  createdAt: string;
  model: string;
}

// Typed shape of the per-campaign settings document stored server-side in
// campaign.settings (shallow-merged via PATCH /api/campaign/{id}/settings).
export interface CampaignSettings {
  storyId?: string;
  selectedModel?: string;
  selectedEffort?: string;
  /** Storyteller backend: "codex" | "grok" (server DM_PROVIDER default when empty). */
  dmProvider?: string;
  /** @deprecated Image gen is GPT/Codex only; ignored by server. */
  imageBackend?: string;
  fontScale?: number;
  showStatHints?: boolean;
  autoSceneImages?: boolean;
  dismissedTips?: string[];
  sceneImages?: SceneImage[];
}

export interface DmProviderInfo {
  id: string;
  label: string;
  connected: boolean;
  model: string;
  models?: Array<{ id: string; label: string }>;
  efforts?: Array<{ id: string; label: string }>;
  message?: string;
}

// Campaign mirrors the server View (backend/internal/game/service.go): every
// mutating endpoint returns this whole shape and the client renders it as-is.
export interface Campaign {
  schemaVersion?: number;
  id?: string;
  updatedAt?: string;
  setupComplete: boolean;
  title: string;
  chapter: string;
  scene: string;
  round: number;
  objective: string;
  objectiveContext: string;
  stakes: string;
  players: PlayerCharacter[];
  story: StoryEntry[];
  pending: Partial<Record<PlayerId, string>>;
  choices?: Choice[];
  // English SD prompt for the current scene, produced by the DM agent and
  // reused by scene-image generation.
  imagePrompt?: string;
  requiredCheck?: RequiredCheck | null;
  combat?: CombatState;
  storyArc?: StoryArc;
  script?: ScriptProgress;
  settings?: Record<string, unknown>;
  xpProgress?: Partial<Record<PlayerId, XpProgress>>;
}

// Story pacing arc: three acts with round deadlines and timed XP rewards.
export interface ArcPhase {
  stage: string; // 前期 | 中期 | 後期
  goal: string;
  deadlineRound: number;
  rewardXp: number;
  completedRound?: number;
  rewardGranted?: boolean;
}

export interface StoryArc {
  phases: ArcPhase[];
  current: number;
  ended?: boolean;
}

// Scripted-campaign progress mirrored from the server View (script.go).
export interface ScriptProgress {
  scriptId: string;
  stage: string; // 前期 | 中期 | 後期 | 結局
  nodeTitle: string;
  nodeType: string;
  alignment: number;
  visitedCount: number;
  totalNodes: number;
  ended: boolean;
  ending?: 'good' | 'bad' | 'neutral';
}

export interface CampaignSummary {
  id: string;
  title: string;
  scene?: string;
  updatedAt: string;
  round: number;
}

export interface AiStatus {
  connected: boolean;
  provider: string;
  model: string | null;
  models?: Array<{ id: string; label: string }>;
  efforts?: Array<{ id: string; label: string }>;
  imageModel?: string;
  imageBackends?: Array<{ id: string; label: string }>;
  imageBackend?: string;
  message?: string;
  dmProvider?: string;
  dmProviders?: DmProviderInfo[];
}

export type Page = 'table' | 'characters' | 'journal' | 'settings';
