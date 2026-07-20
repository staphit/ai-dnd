package rules

// Ported 1:1 from frontend/src/rules/spells.ts: the shared spell catalog
// (Chinese-localized SRD spells with structured effects) plus the makeSpell
// factory used by character templates and advancement.

import "fmt"

// SpellDefinition mirrors the spells.ts SpellDefinition type:
// Omit<CharacterSpell, 'prepared' | 'alwaysPrepared' | 'inSpellbook'>.
type SpellDefinition struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	EnglishName   string       `json:"englishName"`
	Level         int          `json:"level"`
	School        string       `json:"school"`
	CastingTime   string       `json:"castingTime"`
	Range         string       `json:"range"`
	Duration      string       `json:"duration"`
	Description   string       `json:"description"`
	Concentration bool         `json:"concentration"`
	Ritual        bool         `json:"ritual"`
	Effect        *SpellEffect `json:"effect,omitempty"`
}

// spellOpts mirrors the options bag of the spells.ts `spell` helper.
type spellOpts struct {
	concentration bool
	ritual        bool
	effect        *SpellEffect
}

// def mirrors the spells.ts `spell` constructor helper.
func def(id, name, englishName string, level int, school, castingTime, spellRange, duration, description string, opts spellOpts) SpellDefinition {
	return SpellDefinition{
		ID:            id,
		Name:          name,
		EnglishName:   englishName,
		Level:         level,
		School:        school,
		CastingTime:   castingTime,
		Range:         spellRange,
		Duration:      duration,
		Description:   description,
		Concentration: opts.concentration,
		Ritual:        opts.ritual,
		Effect:        opts.effect,
	}
}

