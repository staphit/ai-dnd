package rules

// Ported from frontend/src/rules/characters.ts and then trimmed to the six
// playable level-3 class templates (2024 rules) whose features are all backed
// by server mechanics, plus character creation, resource/rest management,
// spell slot spending, and skill/ability check bonus lookup.

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// AbilityLabels mirrors characters.ts abilityLabels. Where the TS code relies
// on Object.entries(abilityLabels) insertion order, iterate AbilityKeys.
var AbilityLabels = map[AbilityKey]string{
	"str": "力量", "dex": "敏捷", "con": "體質", "int": "智力", "wis": "感知", "cha": "魅力",
}

// skillAbility is one entry of the characters.ts skillAbilities record.
type skillAbility struct {
	name    string
	ability AbilityKey
}

// skillAbilities mirrors characters.ts skillAbilities. TS iterates the record
// with Object.entries, whose string-key insertion order defines the 18-entry
// skill sheet order — so this is an ordered slice, not a map.
var skillAbilities = []skillAbility{
	{"運動", "str"},
	{"雜技", "dex"}, {"巧手", "dex"}, {"隱匿", "dex"},
	{"奧秘", "int"}, {"歷史", "int"}, {"調查", "int"}, {"自然", "int"}, {"宗教", "int"},
	{"馴獸", "wis"}, {"洞悉", "wis"}, {"醫藥", "wis"}, {"察覺", "wis"}, {"求生", "wis"},
	{"欺瞞", "cha"}, {"威嚇", "cha"}, {"表演", "cha"}, {"說服", "cha"},
}

// ResourceTemplate mirrors the characters.ts ResourceTemplate type:
// Omit<CharacterResource, 'current' | 'max'> & { max: number | AbilityKey }.
// The TS max union is split into Max/MaxAbility: when MaxAbility is non-empty
// it wins, and the actual maximum becomes max(1, abilityModifier(score)).
type ResourceTemplate struct {
	ID                string
	Name              string
	Max               int
	MaxAbility        AbilityKey
	Die               string
	Description       string
	ShortRestRecovery ShortRestRecovery
}

// AttackTemplate mirrors the characters.ts AttackTemplate type:
// Omit<CharacterAttack, 'attackBonus' | 'damage'> & { ability; damageDie }.
type AttackTemplate struct {
	ID         string
	Name       string
	Ability    AbilityKey
	DamageDie  string
	DamageType string
	Properties []string
}

// SlotTemplate mirrors the slots entries of the TS SpellcastingTemplate.
type SlotTemplate struct {
	Level int
	Max   int
}

// SpellcastingTemplate mirrors the characters.ts SpellcastingTemplate interface.
type SpellcastingTemplate struct {
	Ability        AbilityKey
	Focus          string
	Mode           string // standard | pact
	PactSlotLevel  int
	Slots          []SlotTemplate
	Cantrips       []string
	Prepared       []string
	AlwaysPrepared []string
	Spellbook      []string
}

// ClassDefinition mirrors the characters.ts ClassDefinition interface.
type ClassDefinition struct {
	Subclass         string
	Background       string
	HitDie           int
	MaxHPBonus       int
	Abilities        AbilityScores
	AC               int
	Speed            int
	Saves            []AbilityKey
	ProficientSkills []string
	ExpertiseSkills  []string
	Equipment        []string
	Attacks          []AttackTemplate
	Resources        []ResourceTemplate
	Features         []Feature
	Spellcasting     *SpellcastingTemplate
}

// feature mirrors the characters.ts `feature` helper.
func feature(id, name, description string) Feature {
	return Feature{ID: id, Name: name, Description: description}
}

// resource mirrors the number-max branch of the characters.ts `resource`
// helper (max: number). See abilityResource for the AbilityKey branch.
func resource(id, name string, max int, description string, shortRestRecovery ShortRestRecovery, die string) ResourceTemplate {
	return ResourceTemplate{ID: id, Name: name, Max: max, Description: description, ShortRestRecovery: shortRestRecovery, Die: die}
}

