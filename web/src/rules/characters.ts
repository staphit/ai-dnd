import type {
  AbilityKey,
  AbilityScores,
  CharacterAttack,
  CharacterResource,
  CharacterSkill,
  CharacterSpell,
  ClassFeature,
  PlayerCharacter,
  PlayerId,
  RestType,
} from '../types';
import { makeSpell } from './spells';

export const abilityLabels: Record<AbilityKey, string> = {
  str: '力量', dex: '敏捷', con: '體質', int: '智力', wis: '感知', cha: '魅力',
};

const skillAbilities: Record<string, AbilityKey> = {
  '運動': 'str',
  '雜技': 'dex', '巧手': 'dex', '隱匿': 'dex',
  '奧秘': 'int', '歷史': 'int', '調查': 'int', '自然': 'int', '宗教': 'int',
  '馴獸': 'wis', '洞悉': 'wis', '醫藥': 'wis', '察覺': 'wis', '求生': 'wis',
  '欺瞞': 'cha', '威嚇': 'cha', '表演': 'cha', '說服': 'cha',
};

export type ClassName = '野蠻人' | '吟遊詩人' | '牧師' | '德魯伊' | '戰士' | '武僧' | '聖武士' | '遊俠' | '盜賊' | '術士' | '魔契師' | '法師';
type ResourceTemplate = Omit<CharacterResource, 'current' | 'max'> & { max: number | AbilityKey };
type AttackTemplate = Omit<CharacterAttack, 'attackBonus' | 'damage'> & { ability: AbilityKey; damageDie: string };

interface SpellcastingTemplate {
  ability: AbilityKey;
  focus: string;
  mode: 'standard' | 'pact';
  pactSlotLevel?: number;
  slots: Array<{ level: number; max: number }>;
  cantrips: string[];
  prepared: string[];
  alwaysPrepared?: string[];
  spellbook?: string[];
}

interface ClassDefinition {
  subclass: string;
  background: string;
  hitDie: number;
  maxHpBonus?: number;
  abilities: AbilityScores;
  ac: number;
  speed: number;
  saves: AbilityKey[];
  proficientSkills: string[];
  expertiseSkills?: string[];
  equipment: string[];
  attacks: AttackTemplate[];
  resources: ResourceTemplate[];
  features: ClassFeature[];
  spellcasting?: SpellcastingTemplate;
}

const feature = (id: string, name: string, description: string): ClassFeature => ({ id, name, description });
const resource = (
  id: string,
  name: string,
  max: number | AbilityKey,
  description: string,
  shortRestRecovery: number | 'all' = 0,
  die?: string,
): ResourceTemplate => ({ id, name, max, description, shortRestRecovery, die });
const attack = (
  id: string,
  name: string,
  ability: AbilityKey,
  damageDie: string,
  damageType: string,
  properties: string[],
): AttackTemplate => ({ id, name, ability, damageDie, damageType, properties });