// spellDefinitions lists every catalog entry in the exact declaration order of
// the spells.ts spellCatalog object literal. In TS, Object.keys(spellCatalog)
// returns this insertion order, and setPreparedSpells (advancement.ts) depends
// on it — so this order is load-bearing; do not sort or reorder.
var spellDefinitions = []SpellDefinition{
	def("dancing_lights", "舞光術", "Dancing Lights", 0, "幻術", "動作", "120 呎", "專注，1 分鐘", "創造最多四個可移動的微光，照亮黑暗區域。", spellOpts{concentration: true}),
	def("vicious_mockery", "惡毒嘲弄", "Vicious Mockery", 0, "惑控", "動作", "60 呎", "立即", "目標進行感知豁免；失敗受到 1d6 心靈傷害，下一次攻擊具有劣勢。", spellOpts{effect: &SpellEffect{Kind: "damage", Target: "creature", Dice: "1d6", SaveAbility: "wis", DamageType: "心靈"}}),
	def("guidance", "神導術", "Guidance", 0, "預言", "動作", "觸碰", "專注，1 分鐘", "目標選擇一項技能，在法術結束前的一次相關檢定增加 1d4。", spellOpts{concentration: true}),
	def("sacred_flame", "聖火術", "Sacred Flame", 0, "塑能", "動作", "60 呎", "立即", "目標進行敏捷豁免；失敗受到 1d8 光耀傷害，掩護不提供此豁免加值。", spellOpts{effect: &SpellEffect{Kind: "damage", Target: "creature", Dice: "1d8", SaveAbility: "dex", DamageType: "光耀"}}),
	def("thaumaturgy", "奇術", "Thaumaturgy", 0, "變化", "動作", "30 呎", "最長 1 分鐘", "產生微小神蹟，例如放大聲音、改變火焰或開關未上鎖的門窗。", spellOpts{}),
	def("druidcraft", "德魯伊伎倆", "Druidcraft", 0, "變化", "動作", "30 呎", "立即", "產生微小自然效果、預測天氣或點燃與熄滅小型火焰。", spellOpts{}),
	def("produce_flame", "燃火術", "Produce Flame", 0, "咒法", "附贈動作", "自身", "10 分鐘", "在手中產生火焰照明，或投向 60 呎內目標造成 1d8 火焰傷害。", spellOpts{effect: &SpellEffect{Kind: "damage", Target: "creature", Dice: "1d8", AttackRoll: true, DamageType: "火焰"}}),
	def("light", "光亮術", "Light", 0, "塑能", "動作", "觸碰", "1 小時", "使一件物體發出明亮與微光，可覆蓋敵對生物攜帶的物體。", spellOpts{}),
	def("prestidigitation", "魔法伎倆", "Prestidigitation", 0, "變化", "動作", "10 呎", "最長 1 小時", "製造微小魔法效果，例如清潔、標記、點火、氣味或無害感官效果。", spellOpts{}),
	def("shocking_grasp", "電爪", "Shocking Grasp", 0, "塑能", "動作", "觸碰", "立即", "近戰法術攻擊造成 1d8 閃電傷害，命中後目標無法進行藉機攻擊。", spellOpts{effect: &SpellEffect{Kind: "damage", Target: "creature", Dice: "1d8", AttackRoll: true, DamageType: "閃電"}}),
	def("sorcerous_burst", "術法爆發", "Sorcerous Burst", 0, "塑能", "動作", "120 呎", "立即", "遠程法術攻擊造成 1d8 自選元素傷害；擲出 8 時可追加傷害骰。", spellOpts{}),
	def("eldritch_blast", "魔能爆", "Eldritch Blast", 0, "塑能", "動作", "120 呎", "立即", "遠程法術攻擊命中造成 1d10 力場傷害。", spellOpts{effect: &SpellEffect{Kind: "damage", Target: "creature", Dice: "1d10", AttackRoll: true, DamageType: "力場"}}),
	def("mage_hand", "法師之手", "Mage Hand", 0, "咒法", "動作", "30 呎", "1 分鐘", "創造一隻幽靈手，可操作物件、開門或搬運最多 10 磅。", spellOpts{}),
	def("ray_of_frost", "冷凍射線", "Ray of Frost", 0, "塑能", "動作", "60 呎", "立即", "遠程法術攻擊造成 1d8 冷凍傷害，並使目標速度減少 10 呎直到下回合開始。", spellOpts{effect: &SpellEffect{Kind: "damage", Target: "creature", Dice: "1d8", AttackRoll: true, DamageType: "冷凍"}}),

	def("animal_friendship", "化獸為友", "Animal Friendship", 1, "惑控", "動作", "30 呎", "24 小時", "野獸進行感知豁免；失敗時在法術期間陷入魅惑。", spellOpts{}),
	def("armor_of_agathys", "阿嘉西斯之鎧", "Armor of Agathys", 1, "防護", "附贈動作", "自身", "1 小時", "獲得 5 點暫時生命；暫時生命仍在時，近戰命中你的生物受到 5 點冷凍傷害。", spellOpts{effect: &SpellEffect{Kind: "temporaryHp", Target: "self", Flat: 5}}),
	def("bless", "祝福術", "Bless", 1, "惑控", "動作", "30 呎", "專注，1 分鐘", "最多三個目標的攻擊與豁免各增加 1d4。", spellOpts{concentration: true}),
	def("burning_hands", "燃燒之手", "Burning Hands", 1, "塑能", "動作", "15 呎錐形", "立即", "範圍內生物進行敏捷豁免，失敗受到 3d6 火焰傷害，成功減半。", spellOpts{}),
	def("charm_person", "魅惑人類", "Charm Person", 1, "惑控", "動作", "30 呎", "1 小時", "一名類人生物進行感知豁免；失敗時被你魅惑。", spellOpts{}),
	def("chromatic_orb", "繁彩球", "Chromatic Orb", 1, "塑能", "動作", "90 呎", "立即", "遠程法術攻擊造成 3d8 自選元素傷害；相同骰值可使能量彈向另一目標。", spellOpts{}),
	def("command", "命令術", "Command", 1, "惑控", "動作", "60 呎", "1 輪", "說出一字命令；目標感知豁免失敗便在下回合遵從。", spellOpts{}),
	def("cure_wounds", "治療傷勢", "Cure Wounds", 1, "防護", "動作", "觸碰", "立即", "目標恢復 2d8 加施法屬性調整值的生命。", spellOpts{effect: &SpellEffect{Kind: "healing", Target: "ally", Dice: "2d8", AddAbilityModifier: true}}),
	def("detect_magic", "偵測魔法", "Detect Magic", 1, "預言", "動作或儀式", "自身", "專注，10 分鐘", "感知 30 呎內魔法並辨識可見魔法靈光的學派。", spellOpts{concentration: true, ritual: true}),
	def("dissonant_whispers", "失諧低語", "Dissonant Whispers", 1, "惑控", "動作", "60 呎", "立即", "感知豁免失敗受到 3d6 心靈傷害，並立即使用反應遠離你。", spellOpts{}),
	def("divine_favor", "神恩", "Divine Favor", 1, "變化", "附贈動作", "自身", "1 分鐘", "你的武器攻擊額外造成 1d4 光耀傷害。", spellOpts{}),
	def("divine_smite", "至聖斬", "Divine Smite", 1, "塑能", "命中後附贈動作", "自身", "立即", "近戰武器或徒手命中後額外造成 2d8 光耀傷害，對邪魔與不死生物再增加 1d8。", spellOpts{}),
	def("ensnaring_strike", "誘捕打擊", "Ensnaring Strike", 1, "咒法", "附贈動作", "自身", "專注，1 分鐘", "下一次武器命中可能以荊棘束縛目標並持續造成傷害。", spellOpts{concentration: true}),
	def("entangle", "糾纏術", "Entangle", 1, "咒法", "動作", "90 呎", "專注，1 分鐘", "20 呎方形區域長出纏繞植物；力量豁免失敗的生物受到束縛。", spellOpts{concentration: true}),
	def("faerie_fire", "妖火術", "Faerie Fire", 1, "塑能", "動作", "60 呎", "專注，1 分鐘", "20 呎立方內生物敏捷豁免失敗便顯形，對其攻擊具有優勢。", spellOpts{concentration: true}),
	def("feather_fall", "羽落術", "Feather Fall", 1, "變化", "反應", "60 呎", "1 分鐘", "最多五個墜落生物的下降速度降至每輪 60 呎並安全著地。", spellOpts{}),
	def("goodberry", "神莓術", "Goodberry", 1, "咒法", "動作", "自身", "24 小時", "創造十顆莓果；每顆恢復 1 生命並提供一天營養。", spellOpts{}),
	def("guiding_bolt", "曳光彈", "Guiding Bolt", 1, "塑能", "動作", "120 呎", "1 輪", "遠程法術攻擊造成 4d6 光耀傷害；下一次攻擊該目標具有優勢。", spellOpts{effect: &SpellEffect{Kind: "damage", Target: "creature", Dice: "4d6", AttackRoll: true, DamageType: "光耀"}}),
	def("healing_word", "治癒真言", "Healing Word", 1, "防護", "附贈動作", "60 呎", "立即", "可見生物恢復 2d4 加施法屬性調整值的生命。", spellOpts{effect: &SpellEffect{Kind: "healing", Target: "ally", Dice: "2d4", AddAbilityModifier: true}}),
	def("hex", "脆弱詛咒", "Hex", 1, "惑控", "附贈動作", "90 呎", "專注，1 小時", "你命中目標時額外造成 1d6 黯蝕傷害，並使其一類能力檢定具有劣勢。", spellOpts{concentration: true}),
	def("hunters_mark", "獵人印記", "Hunter's Mark", 1, "預言", "附贈動作", "90 呎", "專注，1 小時", "武器命中被標記目標時額外造成 1d6 力場傷害，並有利於追蹤。", spellOpts{concentration: true}),
	def("longstrider", "大步奔行", "Longstrider", 1, "變化", "動作", "觸碰", "1 小時", "目標速度增加 10 呎。", spellOpts{}),
	def("mage_armor", "法師護甲", "Mage Armor", 1, "防護", "動作", "觸碰", "8 小時", "未著甲目標的基礎 AC 變為 13 加敏捷調整值。", spellOpts{}),
	def("magic_missile", "魔法飛彈", "Magic Missile", 1, "塑能", "動作", "120 呎", "立即", "產生三枚必中的力場飛彈，每枚造成 1d4+1 力場傷害。", spellOpts{effect: &SpellEffect{Kind: "damage", Target: "creature", Dice: "3d4+3", AutomaticHit: true, DamageType: "力場"}}),
	def("protection_evil_good", "防護善惡", "Protection from Evil and Good", 1, "防護", "動作", "觸碰", "專注，10 分鐘", "保護目標免受特定異界生物攻擊、魅惑、恐慌與附身。", spellOpts{concentration: true}),
	def("sanctuary", "聖域術", "Sanctuary", 1, "防護", "附贈動作", "30 呎", "1 分鐘", "攻擊受保護目標的生物須先通過感知豁免，否則必須改選目標。", spellOpts{}),
	def("shield", "護盾術", "Shield", 1, "防護", "反應", "自身", "1 輪", "直到下回合開始 AC +5，並免疫魔法飛彈。", spellOpts{}),
	def("shield_of_faith", "虔誠護盾", "Shield of Faith", 1, "防護", "附贈動作", "60 呎", "專注，10 分鐘", "一個可見生物的 AC +2。", spellOpts{concentration: true}),
	def("sleep", "睡眠術", "Sleep", 1, "惑控", "動作", "60 呎", "專注，1 分鐘", "範圍內生物感知豁免失敗便陷入失能，第二次失敗後沉睡。", spellOpts{concentration: true}),
	def("speak_with_animals", "動物交談", "Speak with Animals", 1, "預言", "動作或儀式", "自身", "10 分鐘", "理解野獸並以言語與其溝通。", spellOpts{ritual: true}),
	def("thunderwave", "雷鳴波", "Thunderwave", 1, "塑能", "動作", "15 呎立方", "立即", "體質豁免失敗受到 2d8 雷鳴傷害並被推開 10 呎，成功傷害減半。", spellOpts{}),

	def("aid", "援助術", "Aid", 2, "防護", "動作", "30 呎", "8 小時", "最多三個生物的目前與生命上限各增加 5。", spellOpts{}),
	def("alter_self", "變身術", "Alter Self", 2, "變化", "動作", "自身", "專注，1 小時", "改變外貌、適應水域，或長出可作為魔法武器的自然武器。", spellOpts{concentration: true}),
	def("dragons_breath", "龍息術", "Dragon's Breath", 2, "變化", "附贈動作", "觸碰", "專注，1 分鐘", "目標可用魔法動作呼出 15 呎錐形元素吐息，造成 3d6 傷害。", spellOpts{concentration: true}),
	def("hold_person", "人類定身術", "Hold Person", 2, "惑控", "動作", "60 呎", "專注，1 分鐘", "類人生物感知豁免失敗便陷入麻痺，每回合可再豁免。", spellOpts{concentration: true}),
	def("invisibility", "隱形術", "Invisibility", 2, "幻術", "動作", "觸碰", "專注，1 小時", "目標與攜帶物變為隱形；攻擊、造成傷害或施法後結束。", spellOpts{concentration: true}),
	def("lesser_restoration", "次級復原術", "Lesser Restoration", 2, "防護", "附贈動作", "觸碰", "立即", "結束目標的目盲、耳聾、麻痺或中毒之一。", spellOpts{effect: &SpellEffect{Kind: "condition", Target: "ally", Condition: "正常"}}),
	def("misty_step", "迷蹤步", "Misty Step", 2, "咒法", "附贈動作", "自身", "立即", "傳送至 30 呎內一處可見且未被占據的位置。", spellOpts{}),
	def("moonbeam", "月華之光", "Moonbeam", 2, "塑能", "動作", "120 呎", "專注，1 分鐘", "創造可移動的月光柱，進入或開始回合時進行體質豁免並承受光耀傷害。", spellOpts{concentration: true}),
	def("scorching_ray", "灼熱射線", "Scorching Ray", 2, "塑能", "動作", "120 呎", "立即", "射出三道射線，各自進行遠程法術攻擊並造成 2d6 火焰傷害。", spellOpts{}),
	def("shatter", "粉碎音波", "Shatter", 2, "塑能", "動作", "60 呎", "立即", "10 呎球形內生物進行體質豁免，失敗受到 3d8 雷鳴傷害，成功減半。", spellOpts{}),
	def("silence", "沉默術", "Silence", 2, "幻術", "動作或儀式", "120 呎", "專注，10 分鐘", "20 呎球形內無法產生或傳播聲音，也無法施放含聲音成分的法術。", spellOpts{concentration: true, ritual: true}),
	def("spiritual_weapon", "靈體武器", "Spiritual Weapon", 2, "塑能", "附贈動作", "60 呎", "專注，1 分鐘", "創造浮空武器，以近戰法術攻擊造成力場傷害並可持續移動攻擊。", spellOpts{concentration: true}),
	def("suggestion", "暗示術", "Suggestion", 2, "惑控", "動作", "30 呎", "專注，8 小時", "感知豁免失敗的目標會遵循一項聽起來合理的行動建議。", spellOpts{concentration: true}),
}

