package rules

// Golden round-trip tests: hand-written JSON fixtures matching the frontend
// contract in frontend/src/types.ts (PlayerCharacter). They assert that
// json.Unmarshal → json.Marshal through the Go Character type preserves all
// semantic content, so character documents written by the TS client survive a
// round trip through the Go backend unchanged.
//
// Comparison strategy: unmarshal both the fixture and the re-marshaled bytes
// into map[string]any and reflect.DeepEqual after normalizing. Normalization
// removes semantically-empty object entries (null, "", 0, false, empty
// array/object) from both sides, mirroring the TS codebase's `?? fallback` /
// `|| fallback` optional-field semantics: a field Go omits via omitempty is
// only acceptable when the source value was the zero value. Any dropped or
// altered non-zero content still fails DeepEqual.

import (
	"encoding/json"
	"reflect"
	"testing"
)

// goldenCharacterJSON is a realistic level-5 multiclass character exercising
// every optional field of the schema: classLevels (with and without subclass),
// temporaryHp, concentration, abilityPoints, appearance/portraitUrl, resources
// with shortRestRecovery "all" AND numeric (0 and non-zero) variants plus a
// die, and standard spellcasting with cantrips, save/attack-roll/healing/
// temporaryHp spell effects, freeUseResourceId, and a spellbook entry.
const goldenCharacterJSON = `{
  "id": "player1",
  "name": "瑟蕾娜",
  "className": "牧師",
  "subclass": "生命領域",
  "species": "丘陵矮人",
  "background": "侍僧",
  "level": 5,
  "classLevels": [
    { "className": "牧師", "level": 4, "subclass": "生命領域" },
    { "className": "戰士", "level": 1 }
  ],
  "initials": "瑟",
  "hp": 31,
  "temporaryHp": 6,
  "maxHp": 38,
  "ac": 18,
  "passive": 14,
  "speed": 25,
  "initiative": 1,
  "proficiencyBonus": 3,
  "hitDie": 8,
  "hitDice": 4,
  "maxHitDice": 5,
  "abilities": { "str": 14, "dex": 12, "con": 15, "int": 10, "wis": 16, "cha": 13 },
  "savingThrowProficiencies": ["wis", "cha"],
  "skills": [
    { "name": "洞悉", "ability": "wis", "proficient": true, "expertise": false, "bonus": 6 },
    { "name": "宗教", "ability": "int", "proficient": true, "expertise": false, "bonus": 3 },
    { "name": "說服", "ability": "cha", "proficient": false, "expertise": false, "bonus": 1 }
  ],
  "attacks": [
    { "id": "mace", "name": "釘頭錘", "attackBonus": 5, "damage": "1d6+2", "damageType": "鈍擊", "properties": [] },
    { "id": "light_crossbow", "name": "輕弩", "attackBonus": 4, "damage": "1d8+1", "damageType": "穿刺", "properties": ["彈藥", "遠程", "雙手"] }
  ],
  "equipment": ["鎖子甲", "盾牌", "聖徽", "祭司包"],
  "resources": [
    { "id": "channel_divinity", "name": "引導神力", "current": 1, "max": 2, "description": "使用神聖火花、驅散不死生物或領域效果。", "shortRestRecovery": 1 },
    { "id": "action_surge", "name": "動作如潮", "current": 0, "max": 1, "description": "在自己的回合額外進行一個動作，但不能是魔法動作。", "shortRestRecovery": "all" },
    { "id": "bardic_inspiration", "name": "吟遊激勵", "current": 2, "max": 3, "die": "d6", "description": "附贈動作給予 d6；目標檢定失敗時可加入骰值。", "shortRestRecovery": 0 },
    { "id": "free_divine_smite", "name": "免費至聖斬", "current": 1, "max": 1, "description": "每次長休可不消耗法術位施放一次至聖斬。", "shortRestRecovery": 0 }
  ],
  "features": [
    { "id": "divine_order", "name": "神聖秩序", "description": "選擇奇蹟使者或護教軍加強施法或武裝。" },
    { "id": "channel_divinity_feature", "name": "引導神力", "description": "以神聖能量驅動特殊效果。" }
  ],
  "spellcasting": {
    "ability": "wis",
    "attackBonus": 6,
    "saveDc": 14,
    "focus": "聖徽",
    "mode": "standard",
    "slots": [
      { "level": 1, "current": 3, "max": 4 },
      { "level": 2, "current": 2, "max": 3 },
      { "level": 3, "current": 0, "max": 2 }
    ],
    "spells": [
      {
        "id": "sacred_flame", "name": "聖火術", "englishName": "Sacred Flame", "level": 0,
        "school": "塑能", "castingTime": "動作", "range": "60 呎", "duration": "立即",
        "description": "目標進行敏捷豁免；失敗受到 1d8 光耀傷害，掩護不提供此豁免加值。",
        "concentration": false, "ritual": false, "prepared": true, "alwaysPrepared": true, "inSpellbook": false,
        "effect": { "kind": "damage", "target": "creature", "dice": "1d8", "saveAbility": "dex", "damageType": "光耀" }
      },
      {
        "id": "cure_wounds", "name": "治療傷勢", "englishName": "Cure Wounds", "level": 1,
        "school": "防護", "castingTime": "動作", "range": "觸碰", "duration": "立即",
        "description": "目標恢復 2d8 加施法屬性調整值的生命。",
        "concentration": false, "ritual": false, "prepared": true, "alwaysPrepared": false, "inSpellbook": false,
        "effect": { "kind": "healing", "target": "ally", "dice": "2d8", "addAbilityModifier": true }
      },
      {
        "id": "bless", "name": "祝福術", "englishName": "Bless", "level": 1,
        "school": "惑控", "castingTime": "動作", "range": "30 呎", "duration": "專注，1 分鐘",
        "description": "最多三個目標的攻擊與豁免各增加 1d4。",
        "concentration": true, "ritual": false, "prepared": true, "alwaysPrepared": false, "inSpellbook": false
      },
      {
        "id": "guiding_bolt", "name": "曳光彈", "englishName": "Guiding Bolt", "level": 1,
        "school": "塑能", "castingTime": "動作", "range": "120 呎", "duration": "1 輪",
        "description": "遠程法術攻擊造成 4d6 光耀傷害；下一次攻擊該目標具有優勢。",
        "concentration": false, "ritual": false, "prepared": true, "alwaysPrepared": false, "inSpellbook": false,
        "effect": { "kind": "damage", "target": "creature", "dice": "4d6", "attackRoll": true, "damageType": "光耀" }
      },
      {
        "id": "divine_smite", "name": "至聖斬", "englishName": "Divine Smite", "level": 1,
        "school": "塑能", "castingTime": "命中後附贈動作", "range": "自身", "duration": "立即",
        "description": "近戰武器或徒手命中後額外造成 2d8 光耀傷害，對邪魔與不死生物再增加 1d8。",
        "concentration": false, "ritual": false, "prepared": true, "alwaysPrepared": true, "inSpellbook": false,
        "freeUseResourceId": "free_divine_smite"
      },
      {
        "id": "armor_of_agathys", "name": "阿嘉西斯之鎧", "englishName": "Armor of Agathys", "level": 1,
        "school": "防護", "castingTime": "附贈動作", "range": "自身", "duration": "1 小時",
        "description": "獲得 5 點暫時生命；暫時生命仍在時，近戰命中你的生物受到 5 點冷凍傷害。",
        "concentration": false, "ritual": false, "prepared": false, "alwaysPrepared": false, "inSpellbook": true,
        "effect": { "kind": "temporaryHp", "target": "self", "flat": 5 }
      }
    ]
  },
  "concentration": "祝福術",
  "condition": "健康",
  "experience": 6800,
  "abilityPoints": 1,
  "appearance": "銀髮矮人牧師，肩負刻著晨曦之主徽記的盾牌。",
  "portraitUrl": "/portraits/player1.png"
}`