export const classDefinitions: Record<ClassName, ClassDefinition> = {
  '野蠻人': {
    subclass: '狂戰士道途', background: '士兵', hitDie: 12,
    abilities: { str: 17, dex: 14, con: 16, int: 8, wis: 12, cha: 10 }, ac: 15, speed: 30,
    saves: ['str', 'con'], proficientSkills: ['運動', '察覺', '威嚇', '求生'],
    equipment: ['巨斧', '手斧 ×4', '探索套組'],
    attacks: [attack('greataxe', '巨斧', 'str', '1d12', '揮砍', ['重型', '雙手', '劈砍精通']), attack('handaxe', '手斧', 'str', '1d6', '揮砍', ['輕型', '投擲 20/60', '煩擾精通'])],
    resources: [resource('rage', '狂暴', 3, '附贈動作啟動；力量攻擊傷害 +2，並抵抗鈍擊、穿刺與揮砍。', 1)],
    features: [
      feature('unarmored_defense', '無甲防禦', '未著甲時 AC = 10 + 敏捷 + 體質調整值。'),
      feature('reckless_attack', '魯莽攻擊', '以力量進行第一次攻擊時可取得優勢，但敵人攻擊你也具有優勢。'),
      feature('danger_sense', '危險感知', '未失能時，對可見效果的敏捷豁免具有優勢。'),
      feature('frenzy', '狂戰', '狂暴期間首次命中可造成額外狂暴傷害骰。'),
      feature('primal_knowledge', '原始知識', '額外熟練一項野蠻人技能，並可在狂暴時以力量進行部分技能檢定。'),
    ],
  },
  '吟遊詩人': {
    subclass: '逸聞學院', background: '藝人', hitDie: 8,
    abilities: { str: 8, dex: 14, con: 14, int: 10, wis: 12, cha: 17 }, ac: 14, speed: 30,
    saves: ['dex', 'cha'], proficientSkills: ['表演', '說服', '洞悉', '巧手', '奧秘', '歷史'], expertiseSkills: ['表演', '說服'],
    equipment: ['鑲釘皮甲', '細劍', '短弓與 20 支箭', '魯特琴', '藝人套組'],
    attacks: [attack('rapier', '細劍', 'dex', '1d8', '穿刺', ['靈巧', '煩擾精通']), attack('shortbow', '短弓', 'dex', '1d6', '穿刺', ['彈藥 80/320', '雙手'])],
    resources: [resource('bardic_inspiration', '吟遊激勵', 'cha', '附贈動作給予 d6；目標檢定失敗時可加入骰值。', 0, 'd6')],
    features: [feature('jack_of_all_trades', '萬事通', '未熟練的技能檢定仍可加入一半熟練加值。'), feature('expertise', '專精', '表演與說服使用雙倍熟練加值。'), feature('cutting_words', '尖刻語句', '反應消耗吟遊激勵，降低敵人的攻擊、傷害或能力檢定。')],
    spellcasting: {
      ability: 'cha', focus: '樂器', mode: 'standard', slots: [{ level: 1, max: 4 }, { level: 2, max: 2 }],
      cantrips: ['dancing_lights', 'vicious_mockery'],
      prepared: ['charm_person', 'dissonant_whispers', 'healing_word', 'thunderwave', 'invisibility', 'suggestion'],
    },
  },
  '牧師': {
    subclass: '生命領域', background: '侍僧', hitDie: 8,
    abilities: { str: 14, dex: 10, con: 15, int: 8, wis: 17, cha: 12 }, ac: 18, speed: 30,
    saves: ['wis', 'cha'], proficientSkills: ['洞悉', '宗教', '醫藥', '說服'],
    equipment: ['鏈甲', '盾牌', '戰鎚', '聖徽', '祭司套組'],
    attacks: [attack('warhammer', '戰鎚', 'str', '1d8', '鈍擊', ['多用途 1d10', '推離精通'])],
    resources: [resource('channel_divinity', '引導神力', 2, '使用神聖火花、驅散不死生物或領域效果。', 1)],
    features: [feature('divine_order', '神聖使命：守護者', '熟練軍用武器並受訓重甲。'), feature('disciple_of_life', '生命門徒', '以法術位治療時，額外恢復 2 + 法術位環級生命。'), feature('preserve_life', '維持生命', '引導神力治療 30 呎內生物，分配相當於牧師等級五倍的生命。')],
    spellcasting: {
      ability: 'wis', focus: '聖徽', mode: 'standard', slots: [{ level: 1, max: 4 }, { level: 2, max: 2 }],
      cantrips: ['guidance', 'sacred_flame', 'thaumaturgy'],
      prepared: ['command', 'guiding_bolt', 'healing_word', 'shield_of_faith', 'hold_person', 'silence'],
      alwaysPrepared: ['aid', 'bless', 'cure_wounds', 'lesser_restoration'],
    },
  },
  '德魯伊': {
    subclass: '大地結社（溫帶）', background: '隱士', hitDie: 8,
    abilities: { str: 8, dex: 14, con: 15, int: 10, wis: 17, cha: 12 }, ac: 15, speed: 30,
    saves: ['int', 'wis'], proficientSkills: ['自然', '求生', '察覺', '醫藥'],
    equipment: ['皮甲', '木盾', '彎刀', '德魯伊法器', '探索套組'],
    attacks: [attack('scimitar', '彎刀', 'dex', '1d6', '揮砍', ['靈巧', '輕型', '迅捷精通'])],
    resources: [resource('wild_shape', '荒野形態', 2, '附贈動作變形成已知野獸形態，或用荒野夥伴召喚魔寵。', 1)],
    features: [feature('druidic', '德魯伊語', '能留下隱密自然訊息，且永遠準備動物交談。'), feature('primal_order', '原始使命：守衛者', '熟練軍用武器並受訓中甲。'), feature('wild_companion', '荒野夥伴', '消耗荒野形態使用次數施展尋獲魔寵。'), feature('circle_spells', '結社法術：溫帶', '永遠準備迷蹤步、電爪與睡眠術；長休後可改選其他地貌。'), feature('land_aid', '大地援助', '消耗荒野形態，以自然力量傷害敵人並治療盟友。')],
    spellcasting: {
      ability: 'wis', focus: '德魯伊法器', mode: 'standard', slots: [{ level: 1, max: 4 }, { level: 2, max: 2 }],
      cantrips: ['druidcraft', 'produce_flame'],
      prepared: ['animal_friendship', 'cure_wounds', 'faerie_fire', 'thunderwave', 'entangle', 'moonbeam'],
      alwaysPrepared: ['speak_with_animals', 'misty_step', 'shocking_grasp', 'sleep'],
    },
  },
  '戰士': {
    subclass: '勇士', background: '士兵', hitDie: 10,
    abilities: { str: 17, dex: 12, con: 16, int: 10, wis: 13, cha: 8 }, ac: 19, speed: 30,
    saves: ['str', 'con'], proficientSkills: ['運動', '察覺', '威嚇', '求生'],
    equipment: ['鏈甲', '盾牌', '長劍', '輕弩與 20 支弩矢', '地城探索套組'],
    attacks: [attack('longsword', '長劍', 'str', '1d8', '揮砍', ['多用途 1d10', '削弱精通']), attack('light_crossbow', '輕弩', 'dex', '1d8', '穿刺', ['彈藥 80/320', '裝填', '緩速精通'])],
    resources: [resource('second_wind', '回氣', 2, '附贈動作恢復 1d10 + 3 生命。', 1), resource('action_surge', '動作如潮', 1, '在自己的回合額外進行一個動作，但不能是魔法動作。', 'all')],
    features: [feature('fighting_style', '戰鬥風格：防禦', '穿著護甲時 AC +1。'), feature('weapon_mastery', '武器精通', '可運用三種武器的精通屬性。'), feature('tactical_mind', '戰術心智', '能力檢定失敗時可消耗回氣，增加 1d10。'), feature('improved_critical', '精通重擊', '武器與徒手攻擊骰出 19–20 即造成重擊。'), feature('remarkable_athlete', '卓越運動員', '先攻與力量（運動）檢定具有優勢。')],
  },
  '武僧': {
    subclass: '散打勇士', background: '隱士', hitDie: 8,
    abilities: { str: 8, dex: 17, con: 14, int: 10, wis: 16, cha: 12 }, ac: 16, speed: 40,
    saves: ['str', 'dex'], proficientSkills: ['雜技', '洞悉', '隱匿', '宗教'],
    equipment: ['長矛', '匕首 ×5', '探索套組', '工匠工具'],
    attacks: [attack('unarmed', '徒手打擊', 'dex', '1d6', '鈍擊', ['武藝', '附贈動作可再攻擊']), attack('spear', '長矛', 'dex', '1d6', '穿刺', ['多用途 1d8', '投擲 20/60', '削弱精通'])],
    resources: [resource('focus', '專注點', 3, '驅動疾風連擊、耐心防禦、疾步如風及子職業招式。', 'all'), resource('uncanny_metabolism', '超常代謝', 1, '擲先攻時恢復所有專注點並回復 3 + 1d6 生命。', 0)],
    features: [feature('martial_arts', '武藝 d6', '徒手與武僧武器可使用敏捷，並可附贈動作徒手攻擊。'), feature('unarmored_defense', '無甲防禦', '未著甲且未持盾時 AC = 10 + 敏捷 + 感知。'), feature('deflect_attacks', '卸勁攻擊', '反應使鈍擊、穿刺或揮砍傷害減少 1d10 + 敏捷 + 3。'), feature('open_hand_technique', '散打技法', '疾風連擊命中時可附加失去反應、推離或擊倒效果。')],
  },
  '聖武士': {
    subclass: '奉獻之誓', background: '貴族', hitDie: 10,
    abilities: { str: 17, dex: 10, con: 14, int: 8, wis: 12, cha: 16 }, ac: 18, speed: 30,
    saves: ['wis', 'cha'], proficientSkills: ['運動', '說服', '洞悉', '宗教'],
    equipment: ['鏈甲', '盾牌', '長劍', '標槍 ×6', '聖徽', '祭司套組'],
    attacks: [attack('longsword', '長劍', 'str', '1d8', '揮砍', ['多用途 1d10', '削弱精通']), attack('javelin', '標槍', 'str', '1d6', '穿刺', ['投擲 30/120', '緩速精通'])],
    resources: [resource('lay_on_hands', '聖療', 15, '附贈動作從治療池恢復生命，或花費 5 點移除中毒。', 0), resource('channel_divinity', '引導神力', 2, '發動神聖感知或神聖武器。', 1), resource('free_divine_smite', '免費至聖斬', 1, '每次長休可不消耗法術位施放一次至聖斬。', 0)],
    features: [feature('fighting_style', '戰鬥風格：防護', '持盾時能以反應保護鄰近盟友。'), feature('paladins_smite', '至聖斬', '永遠準備至聖斬，且每次長休可免費施放一次。'), feature('divine_sense', '神聖感知', '引導神力感知 60 呎內天界、邪魔、不死生物及聖化或褻瀆區域。'), feature('sacred_weapon', '神聖武器', '引導神力使近戰武器攻擊加入魅力調整值並可造成光耀傷害。')],
    spellcasting: {
      ability: 'cha', focus: '聖徽', mode: 'standard', slots: [{ level: 1, max: 3 }],
      cantrips: [], prepared: ['bless', 'command', 'cure_wounds', 'divine_favor'],
      alwaysPrepared: ['divine_smite', 'protection_evil_good', 'shield_of_faith'],
    },
  },
  '遊俠': {
    subclass: '獵人', background: '嚮導', hitDie: 10,
    abilities: { str: 10, dex: 17, con: 14, int: 8, wis: 16, cha: 12 }, ac: 16, speed: 30,
    saves: ['str', 'dex'], proficientSkills: ['察覺', '隱匿', '求生', '自然', '馴獸'], expertiseSkills: ['求生'],
    equipment: ['鱗甲', '長弓與 20 支箭', '短劍 ×2', '探索套組'],
    attacks: [attack('longbow', '長弓', 'dex', '1d8', '穿刺', ['彈藥 150/600', '重型', '雙手', '緩速精通']), attack('shortsword', '短劍', 'dex', '1d6', '穿刺', ['靈巧', '輕型', '煩擾精通'])],
    resources: [resource('favored_enemy', '宿敵標記', 2, '不消耗法術位施放獵人印記。', 0)],
    features: [feature('deft_explorer', '靈巧探險家', '一項技能取得專精，並學會兩種語言。'), feature('fighting_style', '戰鬥風格：箭術', '遠程武器攻擊 +2。'), feature('hunters_lore', '獵人學識', '獵人印記目標的免疫、抗性與易傷會向你揭露。'), feature('colossus_slayer', '巨像殺手', '每回合一次，命中受傷目標時額外造成 1d8 傷害。')],
    spellcasting: {
      ability: 'wis', focus: '德魯伊法器', mode: 'standard', slots: [{ level: 1, max: 3 }],
      cantrips: [], prepared: ['cure_wounds', 'ensnaring_strike', 'goodberry', 'longstrider'],
      alwaysPrepared: ['hunters_mark'],
    },
  },
  '盜賊': {
    subclass: '竊賊', background: '罪犯', hitDie: 8,
    abilities: { str: 8, dex: 17, con: 14, int: 12, wis: 13, cha: 10 }, ac: 15, speed: 30,
    saves: ['dex', 'int'], proficientSkills: ['隱匿', '巧手', '調查', '察覺', '欺瞞', '雜技'], expertiseSkills: ['隱匿', '巧手'],
    equipment: ['鑲釘皮甲', '短劍', '短弓與 20 支箭', '匕首 ×2', '盜賊工具', '竊賊套組'],
    attacks: [attack('shortsword', '短劍', 'dex', '1d6', '穿刺', ['靈巧', '輕型', '煩擾精通']), attack('shortbow', '短弓', 'dex', '1d6', '穿刺', ['彈藥 80/320', '雙手'])],
    resources: [],
    features: [feature('sneak_attack', '偷襲 2d6', '每回合一次，符合優勢或盟友牽制條件的靈巧／遠程攻擊額外造成 2d6。'), feature('cunning_action', '靈巧動作', '附贈動作疾走、撤離或躲藏。'), feature('steady_aim', '穩定瞄準', '未移動時以附贈動作取得下一次攻擊優勢，之後速度變為 0。'), feature('fast_hands', '快手', '附贈動作進行巧手、使用物件或以盜賊工具開鎖與拆陷阱。'), feature('second_story_work', '飛簷走壁', '獲得等同速度的攀爬速度，跳躍可使用敏捷。')],
  },
  '術士': {
    subclass: '龍族術法', background: '隱士', hitDie: 6, maxHpBonus: 3,
    abilities: { str: 8, dex: 14, con: 15, int: 10, wis: 12, cha: 17 }, ac: 15, speed: 30,
    saves: ['con', 'cha'], proficientSkills: ['奧秘', '說服', '洞悉', '欺瞞'],
    equipment: ['矛', '匕首 ×2', '奧術法器', '地城探索套組'],
    attacks: [attack('dagger', '匕首', 'dex', '1d4', '穿刺', ['靈巧', '輕型', '投擲 20/60'])],
    resources: [resource('innate_sorcery', '內在術法', 2, '附贈動作啟動 1 分鐘；法術豁免 DC +1，法術攻擊具有優勢。', 0), resource('sorcery_points', '術法點', 3, '轉換法術位或使用精妙、延長、遙遠與雙生等超魔。', 0)],
    features: [feature('metamagic', '超魔', '選擇兩種超魔；預設為精妙法術與雙生法術。'), feature('draconic_resilience', '龍族韌性', '生命上限提高，未著甲時可使用龍族防禦。'), feature('draconic_spells', '龍族法術', '永遠準備與龍族血脈相關的額外法術。')],
    spellcasting: {
      ability: 'cha', focus: '奧術法器', mode: 'standard', slots: [{ level: 1, max: 4 }, { level: 2, max: 2 }],
      cantrips: ['light', 'prestidigitation', 'shocking_grasp', 'sorcerous_burst'],
      prepared: ['burning_hands', 'detect_magic', 'mage_armor', 'magic_missile', 'scorching_ray', 'suggestion'],
      alwaysPrepared: ['alter_self', 'chromatic_orb', 'command', 'dragons_breath'],
    },
  },
  '魔契師': {
    subclass: '邪魔宗主', background: '江湖騙子', hitDie: 8,
    abilities: { str: 8, dex: 14, con: 15, int: 10, wis: 12, cha: 17 }, ac: 13, speed: 30,
    saves: ['wis', 'cha'], proficientSkills: ['奧秘', '欺瞞', '調查', '威嚇'],
    equipment: ['皮甲', '鐮刀', '匕首 ×2', '奧術法器', '學者套組'],
    attacks: [attack('sickle', '鐮刀', 'dex', '1d4', '揮砍', ['輕型'])],
    resources: [resource('magical_cunning', '魔法巧思', 1, '進行 1 分鐘儀式，恢復最多一半的契約法術位。', 0)],
    features: [feature('eldritch_invocations', '魔能祈喚 ×3', '預設為苦痛魔爆、暗影護甲與契約魔寵。'), feature('dark_ones_blessing', '黑暗者賜福', '敵人死亡時可獲得魅力調整值 + 魔契師等級的暫時生命。')],
    spellcasting: {
      ability: 'cha', focus: '奧術法器', mode: 'pact', pactSlotLevel: 2, slots: [{ level: 2, max: 2 }],
      cantrips: ['eldritch_blast', 'prestidigitation'], prepared: ['armor_of_agathys', 'charm_person', 'hex', 'misty_step'],
      alwaysPrepared: ['burning_hands', 'command', 'scorching_ray', 'suggestion'],
    },
  },
  '法師': {
    subclass: '塑能師', background: '賢者', hitDie: 6,
    abilities: { str: 8, dex: 14, con: 15, int: 17, wis: 12, cha: 10 }, ac: 12, speed: 30,
    saves: ['int', 'wis'], proficientSkills: ['奧秘', '歷史', '調查', '洞悉'], expertiseSkills: ['奧秘'],
    equipment: ['長棍', '匕首', '奧術法器', '法術書', '學者套組'],
    attacks: [attack('quarterstaff', '長棍', 'str', '1d6', '鈍擊', ['多用途 1d8', '擊倒精通'])],
    resources: [resource('arcane_recovery', '奧術回復', 1, '短休後恢復合計至多 2 環的法術位；每次長休可用一次。', 0)],
    features: [feature('ritual_adept', '儀式專家', '法術書內具儀式標籤的法術無須準備即可進行儀式施放。'), feature('scholar', '學者：奧秘', '奧秘技能取得專精。'), feature('evocation_savant', '塑能學者', '額外將兩個不高於二環的塑能法術加入法術書。'), feature('potent_cantrip', '強效戲法', '傷害戲法未命中或目標豁免成功時，仍造成一半傷害但不附帶其他效果。')],
    spellcasting: {
      ability: 'int', focus: '奧術法器或法術書', mode: 'standard', slots: [{ level: 1, max: 4 }, { level: 2, max: 2 }],
      cantrips: ['light', 'mage_hand', 'ray_of_frost'],
      prepared: ['mage_armor', 'magic_missile', 'shield', 'sleep', 'misty_step', 'scorching_ray'],
      spellbook: ['detect_magic', 'feather_fall', 'mage_armor', 'magic_missile', 'shield', 'sleep', 'thunderwave', 'charm_person', 'misty_step', 'scorching_ray', 'burning_hands', 'shatter'],
    },
  },
};

