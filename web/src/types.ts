export type PlayerId = `player${number}`;
export type Speaker = 'dm' | 'system' | PlayerId;
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

export interface StoryEntry {
  id: string;
  speaker: Speaker;
  text: string;
  time: string;
}

export interface PlayerCharacter {
  id: PlayerId;
  name: string;
  className: string;
  subclass: string;
  species: string;
  background: string;
  level: number;
  initials: string;
  hp: number;
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

export interface Campaign {
  setupComplete: boolean;
  title: string;
  chapter: string;
  scene: string;
  round: number;
  objective: string;
  players: PlayerCharacter[];
  story: StoryEntry[];
  pending: Partial<Record<PlayerId, string>>;
  sceneImage?: {
    url: string;
    scene: string;
    createdAt: string;
    model: string;
  };
}

export interface AiStatus {
  connected: boolean;
  provider: string;
  model: string | null;
  imageModel?: string;
  message?: string;
}

export type Page = 'table' | 'journal' | 'settings';
