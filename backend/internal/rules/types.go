// Package rules is the server-side D&D 5e (2024 / SRD 5.2.1) rules engine,
// ported 1:1 from frontend/src/rules/*.ts so the server becomes the single
// source of truth for character state, derived stats, and combat resolution.
//
// JSON tags match the frontend TypeScript field names (camelCase) exactly:
// character documents stored in SQLite, localStorage vault imports, and the
// campaign view API all round-trip through these types unchanged.
package rules

import (
	"encoding/json"
	"fmt"
)

// AbilityKey is one of str/dex/con/int/wis/cha.
type AbilityKey = string

// AbilityKeys preserves the canonical ability order used across prompts and UI.
var AbilityKeys = []AbilityKey{"str", "dex", "con", "int", "wis", "cha"}

// AbilityScores mirrors types.ts AbilityScores.
type AbilityScores struct {
	Str int `json:"str"`
	Dex int `json:"dex"`
	Con int `json:"con"`
	Int int `json:"int"`
	Wis int `json:"wis"`
	Cha int `json:"cha"`
}

// Get returns the score for an ability key (0 for unknown keys).
func (a AbilityScores) Get(key AbilityKey) int {
	switch key {
	case "str":
		return a.Str
	case "dex":
		return a.Dex
	case "con":
		return a.Con
	case "int":
		return a.Int
	case "wis":
		return a.Wis
	case "cha":
		return a.Cha
	}
	return 0
}

// Set returns a copy with the given ability changed.
func (a AbilityScores) Set(key AbilityKey, value int) AbilityScores {
	switch key {
	case "str":
		a.Str = value
	case "dex":
		a.Dex = value
	case "con":
		a.Con = value
	case "int":
		a.Int = value
	case "wis":
		a.Wis = value
	case "cha":
		a.Cha = value
	}
	return a
}

// Skill mirrors types.ts CharacterSkill.
type Skill struct {
	Name       string     `json:"name"`
	Ability    AbilityKey `json:"ability"`
	Proficient bool       `json:"proficient"`
	Expertise  bool       `json:"expertise"`
	Bonus      int        `json:"bonus"`
}

// Attack mirrors types.ts CharacterAttack.
type Attack struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AttackBonus int      `json:"attackBonus"`
	Damage      string   `json:"damage"`
	DamageType  string   `json:"damageType"`
	Properties  []string `json:"properties"`
}

// ShortRestRecovery mirrors the TS union `number | 'all'`. All=true means the
// resource fully recovers on a short rest; otherwise Amount applies.
type ShortRestRecovery struct {
	All    bool
	Amount int
}

// MarshalJSON emits "all" or a bare number, matching the TS union.
func (s ShortRestRecovery) MarshalJSON() ([]byte, error) {
	if s.All {
		return json.Marshal("all")
	}
	return json.Marshal(s.Amount)
}

// UnmarshalJSON accepts "all" or a number.
func (s *ShortRestRecovery) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "all" {
			*s = ShortRestRecovery{All: true}
			return nil
		}
		return fmt.Errorf("invalid shortRestRecovery %q", str)
	}
	var n float64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("invalid shortRestRecovery: %s", data)
	}
	*s = ShortRestRecovery{Amount: int(n)}
	return nil
}

// Resource mirrors types.ts CharacterResource.
type Resource struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Current           int               `json:"current"`
	Max               int               `json:"max"`
	Die               string            `json:"die,omitempty"`
	Description       string            `json:"description"`
	ShortRestRecovery ShortRestRecovery `json:"shortRestRecovery"`
}

// Feature mirrors types.ts ClassFeature.
type Feature struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SpellEffect mirrors types.ts SpellEffect.
type SpellEffect struct {
	Kind               string     `json:"kind"` // damage | healing | temporaryHp | condition
	Target             string     `json:"target"`
	Dice               string     `json:"dice,omitempty"`
	Flat               int        `json:"flat,omitempty"`
	AddAbilityModifier bool       `json:"addAbilityModifier,omitempty"`
	AttackRoll         bool       `json:"attackRoll,omitempty"`
	AutomaticHit       bool       `json:"automaticHit,omitempty"`
	SaveAbility        AbilityKey `json:"saveAbility,omitempty"`
	HalfOnSave         bool       `json:"halfOnSave,omitempty"`
	Condition          string     `json:"condition,omitempty"`
	DamageType         string     `json:"damageType,omitempty"`
}

// Spell mirrors types.ts CharacterSpell.
type Spell struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	EnglishName       string       `json:"englishName"`
	Level             int          `json:"level"`
	School            string       `json:"school"`
	CastingTime       string       `json:"castingTime"`
	Range             string       `json:"range"`
	Duration          string       `json:"duration"`
	Description       string       `json:"description"`
	Concentration     bool         `json:"concentration"`
	Ritual            bool         `json:"ritual"`
	Prepared          bool         `json:"prepared"`
	AlwaysPrepared    bool         `json:"alwaysPrepared"`
	InSpellbook       bool         `json:"inSpellbook"`
	FreeUseResourceID string       `json:"freeUseResourceId,omitempty"`
	Effect            *SpellEffect `json:"effect,omitempty"`
}