export const classNames = Object.keys(classDefinitions) as ClassName[];

export function abilityModifier(score: number) {
  return Math.floor((score - 10) / 2);
}

function makeSkills(definition: ClassDefinition, proficiencyBonus: number): CharacterSkill[] {
  return Object.entries(skillAbilities).map(([name, ability]) => {
    const proficient = definition.proficientSkills.includes(name);
    const expertise = Boolean(definition.expertiseSkills?.includes(name));
    return {
      name,
      ability,
      proficient,
      expertise,
      bonus: abilityModifier(definition.abilities[ability]) + (expertise ? proficiencyBonus * 2 : proficient ? proficiencyBonus : 0),
    };
  });
}

function makeSpells(template: SpellcastingTemplate): CharacterSpell[] {
  const cantrips = template.cantrips.map((id) => makeSpell(id));
  const prepared = template.prepared.map((id) => makeSpell(id, { prepared: true, inSpellbook: Boolean(template.spellbook?.includes(id)) }));
  const alwaysPrepared = (template.alwaysPrepared || []).map((id) => makeSpell(id, {
    prepared: true,
    alwaysPrepared: true,
    freeUseResourceId: id === 'divine_smite' ? 'free_divine_smite' : undefined,
  }));
  const preparedIds = new Set([...template.cantrips, ...template.prepared, ...(template.alwaysPrepared || [])]);
  const bookOnly = (template.spellbook || [])
    .filter((id) => !preparedIds.has(id))
    .map((id) => makeSpell(id, { prepared: false, inSpellbook: true }));
  return [...cantrips, ...prepared, ...alwaysPrepared, ...bookOnly];
}