// abilityResource mirrors the AbilityKey-max branch of the characters.ts
// `resource` helper (max: AbilityKey).
func abilityResource(id, name string, maxAbility AbilityKey, description string, shortRestRecovery ShortRestRecovery, die string) ResourceTemplate {
	return ResourceTemplate{ID: id, Name: name, MaxAbility: maxAbility, Description: description, ShortRestRecovery: shortRestRecovery, Die: die}
}

// attack mirrors the characters.ts `attack` helper.
func attack(id, name string, ability AbilityKey, damageDie, damageType string, properties ...string) AttackTemplate {
	return AttackTemplate{ID: id, Name: name, Ability: ability, DamageDie: damageDie, DamageType: damageType, Properties: properties}
}

// recoverAll is the 'all' arm of the TS shortRestRecovery union; recoverNone
// is the `= 0` default.
var (
	recoverAll  = ShortRestRecovery{All: true}
	recoverNone = ShortRestRecovery{}
)

// classEntry pairs a class name with its definition so the TS object-literal
// declaration order survives the port (Go maps do not preserve order).
type classEntry struct {
	name       string
	definition ClassDefinition
}

// classEntries mirrors the characters.ts classDefinitions declaration order,
// trimmed to the six playable classes (吟遊詩人 → 法師). ClassNames and
// ClassDefinitions derive from it. Every listed feature and resource is
// backed by a real server mechanic — keep descriptions in sync with the code.
var classEntries = []classEntry{
	{"吟遊詩人", ClassDefinition{
		Subclass: "逸聞學院", Background: "藝人", HitDie: 8,
		Abilities: AbilityScores{Str: 8, Dex: 14, Con: 14, Int: 10, Wis: 12, Cha: 17}, AC: 14, Speed: 30,
		Saves: []AbilityKey{"dex", "cha"}, ProficientSkills: []string{"表演", "說服", "洞悉", "巧手", "奧秘", "歷史"}, ExpertiseSkills: []string{"表演", "說服"},
		Equipment: []string{"鑲釘皮甲", "細劍", "短弓與 20 支箭", "魯特琴", "藝人套組"},
		Attacks:   []AttackTemplate{attack("rapier", "細劍", "dex", "1d8", "穿刺", "靈巧", "煩擾精通"), attack("shortbow", "短弓", "dex", "1d6", "穿刺", "彈藥 80/320", "雙手")},
		Resources: []ResourceTemplate{abilityResource("bardic_inspiration", "吟遊激勵", "cha", "消耗 1 次：下一次任何隊員的必要檢定由系統額外加骰 1d6。", recoverNone, "d6")},
		Features:  []Feature{feature("jack_of_all_trades", "萬事通", "未熟練的技能與能力檢定仍加入一半熟練加值（系統自動計算）。"), feature("expertise", "專精", "表演與說服使用雙倍熟練加值。")},
		Spellcasting: &SpellcastingTemplate{
			Ability: "cha", Focus: "樂器", Mode: "standard", Slots: []SlotTemplate{{Level: 1, Max: 4}, {Level: 2, Max: 2}},
			Cantrips: []string{"dancing_lights", "vicious_mockery"},
			Prepared: []string{"charm_person", "dissonant_whispers", "healing_word", "thunderwave", "invisibility", "suggestion"},
		},
	}},
	{"牧師", ClassDefinition{
		Subclass: "生命領域", Background: "侍僧", HitDie: 8,
		Abilities: AbilityScores{Str: 14, Dex: 10, Con: 15, Int: 8, Wis: 17, Cha: 12}, AC: 18, Speed: 30,
		Saves: []AbilityKey{"wis", "cha"}, ProficientSkills: []string{"洞悉", "宗教", "醫藥", "說服"},
		Equipment: []string{"鏈甲", "盾牌", "戰鎚", "聖徽", "祭司套組"},
		Attacks:   []AttackTemplate{attack("warhammer", "戰鎚", "str", "1d8", "鈍擊", "多用途 1d10", "推離精通")},
		Resources: []ResourceTemplate{resource("channel_divinity", "引導神力", 2, "保存生機：分配共 5 × 牧師等級的治療給生命低於一半的隊員，優先治療傷勢最重者，且不會治療超過一半生命上限。", ShortRestRecovery{Amount: 1}, "")},
		Features:  []Feature{feature("disciple_of_life", "生命門徒", "以法術位施放治療法術時，額外恢復 2 + 法術位環級生命。")},
		Spellcasting: &SpellcastingTemplate{
			Ability: "wis", Focus: "聖徽", Mode: "standard", Slots: []SlotTemplate{{Level: 1, Max: 4}, {Level: 2, Max: 2}},
			Cantrips:       []string{"guidance", "sacred_flame", "thaumaturgy"},
			Prepared:       []string{"command", "guiding_bolt", "healing_word", "shield_of_faith", "hold_person", "silence"},
			AlwaysPrepared: []string{"aid", "bless", "cure_wounds", "lesser_restoration"},
		},
	}},
	{"戰士", ClassDefinition{
		Subclass: "勇士", Background: "士兵", HitDie: 10,
		Abilities: AbilityScores{Str: 17, Dex: 12, Con: 16, Int: 10, Wis: 13, Cha: 8}, AC: 19, Speed: 30,
		Saves: []AbilityKey{"str", "con"}, ProficientSkills: []string{"運動", "察覺", "威嚇", "求生"},
		Equipment: []string{"鏈甲", "盾牌", "長劍", "輕弩與 20 支弩矢", "地城探索套組"},
		Attacks:   []AttackTemplate{attack("longsword", "長劍", "str", "1d8", "揮砍", "多用途 1d10", "削弱精通"), attack("light_crossbow", "輕弩", "dex", "1d8", "穿刺", "彈藥 80/320", "裝填", "緩速精通")},
		Resources: []ResourceTemplate{resource("second_wind", "回氣", 2, "附贈動作恢復 1d10 + 3 生命。", ShortRestRecovery{Amount: 1}, ""), resource("action_surge", "動作如潮", 1, "在自己的回合額外進行一個動作，但不能是魔法動作。", recoverAll, "")},
		Features:  []Feature{feature("fighting_style", "戰鬥風格：防禦", "穿著護甲時 AC +1，已計入AC。"), feature("improved_critical", "精通重擊", "攻擊骰出 19–20 即造成重擊。")},
	}},
	{"聖武士", ClassDefinition{
		Subclass: "奉獻之誓", Background: "貴族", HitDie: 10,
		Abilities: AbilityScores{Str: 17, Dex: 10, Con: 14, Int: 8, Wis: 12, Cha: 16}, AC: 18, Speed: 30,
		Saves: []AbilityKey{"wis", "cha"}, ProficientSkills: []string{"運動", "說服", "洞悉", "宗教"},
		Equipment: []string{"鏈甲", "盾牌", "長劍", "標槍 ×6", "聖徽", "祭司套組"},
		Attacks:   []AttackTemplate{attack("longsword", "長劍", "str", "1d8", "揮砍", "多用途 1d10", "削弱精通"), attack("javelin", "標槍", "str", "1d6", "穿刺", "投擲 30/120", "緩速精通")},
		Resources: []ResourceTemplate{resource("lay_on_hands", "聖療", 15, "花費點數治療隊伍中傷勢最重的成員，每點恢復 1 生命（由系統結算）。", recoverNone, ""), resource("free_divine_smite", "免費至聖斬", 1, "每次長休可不消耗法術位施放一次至聖斬。", recoverNone, "")},
		Features:  []Feature{feature("fighting_style", "戰鬥風格：防禦", "穿著護甲時 AC +1，已計入AC。"), feature("paladins_smite", "至聖斬", "永遠準備至聖斬（命中後 2d8 光耀傷害），且每次長休可免費施放一次。")},
		Spellcasting: &SpellcastingTemplate{
			Ability: "cha", Focus: "聖徽", Mode: "standard", Slots: []SlotTemplate{{Level: 1, Max: 3}},
			Cantrips: []string{}, Prepared: []string{"bless", "command", "cure_wounds", "divine_favor"},
			AlwaysPrepared: []string{"divine_smite", "protection_evil_good", "shield_of_faith"},
		},
	}},
	{"盜賊", ClassDefinition{
		Subclass: "竊賊", Background: "罪犯", HitDie: 8,
		Abilities: AbilityScores{Str: 10, Dex: 17, Con: 14, Int: 13, Wis: 12, Cha: 8}, AC: 15, Speed: 30,
		Saves: []AbilityKey{"dex", "int"}, ProficientSkills: []string{"隱匿", "巧手", "特技", "調查", "察覺", "欺瞞"}, ExpertiseSkills: []string{"隱匿", "巧手"},
		Equipment: []string{"皮甲", "短劍 ×2", "短弓與 20 支箭", "盜賊工具", "竊賊套組"},
		Attacks:   []AttackTemplate{attack("shortsword", "短劍", "dex", "1d6", "穿刺", "靈巧", "輕型", "煩擾精通"), attack("shortbow", "短弓", "dex", "1d6", "穿刺", "彈藥 80/320", "雙手")},
		Features:  []Feature{feature("sneak_attack", "偷襲", "只要仍有隊友未倒下與你夾擊，每次武器命中額外造成 2d6 傷害（隨等級成長，由系統結算）。"), feature("expertise", "專精", "隱匿與巧手使用雙倍熟練加值。")},
	}},
	{"法師", ClassDefinition{
		Subclass: "塑能師", Background: "賢者", HitDie: 6,
		Abilities: AbilityScores{Str: 8, Dex: 14, Con: 15, Int: 17, Wis: 12, Cha: 10}, AC: 12, Speed: 30,
		Saves: []AbilityKey{"int", "wis"}, ProficientSkills: []string{"奧秘", "歷史", "調查", "洞悉"}, ExpertiseSkills: []string{"奧秘"},
		Equipment: []string{"長棍", "匕首", "奧術法器", "法術書", "學者套組"},
		Attacks:   []AttackTemplate{attack("quarterstaff", "長棍", "str", "1d6", "鈍擊", "多用途 1d8", "擊倒精通")},
		Resources: []ResourceTemplate{resource("arcane_recovery", "奧術回復", 1, "在戰鬥外恢復合計至多 2 環的已消耗法術位（低環優先）；每次長休可用一次。", recoverNone, "")},
		Features:  []Feature{feature("ritual_adept", "儀式專家", "法術書內具儀式標籤的法術無須準備即可進行儀式施放。"), feature("scholar", "學者：奧秘", "奧秘技能取得專精。"), feature("potent_cantrip", "強效戲法", "傷害戲法即使目標豁免成功，仍造成一半傷害。")},
		Spellcasting: &SpellcastingTemplate{
			Ability: "int", Focus: "奧術法器或法術書", Mode: "standard", Slots: []SlotTemplate{{Level: 1, Max: 4}, {Level: 2, Max: 2}},
			Cantrips:  []string{"acid_splash", "light", "mage_hand", "ray_of_frost"},
			Prepared:  []string{"mage_armor", "magic_missile", "shield", "sleep", "misty_step", "scorching_ray"},
			Spellbook: []string{"detect_magic", "feather_fall", "mage_armor", "magic_missile", "shield", "sleep", "thunderwave", "charm_person", "misty_step", "scorching_ray", "burning_hands", "shatter"},
		},
	}},
}