// SpellCatalog mirrors spells.ts spellCatalog. Go map iteration order is
// random — wherever the TS code relies on Object.keys(spellCatalog) insertion
// order (e.g. advancement.ts setPreparedSpells), iterate SpellIDs instead.
var SpellCatalog = func() map[string]SpellDefinition {
	catalog := make(map[string]SpellDefinition, len(spellDefinitions))
	for _, definition := range spellDefinitions {
		catalog[definition.ID] = definition
	}
	return catalog
}()

// SpellIDs is Object.keys(spellCatalog) in the TS object-literal declaration
// order (string keys preserve insertion order in JS).
var SpellIDs = func() []string {
	ids := make([]string, len(spellDefinitions))
	for i, definition := range spellDefinitions {
		ids[i] = definition.ID
	}
	return ids
}()

// SpellOverrides mirrors the makeSpell options parameter in spells.ts:
// Partial<Pick<CharacterSpell, 'prepared' | 'alwaysPrepared' | 'inSpellbook' | 'freeUseResourceId'>>.
type SpellOverrides struct {
	// Prepared mirrors `options.prepared ?? true`: nil (unspecified) defaults
	// to true; point at false to explicitly mark the spell unprepared.
	Prepared *bool
	// AlwaysPrepared mirrors `options.alwaysPrepared ?? false` (the Go zero
	// value equals the TS nullish default).
	AlwaysPrepared bool
	// InSpellbook mirrors `options.inSpellbook ?? false`.
	InSpellbook bool
	// FreeUseResourceID mirrors options.freeUseResourceId; "" means unset.
	FreeUseResourceID string
}

// MakeSpell mirrors spells.ts makeSpell: it instantiates a character-owned
// Spell from a catalog definition, applying the preparation overrides. The TS
// version throws on an unknown id, so this returns an error with the exact
// same message.
func MakeSpell(id string, overrides SpellOverrides) (Spell, error) {
	definition, ok := SpellCatalog[id]
	if !ok {
		return Spell{}, fmt.Errorf("Unknown spell: %s", id)
	}
	prepared := true
	if overrides.Prepared != nil {
		prepared = *overrides.Prepared
	}
	return Spell{
		ID:                definition.ID,
		Name:              definition.Name,
		EnglishName:       definition.EnglishName,
		Level:             definition.Level,
		School:            definition.School,
		CastingTime:       definition.CastingTime,
		Range:             definition.Range,
		Duration:          definition.Duration,
		Description:       definition.Description,
		Concentration:     definition.Concentration,
		Ritual:            definition.Ritual,
		Prepared:          prepared,
		AlwaysPrepared:    overrides.AlwaysPrepared,
		InSpellbook:       overrides.InSpellbook,
		FreeUseResourceID: overrides.FreeUseResourceID,
		Effect:            definition.Effect,
	}, nil
}