// goldenWarlockJSON exercises pact-magic spellcasting (mode "pact" with
// pactSlotLevel) plus an "all" short-rest resource.
const goldenWarlockJSON = `{
  "id": "player2",
  "name": "維茲",
  "className": "邪術師",
  "subclass": "至高妖精宗主",
  "species": "提夫林",
  "background": "隱士",
  "level": 3,
  "classLevels": [
    { "className": "邪術師", "level": 3, "subclass": "至高妖精宗主" }
  ],
  "initials": "維",
  "hp": 21,
  "maxHp": 24,
  "ac": 13,
  "passive": 11,
  "speed": 30,
  "initiative": 2,
  "proficiencyBonus": 2,
  "hitDie": 8,
  "hitDice": 3,
  "maxHitDice": 3,
  "abilities": { "str": 8, "dex": 14, "con": 14, "int": 12, "wis": 10, "cha": 16 },
  "savingThrowProficiencies": ["wis", "cha"],
  "skills": [
    { "name": "奧祕", "ability": "int", "proficient": true, "expertise": false, "bonus": 3 },
    { "name": "欺瞞", "ability": "cha", "proficient": true, "expertise": true, "bonus": 7 }
  ],
  "attacks": [
    { "id": "dagger", "name": "匕首", "attackBonus": 4, "damage": "1d4+2", "damageType": "穿刺", "properties": ["靈巧", "輕型", "投擲"] }
  ],
  "equipment": ["皮甲", "匕首", "秘法法器"],
  "resources": [
    { "id": "pact_boon", "name": "魔契恩賜", "current": 1, "max": 1, "description": "短休後恢復所有契約法術位。", "shortRestRecovery": "all" }
  ],
  "features": [
    { "id": "eldritch_invocations", "name": "魔能祈喚", "description": "習得增強邪術能力的祕法祈喚。" }
  ],
  "spellcasting": {
    "ability": "cha",
    "attackBonus": 5,
    "saveDc": 13,
    "focus": "秘法法器",
    "mode": "pact",
    "pactSlotLevel": 2,
    "slots": [
      { "level": 2, "current": 1, "max": 2 }
    ],
    "spells": [
      {
        "id": "armor_of_agathys", "name": "阿嘉西斯之鎧", "englishName": "Armor of Agathys", "level": 1,
        "school": "防護", "castingTime": "附贈動作", "range": "自身", "duration": "1 小時",
        "description": "獲得 5 點暫時生命；暫時生命仍在時，近戰命中你的生物受到 5 點冷凍傷害。",
        "concentration": false, "ritual": false, "prepared": true, "alwaysPrepared": false, "inSpellbook": false,
        "effect": { "kind": "temporaryHp", "target": "self", "flat": 5 }
      }
    ]
  },
  "condition": "健康",
  "experience": 900
}`