// ClassDefinitions mirrors characters.ts classDefinitions. Iterate ClassNames
// wherever the TS declaration order matters.
var ClassDefinitions = func() map[string]ClassDefinition {
	definitions := make(map[string]ClassDefinition, len(classEntries))
	for _, entry := range classEntries {
		definitions[entry.name] = entry.definition
	}
	return definitions
}()

// ClassNames mirrors characters.ts classNames: Object.keys(classDefinitions)
// in object-literal declaration order.
var ClassNames = func() []string {
	names := make([]string, len(classEntries))
	for i, entry := range classEntries {
		names[i] = entry.name
	}
	return names
}()

// AbilityModifier mirrors characters.ts abilityModifier:
// Math.floor((score - 10) / 2). math.Floor keeps the JS rounding toward
// negative infinity for scores below 10 (Go int division truncates toward 0).
func AbilityModifier(score int) int {
	return int(math.Floor(float64(score-10) / 2))
}

// makeSkills mirrors characters.ts makeSkills, iterating skillAbilities in
// the TS Object.entries insertion order.
func makeSkills(definition ClassDefinition, proficiencyBonus int) []Skill {
	skills := make([]Skill, len(skillAbilities))
	for i, entry := range skillAbilities {
		proficient := containsString(definition.ProficientSkills, entry.name)
		expertise := containsString(definition.ExpertiseSkills, entry.name)
		bonus := AbilityModifier(definition.Abilities.Get(entry.ability))
		if expertise {
			bonus += proficiencyBonus * 2
		} else if proficient {
			bonus += proficiencyBonus
		}
		skills[i] = Skill{Name: entry.name, Ability: entry.ability, Proficient: proficient, Expertise: expertise, Bonus: bonus}
	}
	return skills
}

