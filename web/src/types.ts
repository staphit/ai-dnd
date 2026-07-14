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
  resources: CharacterResource[];
  features: ClassFeature[];
  spellcasting?: CharacterSpellcasting;
  concentration?: string;
  condition: string;
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
}

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
  selectedModel?: string;
  players: PlayerCharacter[];
  story: StoryEntry[];
  pending: Partial<Record<PlayerId, string>>;
  combat?: CombatState;
  sceneImage?: {
    url: string;
    scene: string;
    createdAt: string;
    model: string;
  };
}

export interface CampaignSummary {
  id: string;
  title: string;
  updatedAt: string;
  round: number;
}

export interface AiStatus {
  connected: boolean;
  provider: string;
  model: string | null;
  models?: Array<{ id: string; label: string }>;
  imageModel?: string;
  message?: string;
}

export type Page = 'table' | 'combat' | 'characters' | 'journal' | 'settings';