// SlotPool mirrors types.ts SpellSlotPool.
type SlotPool struct {
	Level   int `json:"level"`
	Current int `json:"current"`
	Max     int `json:"max"`
}

// Spellcasting mirrors types.ts CharacterSpellcasting.
type Spellcasting struct {
	Ability       AbilityKey `json:"ability"`
	AttackBonus   int        `json:"attackBonus"`
	SaveDC        int        `json:"saveDc"`
	Focus         string     `json:"focus"`
	Mode          string     `json:"mode"` // standard | pact
	PactSlotLevel int        `json:"pactSlotLevel,omitempty"`
	Slots         []SlotPool `json:"slots"`
	Spells        []Spell    `json:"spells"`
}

// ClassLevel mirrors types.ts CharacterClassLevel.
type ClassLevel struct {
	ClassName string `json:"className"`
	Level     int    `json:"level"`
	Subclass  string `json:"subclass,omitempty"`
}

// Character mirrors types.ts PlayerCharacter.
type Character struct {
	ID               string        `json:"id"` // player1..player4
	Name             string        `json:"name"`
	ClassName        string        `json:"className"`
	Subclass         string        `json:"subclass"`
	Species          string        `json:"species"`
	Background       string        `json:"background"`
	Level            int           `json:"level"`
	ClassLevels      []ClassLevel  `json:"classLevels,omitempty"`
	Initials         string        `json:"initials"`
	HP               int           `json:"hp"`
	TemporaryHP      int           `json:"temporaryHp,omitempty"`
	MaxHP            int           `json:"maxHp"`
	AC               int           `json:"ac"`
	Passive          int           `json:"passive"`
	Speed            int           `json:"speed"`
	Initiative       int           `json:"initiative"`
	ProficiencyBonus int           `json:"proficiencyBonus"`
	HitDie           int           `json:"hitDie"`
	HitDice          int           `json:"hitDice"`
	MaxHitDice       int           `json:"maxHitDice"`
	Abilities        AbilityScores `json:"abilities"`
	SavingThrowProfs []AbilityKey  `json:"savingThrowProficiencies"`
	Skills           []Skill       `json:"skills"`
	Attacks          []Attack      `json:"attacks"`
	Equipment        []string      `json:"equipment"`
	Gold             int           `json:"gold"`
	Resources        []Resource    `json:"resources"`
	Features         []Feature     `json:"features"`
	Spellcasting     *Spellcasting `json:"spellcasting,omitempty"`
	Concentration    string        `json:"concentration,omitempty"`
	Condition        string        `json:"condition"`
	Experience       int           `json:"experience"`
	AbilityPoints    int           `json:"abilityPoints,omitempty"`
	Appearance       string        `json:"appearance,omitempty"`
	PortraitURL      string        `json:"portraitUrl,omitempty"`
}

// TurnEconomy mirrors the per-combatant action-usage record in types.ts CombatState.
type TurnEconomy struct {
	ActionUsed      bool `json:"actionUsed"`
	BonusActionUsed bool `json:"bonusActionUsed"`
	ReactionUsed    bool `json:"reactionUsed"`
}

// Combatant mirrors types.ts Combatant.
type Combatant struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Side            string         `json:"side"` // party | enemy | neutral
	PlayerID        string         `json:"playerId,omitempty"`
	InitiativeBonus int            `json:"initiativeBonus"`
	Initiative      int            `json:"initiative"`
	AC              int            `json:"ac"`
	HP              int            `json:"hp"`
	TemporaryHP     int            `json:"temporaryHp,omitempty"`
	MaxHP           int            `json:"maxHp"`
	AttackBonus     int            `json:"attackBonus"`
	Damage          string         `json:"damage"`
	DamageType      string         `json:"damageType"`
	SavingThrows    map[string]int `json:"savingThrows,omitempty"`
	Defeated        bool           `json:"defeated,omitempty"`
}

// CombatState mirrors types.ts CombatState.
type CombatState struct {
	Active      bool                   `json:"active"`
	Round       int                    `json:"round"`
	TurnIndex   int                    `json:"turnIndex"`
	Combatants  []Combatant            `json:"combatants"`
	TurnEconomy map[string]TurnEconomy `json:"turnEconomy,omitempty"`
}

// RequiredCheck mirrors types.ts RequiredCheck.
type RequiredCheck struct {
	Character string `json:"character"`
	Ability   string `json:"ability"`
	Skill     string `json:"skill"`
	DC        int    `json:"dc"`
	Reason    string `json:"reason"`
	Modifier  int    `json:"modifier,omitempty"`
	PlayerID  string `json:"playerId,omitempty"`
}

// Choice mirrors types.ts Choice.
type Choice struct {
	Text     string `json:"text"`
	PlayerID string `json:"playerId,omitempty"`
}

// StoryEntry mirrors types.ts StoryEntry.
type StoryEntry struct {
	ID       string `json:"id"`
	Speaker  string `json:"speaker"`
	Text     string `json:"text"`
	Time     string `json:"time"`
	Audience string `json:"audience,omitempty"`
}