// containsString mirrors Array.prototype.includes for the template string
// slices (nil slices behave like the TS optional-chained `?.includes`).
func containsString(values []string, value string) bool {
	for _, entry := range values {
		if entry == value {
			return true
		}
	}
	return false
}

// mustMakeSpell wraps MakeSpell for the class templates, whose spell ids are
// all statically present in the catalog. The TS makeSpell would throw on an
// unknown id; a panic is the equivalent for this cannot-happen case.
func mustMakeSpell(id string, overrides SpellOverrides) Spell {
	spell, err := MakeSpell(id, overrides)
	if err != nil {
		panic(fmt.Sprintf("rules: class template references %v", err))
	}
	return spell
}

// boolPtr returns a pointer for SpellOverrides.Prepared (the TS
// `prepared ?? true` default makes the field a *bool in Go).
func boolPtr(value bool) *bool { return &value }

// makeSpells mirrors characters.ts makeSpells.
func makeSpells(template SpellcastingTemplate) []Spell {
	spells := make([]Spell, 0, len(template.Cantrips)+len(template.Prepared)+len(template.AlwaysPrepared)+len(template.Spellbook))
	for _, id := range template.Cantrips {
		spells = append(spells, mustMakeSpell(id, SpellOverrides{}))
	}
	for _, id := range template.Prepared {
		spells = append(spells, mustMakeSpell(id, SpellOverrides{Prepared: boolPtr(true), InSpellbook: containsString(template.Spellbook, id)}))
	}
	for _, id := range template.AlwaysPrepared {
		freeUseResourceID := ""
		if id == "divine_smite" {
			freeUseResourceID = "free_divine_smite"
		}
		spells = append(spells, mustMakeSpell(id, SpellOverrides{Prepared: boolPtr(true), AlwaysPrepared: true, FreeUseResourceID: freeUseResourceID}))
	}
	preparedIDs := make(map[string]struct{}, len(template.Cantrips)+len(template.Prepared)+len(template.AlwaysPrepared))
	for _, id := range template.Cantrips {
		preparedIDs[id] = struct{}{}
	}
	for _, id := range template.Prepared {
		preparedIDs[id] = struct{}{}
	}
	for _, id := range template.AlwaysPrepared {
		preparedIDs[id] = struct{}{}
	}
	for _, id := range template.Spellbook {
		if _, ok := preparedIDs[id]; ok {
			continue
		}
		spells = append(spells, mustMakeSpell(id, SpellOverrides{Prepared: boolPtr(false), InSpellbook: true}))
	}
	return spells
}