// isSemanticallyEmpty reports whether a decoded JSON value is the TS-optional
// zero value: null, false, 0, "", or an empty array/object.
func isSemanticallyEmpty(v any) bool {
	switch value := v.(type) {
	case nil:
		return true
	case bool:
		return !value
	case float64:
		return value == 0
	case string:
		return value == ""
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	}
	return false
}

// normalizeJSON recursively strips semantically-empty entries from objects.
// Array elements are kept in place (positions are meaningful) but normalized.
func normalizeJSON(v any) any {
	switch value := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, entry := range value {
			normalized := normalizeJSON(entry)
			if isSemanticallyEmpty(normalized) {
				continue
			}
			out[key] = normalized
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, entry := range value {
			out[i] = normalizeJSON(entry)
		}
		return out
	}
	return v
}

// roundTripCharacter unmarshals fixture into Character, marshals it back, and
// returns the typed character plus the fixture and round-trip generic maps.
func roundTripCharacter(t *testing.T, fixture string) (Character, map[string]any, map[string]any) {
	t.Helper()

	var character Character
	if err := json.Unmarshal([]byte(fixture), &character); err != nil {
		t.Fatalf("unmarshal fixture into Character: %v", err)
	}
	remarshaled, err := json.Marshal(character)
	if err != nil {
		t.Fatalf("marshal Character: %v", err)
	}

	var original map[string]any
	if err := json.Unmarshal([]byte(fixture), &original); err != nil {
		t.Fatalf("unmarshal fixture into map: %v", err)
	}
	var roundTrip map[string]any
	if err := json.Unmarshal(remarshaled, &roundTrip); err != nil {
		t.Fatalf("unmarshal round-trip output into map: %v", err)
	}
	return character, original, roundTrip
}