function makeInitials(name: string) {
  const latinParts = name.trim().split(/[\s・]+/).filter(Boolean);
  if (latinParts.length > 1 && latinParts.every((part) => /^[a-z]/i.test(part))) {
    return latinParts.slice(0, 2).map((part) => part[0].toUpperCase()).join('');
  }
  return [...name.trim().replace(/[\s・]/g, '')].slice(0, 2).join('').toUpperCase() || 'AD';
}

export function createLevel3Character(id: PlayerId, name: string, className: string): PlayerCharacter {
  const requestedClass = className as ClassName;
  const definition = classDefinitions[requestedClass] || classDefinitions['戰士'];
  const resolvedClassName = classDefinitions[requestedClass] ? requestedClass : '戰士';
  const proficiencyBonus = 2;
  const conModifier = abilityModifier(definition.abilities.con);
  const averageHitDie = Math.floor(definition.hitDie / 2) + 1;
  const maxHp = definition.hitDie + conModifier + (averageHitDie + conModifier) * 2 + (definition.maxHpBonus || 0);
  const skills = makeSkills(definition, proficiencyBonus);
  const perception = skills.find((skill) => skill.name === '察覺')?.bonus || abilityModifier(definition.abilities.wis);
  const resources = definition.resources.map((entry) => {
    const max = typeof entry.max === 'number' ? entry.max : Math.max(1, abilityModifier(definition.abilities[entry.max]));
    return { ...entry, max, current: max };
  });
  const attacks = definition.attacks.map((entry) => {
    const modifier = abilityModifier(definition.abilities[entry.ability]);
    return {
      id: entry.id,
      name: entry.name,
      attackBonus: modifier + proficiencyBonus + (resolvedClassName === '遊俠' && entry.id === 'longbow' ? 2 : 0),
      damage: `${entry.damageDie}${modifier >= 0 ? '+' : ''}${modifier}`,
      damageType: entry.damageType,
      properties: entry.properties,
    };
  });
  const spellcasting = definition.spellcasting
    ? {
        ability: definition.spellcasting.ability,
        attackBonus: proficiencyBonus + abilityModifier(definition.abilities[definition.spellcasting.ability]),
        saveDc: 8 + proficiencyBonus + abilityModifier(definition.abilities[definition.spellcasting.ability]),
        focus: definition.spellcasting.focus,
        mode: definition.spellcasting.mode,
        pactSlotLevel: definition.spellcasting.pactSlotLevel,
        slots: definition.spellcasting.slots.map((slot) => ({ ...slot, current: slot.max })),
        spells: makeSpells(definition.spellcasting),
      }
    : undefined;

  return {
    id,
    name: name.trim(),
    className: resolvedClassName,
    subclass: definition.subclass,
    species: '人類',
    background: definition.background,
    level: 3,
    initials: makeInitials(name),
    hp: maxHp,
    maxHp,
    ac: definition.ac,
    passive: 10 + perception,
    speed: definition.speed,
    initiative: abilityModifier(definition.abilities.dex),
    proficiencyBonus,
    hitDie: definition.hitDie,
    hitDice: 3,
    maxHitDice: 3,
    abilities: { ...definition.abilities },
    savingThrowProficiencies: [...definition.saves],
    skills,
    attacks,
    equipment: [...definition.equipment],
    resources,
    features: definition.features.map((entry) => ({ ...entry })),
    spellcasting,
    condition: '正常',
    experience: 900,
    abilityPoints: 0,
  };
}