// isJSWhitespace reports whether r matches the JS regex \s class
// (WhiteSpace + LineTerminator), which String.prototype.trim also uses.
func isJSWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\u00A0', '\u1680', '\u2028', '\u2029', '\u202F', '\u205F', '\u3000', '\uFEFF':
		return true
	}
	return r >= '\u2000' && r <= '\u200A'
}

// isInitialSeparator matches the characters.ts /[\s・]/ separator class.
func isInitialSeparator(r rune) bool {
	return r == '・' || isJSWhitespace(r)
}

// makeInitials mirrors characters.ts makeInitials.
func makeInitials(name string) string {
	trimmed := strings.TrimFunc(name, isJSWhitespace)
	// split(/[\s・]+/).filter(Boolean)
	latinParts := strings.FieldsFunc(trimmed, isInitialSeparator)
	if len(latinParts) > 1 {
		allLatin := true
		for _, part := range latinParts {
			first := firstRune(part)
			if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')) {
				allLatin = false
				break
			}
		}
		if allLatin {
			var initials strings.Builder
			for _, part := range latinParts[:2] {
				initials.WriteRune(unicode.ToUpper(firstRune(part)))
			}
			return initials.String()
		}
	}
	// [...name.trim().replace(/[\s・]/g, '')].slice(0, 2).join('').toUpperCase() || 'AD'
	compact := strings.Map(func(r rune) rune {
		if isInitialSeparator(r) {
			return -1
		}
		return r
	}, trimmed)
	runes := []rune(compact)
	if len(runes) > 2 {
		runes = runes[:2]
	}
	initials := strings.ToUpper(string(runes))
	if initials == "" {
		return "AD"
	}
	return initials
}