func TestGoldenCharacterRoundTrip(t *testing.T) {
	character, original, roundTrip := roundTripCharacter(t, goldenCharacterJSON)

	// Typed spot checks: the union and optional fields decoded correctly.
	if character.TemporaryHP != 6 {
		t.Fatalf("temporaryHp = %d, want 6", character.TemporaryHP)
	}
	if len(character.ClassLevels) != 2 || character.ClassLevels[0].Subclass != "生命領域" || character.ClassLevels[1].Subclass != "" {
		t.Fatalf("classLevels decoded incorrectly: %+v", character.ClassLevels)
	}
	wantRecovery := map[string]ShortRestRecovery{
		"channel_divinity":   {Amount: 1},
		"action_surge":       {All: true},
		"bardic_inspiration": {},
		"free_divine_smite":  {},
	}
	for _, res := range character.Resources {
		if res.ShortRestRecovery != wantRecovery[res.ID] {
			t.Fatalf("resource %s shortRestRecovery = %+v, want %+v", res.ID, res.ShortRestRecovery, wantRecovery[res.ID])
		}
	}
	if character.Spellcasting == nil || character.Spellcasting.Mode != "standard" || len(character.Spellcasting.Spells) != 6 {
		t.Fatalf("spellcasting decoded incorrectly: %+v", character.Spellcasting)
	}
	if character.Spellcasting.Spells[4].FreeUseResourceID != "free_divine_smite" {
		t.Fatalf("freeUseResourceId = %q, want free_divine_smite", character.Spellcasting.Spells[4].FreeUseResourceID)
	}
	if effect := character.Spellcasting.Spells[5].Effect; effect == nil || effect.Kind != "temporaryHp" || effect.Flat != 5 {
		t.Fatalf("armor_of_agathys effect decoded incorrectly: %+v", character.Spellcasting.Spells[5].Effect)
	}

	// Raw output spot checks: the union re-encodes as "all" / bare numbers and
	// non-zero optionals survive verbatim.
	resources := roundTrip["resources"].([]any)
	if got := resources[1].(map[string]any)["shortRestRecovery"]; got != "all" {
		t.Fatalf(`re-marshaled action_surge shortRestRecovery = %v, want "all"`, got)
	}
	if got := resources[0].(map[string]any)["shortRestRecovery"]; got != float64(1) {
		t.Fatalf("re-marshaled channel_divinity shortRestRecovery = %v, want 1", got)
	}
	if got := resources[2].(map[string]any)["shortRestRecovery"]; got != float64(0) {
		t.Fatalf("re-marshaled bardic_inspiration shortRestRecovery = %v, want 0", got)
	}
	if got := roundTrip["temporaryHp"]; got != float64(6) {
		t.Fatalf("re-marshaled temporaryHp = %v, want 6", got)
	}
	if got := roundTrip["concentration"]; got != "祝福術" {
		t.Fatalf("re-marshaled concentration = %v, want 祝福術", got)
	}

	// Full semantic comparison.
	normalizedOriginal := normalizeJSON(original)
	normalizedRoundTrip := normalizeJSON(roundTrip)
	if !reflect.DeepEqual(normalizedOriginal, normalizedRoundTrip) {
		originalJSON, _ := json.MarshalIndent(normalizedOriginal, "", "  ")
		roundTripJSON, _ := json.MarshalIndent(normalizedRoundTrip, "", "  ")
		t.Fatalf("round trip lost semantic content.\noriginal:\n%s\nround trip:\n%s", originalJSON, roundTripJSON)
	}
}