export function changeResource(character: PlayerCharacter, resourceId: string, delta: number): PlayerCharacter {
  return {
    ...character,
    resources: character.resources.map((entry) => entry.id === resourceId
      ? { ...entry, current: Math.max(0, Math.min(entry.max, entry.current + delta)) }
      : entry),
  };
}

export function restCharacter(character: PlayerCharacter, type: RestType): PlayerCharacter {
  const resources = character.resources.map((entry) => {
    if (type === 'long') return { ...entry, current: entry.max };
    const recovered = entry.shortRestRecovery === 'all' ? entry.max : entry.current + entry.shortRestRecovery;
    return { ...entry, current: Math.min(entry.max, recovered) };
  });
  const spellcasting = character.spellcasting
    ? {
        ...character.spellcasting,
        slots: character.spellcasting.slots.map((slot) => ({
          ...slot,
          current: type === 'long' || character.spellcasting?.mode === 'pact' ? slot.max : slot.current,
        })),
      }
    : undefined;
  return {
    ...character,
    hp: type === 'long' ? character.maxHp : character.hp,
    hitDice: type === 'long' ? character.maxHitDice : character.hitDice,
    concentration: type === 'long' ? undefined : character.concentration,
    condition: type === 'long' ? '正常' : character.condition,
    resources,
    spellcasting,
  };
}