// firstRune returns the first rune of s (U+FFFD for the empty string, which
// fails the Latin check just like an empty JS part would).
func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return '\uFFFD'
}

// CreateLevel3Character mirrors characters.ts createLevel3Character: it builds
// a fresh level-3 character sheet from a class template, falling back to 戰士
// for unknown class names.
func CreateLevel3Character(id, name, className string) Character {
	definition, ok := ClassDefinitions[className]
	resolvedClassName := className
	if !ok {
		definition = ClassDefinitions["戰士"]
		resolvedClassName = "戰士"
	}
	proficiencyBonus := 2
	conModifier := AbilityModifier(definition.Abilities.Con)
	averageHitDie := definition.HitDie/2 + 1 // Math.floor(hitDie / 2) + 1; hitDie > 0 so truncation == floor
	maxHP := definition.HitDie + conModifier + (averageHitDie+conModifier)*2 + definition.MaxHPBonus
	skills := makeSkills(definition, proficiencyBonus)
	// TS: skills.find((skill) => skill.name === '察覺')?.bonus || abilityModifier(wis)
	// The || means a found-but-zero bonus also falls back to the wis modifier.
	perception := AbilityModifier(definition.Abilities.Wis)
	for _, skill := range skills {
		if skill.Name == "察覺" {
			if skill.Bonus != 0 {
				perception = skill.Bonus
			}
			break
		}
	}
	resources := make([]Resource, len(definition.Resources))
	for i, entry := range definition.Resources {
		max := entry.Max
		if entry.MaxAbility != "" {
			max = AbilityModifier(definition.Abilities.Get(entry.MaxAbility))
			if max < 1 {
				max = 1 // Math.max(1, ...)
			}
		}
		resources[i] = Resource{
			ID:                entry.ID,
			Name:              entry.Name,
			Current:           max,
			Max:               max,
			Die:               entry.Die,
			Description:       entry.Description,
			ShortRestRecovery: entry.ShortRestRecovery,
		}
	}
	attacks := make([]Attack, len(definition.Attacks))
	for i, entry := range definition.Attacks {
		modifier := AbilityModifier(definition.Abilities.Get(entry.Ability))
		attackBonus := modifier + proficiencyBonus
		attacks[i] = Attack{
			ID:          entry.ID,
			Name:        entry.Name,
			AttackBonus: attackBonus,
			// `${damageDie}${modifier >= 0 ? '+' : ''}${modifier}`
			Damage:     fmt.Sprintf("%s%+d", entry.DamageDie, modifier),
			DamageType: entry.DamageType,
			// The TS template shares the properties array by reference; keep
			// the template slice immutable downstream.
			Properties: entry.Properties,
		}
	}
	var spellcasting *Spellcasting
	if definition.Spellcasting != nil {
		template := definition.Spellcasting
		castingModifier := AbilityModifier(definition.Abilities.Get(template.Ability))
		slots := make([]SlotPool, len(template.Slots))
		for i, slot := range template.Slots {
			slots[i] = SlotPool{Level: slot.Level, Current: slot.Max, Max: slot.Max}
		}
		spellcasting = &Spellcasting{
			Ability:       template.Ability,
			AttackBonus:   proficiencyBonus + castingModifier,
			SaveDC:        8 + proficiencyBonus + castingModifier,
			Focus:         template.Focus,
			Mode:          template.Mode,
			PactSlotLevel: template.PactSlotLevel,
			Slots:         slots,
			Spells:        makeSpells(*template),
		}
	}

	// 戰士 精通重擊 (improved critical): crits on natural 19–20.
	critThreshold := 0
	if resolvedClassName == "戰士" {
		critThreshold = 19
	}
	// 盜賊 偷襲 (sneak attack): 2d6 at level 3.
	sneakAttackDice := 0
	if resolvedClassName == "盜賊" {
		sneakAttackDice = 2
	}

	return Character{
		ID:               id,
		Name:             strings.TrimFunc(name, isJSWhitespace),
		ClassName:        resolvedClassName,
		CritThreshold:    critThreshold,
		SneakAttackDice:  sneakAttackDice,
		Subclass:         definition.Subclass,
		Species:          "人類",
		Background:       definition.Background,
		Level:            3,
		Initials:         makeInitials(name),
		HP:               maxHP,
		MaxHP:            maxHP,
		AC:               definition.AC,
		Passive:          10 + perception,
		Speed:            definition.Speed,
		Initiative:       AbilityModifier(definition.Abilities.Dex),
		ProficiencyBonus: proficiencyBonus,
		HitDie:           definition.HitDie,
		HitDice:          3,
		MaxHitDice:       3,
		Abilities:        definition.Abilities,
		SavingThrowProfs: append([]AbilityKey(nil), definition.Saves...),
		Skills:           skills,
		Attacks:          attacks,
		Equipment:        append([]string(nil), definition.Equipment...),
		Resources:        resources,
		Features:         append([]Feature(nil), definition.Features...),
		Spellcasting:     spellcasting,
		Condition:        "正常",
		Experience:       900,
		AbilityPoints:    0,
	}
}