func TestGoldenWarlockPactRoundTrip(t *testing.T) {
	character, original, roundTrip := roundTripCharacter(t, goldenWarlockJSON)

	if character.Spellcasting == nil || character.Spellcasting.Mode != "pact" || character.Spellcasting.PactSlotLevel != 2 {
		t.Fatalf("pact spellcasting decoded incorrectly: %+v", character.Spellcasting)
	}
	if len(character.Resources) != 1 || !character.Resources[0].ShortRestRecovery.All {
		t.Fatalf("pact resource decoded incorrectly: %+v", character.Resources)
	}
	spellcasting := roundTrip["spellcasting"].(map[string]any)
	if got := spellcasting["pactSlotLevel"]; got != float64(2) {
		t.Fatalf("re-marshaled pactSlotLevel = %v, want 2", got)
	}

	if !reflect.DeepEqual(normalizeJSON(original), normalizeJSON(roundTrip)) {
		originalJSON, _ := json.MarshalIndent(normalizeJSON(original), "", "  ")
		roundTripJSON, _ := json.MarshalIndent(normalizeJSON(roundTrip), "", "  ")
		t.Fatalf("round trip lost semantic content.\noriginal:\n%s\nround trip:\n%s", originalJSON, roundTripJSON)
	}
}

// TestGoldenZeroOptionalFieldsRoundTrip documents the accepted omitempty
// behavior: zero-valued optional fields (temporaryHp: 0, abilityPoints: 0,
// concentration: "") are dropped by Go's marshaler. This matches the TS
// codebase's `?? 0` / `|| ”` reads of these optionals, so the normalized
// semantic content is still identical.
func TestGoldenZeroOptionalFieldsRoundTrip(t *testing.T) {
	fixture := `{
  "id": "player3",
  "name": "布倫",
  "className": "戰士",
  "subclass": "",
  "species": "人類",
  "background": "士兵",
  "level": 1,
  "initials": "布",
  "hp": 12,
  "temporaryHp": 0,
  "maxHp": 12,
  "ac": 16,
  "passive": 11,
  "speed": 30,
  "initiative": 1,
  "proficiencyBonus": 2,
  "hitDie": 10,
  "hitDice": 1,
  "maxHitDice": 1,
  "abilities": { "str": 16, "dex": 12, "con": 14, "int": 10, "wis": 12, "cha": 8 },
  "savingThrowProficiencies": ["str", "con"],
  "skills": [],
  "attacks": [],
  "equipment": [],
  "resources": [],
  "features": [],
  "concentration": "",
  "condition": "健康",
  "experience": 0,
  "abilityPoints": 0
}`
	character, original, roundTrip := roundTripCharacter(t, fixture)

	if character.TemporaryHP != 0 || character.AbilityPoints != 0 || character.Concentration != "" {
		t.Fatalf("zero optionals decoded incorrectly: %+v", character)
	}
	for _, key := range []string{"temporaryHp", "abilityPoints"} {
		if _, present := roundTrip[key]; present {
			t.Fatalf("expected zero-valued %s to be omitted by omitempty", key)
		}
	}
	if !reflect.DeepEqual(normalizeJSON(original), normalizeJSON(roundTrip)) {
		originalJSON, _ := json.MarshalIndent(normalizeJSON(original), "", "  ")
		roundTripJSON, _ := json.MarshalIndent(normalizeJSON(roundTrip), "", "  ")
		t.Fatalf("round trip lost semantic content.\noriginal:\n%s\nround trip:\n%s", originalJSON, roundTripJSON)
	}
}