export function spendSpellSlot(character: PlayerCharacter, spell: CharacterSpell, asRitual = false): PlayerCharacter | null {
  if (spell.level === 0 || (asRitual && spell.ritual)) {
    return spell.concentration ? { ...character, concentration: spell.name } : character;
  }
  if (!character.spellcasting) return null;
  if (spell.freeUseResourceId) {
    const freeUse = character.resources.find((entry) => entry.id === spell.freeUseResourceId && entry.current > 0);
    if (freeUse) {
      return {
        ...character,
        concentration: spell.concentration ? spell.name : character.concentration,
        resources: character.resources.map((entry) => entry.id === spell.freeUseResourceId ? { ...entry, current: entry.current - 1 } : entry),
      };
    }
  }
  const slotIndex = character.spellcasting.slots.findIndex((slot) => slot.level >= spell.level && slot.current > 0);
  if (slotIndex < 0) return null;
  const slots = character.spellcasting.slots.map((slot, index) => index === slotIndex
    ? { ...slot, current: slot.current - 1 }
    : slot);
  return {
    ...character,
    concentration: spell.concentration ? spell.name : character.concentration,
    spellcasting: { ...character.spellcasting, slots },
  };
}

export function getCheckBonus(character: PlayerCharacter, check: string): number {
  if (check === '先攻') return character.initiative;
  const skill = character.skills.find((entry) => entry.name === check);
  if (skill) return skill.bonus;
  const ability = (Object.entries(abilityLabels).find(([, label]) => label === check)?.[0] || '') as AbilityKey;
  return ability ? abilityModifier(character.abilities[ability]) : 0;
}