// ChangeResource mirrors characters.ts changeResource: it clamps the matched
// resource's current value to [0, max] after applying delta, returning a copy.
func ChangeResource(c Character, resourceID string, delta int) Character {
	resources := make([]Resource, len(c.Resources))
	for i, entry := range c.Resources {
		if entry.ID == resourceID {
			// Math.max(0, Math.min(entry.max, entry.current + delta))
			next := entry.Current + delta
			if next > entry.Max {
				next = entry.Max
			}
			if next < 0 {
				next = 0
			}
			entry.Current = next
		}
		resources[i] = entry
	}
	c.Resources = resources
	return c
}

// RestCharacter mirrors characters.ts restCharacter. restType is "short" or
// "long" (the TS RestType union); anything else behaves as a short rest,
// matching the TS `type === 'long'` checks.
func RestCharacter(c Character, restType string) Character {
	long := restType == "long"
	resources := make([]Resource, len(c.Resources))
	for i, entry := range c.Resources {
		if long {
			entry.Current = entry.Max
		} else {
			recovered := entry.Current + entry.ShortRestRecovery.Amount
			if entry.ShortRestRecovery.All {
				recovered = entry.Max
			}
			if recovered > entry.Max {
				recovered = entry.Max // Math.min(entry.max, recovered)
			}
			entry.Current = recovered
		}
		resources[i] = entry
	}
	var spellcasting *Spellcasting
	if c.Spellcasting != nil {
		copied := *c.Spellcasting
		slots := make([]SlotPool, len(copied.Slots))
		for i, slot := range copied.Slots {
			if long || copied.Mode == "pact" {
				slot.Current = slot.Max
			}
			slots[i] = slot
		}
		copied.Slots = slots
		spellcasting = &copied
	}
	if long {
		c.HP = c.MaxHP
		c.HitDice = c.MaxHitDice
		c.Concentration = ""
		c.Condition = "正常"
	}
	c.Resources = resources
	c.Spellcasting = spellcasting
	return c
}

// SpendSpellSlot mirrors characters.ts spendSpellSlot. The TS function returns
// null when the spell needs a slot but none is available (or the character has
// no spellcasting); that maps to (Character{}, false) here. Cantrips and
// ritual casts cost nothing, and a free-use resource (聖武士 free_divine_smite)
// is consumed before any slot.
func SpendSpellSlot(c Character, spell Spell, asRitual bool) (Character, bool) {
	if spell.Level == 0 || (asRitual && spell.Ritual) {
		if spell.Concentration {
			c.Concentration = spell.Name
		}
		return c, true
	}
	if c.Spellcasting == nil {
		return Character{}, false
	}
	if spell.FreeUseResourceID != "" {
		freeUse := false
		for _, entry := range c.Resources {
			if entry.ID == spell.FreeUseResourceID && entry.Current > 0 {
				freeUse = true
				break
			}
		}
		if freeUse {
			resources := make([]Resource, len(c.Resources))
			for i, entry := range c.Resources {
				if entry.ID == spell.FreeUseResourceID {
					entry.Current--
				}
				resources[i] = entry
			}
			if spell.Concentration {
				c.Concentration = spell.Name
			}
			c.Resources = resources
			return c, true
		}
	}
	slotIndex := -1
	for i, slot := range c.Spellcasting.Slots {
		if slot.Level >= spell.Level && slot.Current > 0 {
			slotIndex = i
			break
		}
	}
	if slotIndex < 0 {
		return Character{}, false
	}
	copied := *c.Spellcasting
	slots := make([]SlotPool, len(copied.Slots))
	copy(slots, copied.Slots)
	slots[slotIndex].Current--
	copied.Slots = slots
	if spell.Concentration {
		c.Concentration = spell.Name
	}
	c.Spellcasting = &copied
	return c, true
}

// GetCheckBonus mirrors characters.ts getCheckBonus: 先攻 uses initiative,
// a named skill uses its sheet bonus, an ability label (力量…魅力) uses the
// raw ability modifier, and anything else is 0. On top of the TS port, a
// 吟遊詩人 adds half proficiency (萬事通, jack of all trades) to any skill or
// ability check that does not already include the proficiency bonus.
func GetCheckBonus(c Character, check string) int {
	if check == "先攻" {
		return c.Initiative
	}
	for _, skill := range c.Skills {
		if skill.Name == check {
			bonus := skill.Bonus
			if !skill.Proficient && !skill.Expertise && HasClass(c, "吟遊詩人") {
				bonus += c.ProficiencyBonus / 2
			}
			return bonus
		}
	}
	// Object.entries(abilityLabels) insertion order == AbilityKeys order.
	for _, key := range AbilityKeys {
		if AbilityLabels[key] == check {
			bonus := AbilityModifier(c.Abilities.Get(key))
			if HasClass(c, "吟遊詩人") {
				bonus += c.ProficiencyBonus / 2
			}
			return bonus
		}
	}
	return 0
}
