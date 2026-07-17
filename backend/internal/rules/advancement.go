package rules

// Ported 1:1 from frontend/src/rules/advancement.ts: experience thresholds,
// derived-stat recalculation, configured character creation, level-up /
// multiclassing, ability score improvements, and prepared-spell selection.

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// fullCasters mirrors the advancement.ts fullCasters set: classes whose levels
// count fully toward the shared spell slot table.
var fullCasters = map[string]bool{
	"吟遊詩人": true, "牧師": true, "德魯伊": true, "術士": true, "法師": true,
}

// halfCasters mirrors the advancement.ts halfCasters set: classes whose levels
// count at half rate (rounded up) toward the shared spell slot table.
var halfCasters = map[string]bool{
	"聖武士": true, "遊俠": true,
}

// ExperienceThresholds mirrors advancement.ts experienceThresholds: the total
// XP required to reach each level, indexed by level (index 0 unused).
// Deliberately far below the official 5e table (~1/10–1/18): this duet plays
// short sessions with modest per-turn awards, so levels should land every few
// encounters, not after hundreds of rounds.
var ExperienceThresholds = []int{
	0, 0, 100, 250, 500, 900, 1400, 2000, 2700, 3500,
	4400, 5400, 6500, 7700, 9000, 10400, 11900, 13500, 15200, 17000,
	19000,
}

// abilityPointsPerLevel: this duet grants ability points on EVERY level-up
// (house rule replacing the official 4/8/12/16/19 ASI schedule) so growth is
// felt each level.
const abilityPointsPerLevel = 5

// slotTable mirrors the advancement.ts slotTable: standard spell slots per
// caster level (row index), one column per slot level starting at 1.
var slotTable = [][]int{
	{}, {2}, {3}, {4, 2}, {4, 3}, {4, 3, 2}, {4, 3, 3}, {4, 3, 3, 1}, {4, 3, 3, 2}, {4, 3, 3, 3, 1},
	{4, 3, 3, 3, 2}, {4, 3, 3, 3, 2, 1}, {4, 3, 3, 3, 2, 1}, {4, 3, 3, 3, 2, 1, 1}, {4, 3, 3, 3, 2, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1},
	{4, 3, 3, 3, 3, 1, 1, 1, 1}, {4, 3, 3, 3, 3, 2, 1, 1, 1}, {4, 3, 3, 3, 3, 2, 2, 1, 1},
}

// proficiencyForLevel mirrors advancement.ts proficiencyForLevel:
// 2 + Math.floor((Math.max(1, level) - 1) / 4). The clamped numerator is
// non-negative, so Go integer division matches Math.floor.
func proficiencyForLevel(level int) int {
	if level < 1 {
		level = 1
	}
	return 2 + (level-1)/4
}

// ExperienceForLevel mirrors advancement.ts experienceForLevel: the XP
// threshold for a level clamped to [1, 20].
func ExperienceForLevel(level int) int {
	if level < 1 {
		level = 1
	}
	if level > 20 {
		level = 20
	}
	return ExperienceThresholds[level]
}

// XPProgress mirrors the object literal returned by advancement.ts
// experienceToNextLevel.
type XPProgress struct {
	Current   int     `json:"current"`
	Required  int     `json:"required"`
	Remaining int     `json:"remaining"`
	Ready     bool    `json:"ready"`
	Progress  float64 `json:"progress"`
}

// ExperienceToNextLevel mirrors advancement.ts experienceToNextLevel.
func ExperienceToNextLevel(c Character) XPProgress {
	if c.Level >= 20 {
		return XPProgress{Current: c.Experience, Required: c.Experience, Remaining: 0, Ready: false, Progress: 1}
	}
	floor := ExperienceForLevel(c.Level)
	required := ExperienceForLevel(c.Level + 1)
	// Math.max(floor, character.experience || 0)
	current := c.Experience
	if current < floor {
		current = floor
	}
	remaining := required - current
	if remaining < 0 {
		remaining = 0
	}
	// Math.max(0, Math.min(1, (current - floor) / (required - floor)));
	// math.Min/math.Max propagate NaN exactly like the JS Math functions.
	progress := math.Max(0, math.Min(1, float64(current-floor)/float64(required-floor)))
	return XPProgress{Current: current, Required: required, Remaining: remaining, Ready: current >= required, Progress: progress}
}

// GrantExperience mirrors advancement.ts grantExperience. amount is already an
// int, so the TS Math.floor(amount) is a no-op here.
func GrantExperience(c Character, amount int) Character {
	// character.experience || experienceForLevel(character.level)
	base := c.Experience
	if base == 0 {
		base = ExperienceForLevel(c.Level)
	}
	if amount < 0 {
		amount = 0 // Math.max(0, Math.floor(amount))
	}
	next := base + amount
	if next > 9999999 {
		next = 9999999 // Math.min(9_999_999, ...)
	}
	if next < 0 {
		next = 0 // Math.max(0, ...)
	}
	c.Experience = next
	return c
}

// normalizedClasses mirrors advancement.ts normalizedClasses: the multiclass
// breakdown, falling back to a single entry synthesized from the top-level
// class fields. Like the TS version it returns the existing slice when
// classLevels is populated — treat the result as read-only.
func normalizedClasses(c Character) []ClassLevel {
	if len(c.ClassLevels) > 0 {
		return c.ClassLevels
	}
	return []ClassLevel{{ClassName: c.ClassName, Level: c.Level, Subclass: c.Subclass}}
}

// casterLevel mirrors advancement.ts casterLevel: full casters contribute
// their whole level, half casters contribute Math.ceil(level / 2), capped at 20.
func casterLevel(classes []ClassLevel) int {
	total := 0
	for _, entry := range classes {
		switch {
		case fullCasters[entry.ClassName]:
			total += entry.Level
		case halfCasters[entry.ClassName]:
			total += int(math.Ceil(float64(entry.Level) / 2))
		}
	}
	if total > 20 {
		total = 20 // Math.min(20, ...)
	}
	return total
}

// attackDamageDiePattern mirrors the advancement.ts damage-die prefix regex
// /^\d+d\d+/ (both JS and RE2 \d are ASCII digits).
var attackDamageDiePattern = regexp.MustCompile(`^\d+d\d+`)

// dexterousAttackProperty mirrors the advancement.ts property test
// /靈巧|彈藥|遠程/.test(value).
func dexterousAttackProperty(value string) bool {
	return strings.Contains(value, "靈巧") || strings.Contains(value, "彈藥") || strings.Contains(value, "遠程")
}

// Recalculate ports the private advancement.ts recalculate(): it rebuilds
// every derived stat (proficiency, max HP, skill bonuses, attack bonuses,
// passive perception, initiative, and spell slots) from level, classes, and
// ability scores. Exported because the Go game service applies it after
// server-side state changes.
func Recalculate(c Character) Character {
	proficiencyBonus := proficiencyForLevel(c.Level)
	con := AbilityModifier(c.Abilities.Con)
	primary := normalizedClasses(c)[0]
	// classDefinitions[primary.className]?.hitDie || character.hitDie — the
	// zero-value ClassDefinition of a missing key yields HitDie 0, which the
	// || sends to the character's stored hit die.
	hitDie := ClassDefinitions[primary.ClassName].HitDie
	if hitDie == 0 {
		hitDie = c.HitDie
	}
	average := int(math.Floor(float64(hitDie)/2)) + 1
	perLevel := average + con
	if perLevel < 1 {
		perLevel = 1 // Math.max(1, average + con)
	}
	extraLevels := c.Level - 1
	if extraLevels < 0 {
		extraLevels = 0 // Math.max(0, character.level - 1)
	}
	maxHP := hitDie + con + extraLevels*perLevel
	if maxHP < c.Level {
		maxHP = c.Level // Math.max(character.level, ...)
	}
	skills := make([]Skill, len(c.Skills))
	for i, skill := range c.Skills {
		bonus := AbilityModifier(c.Abilities.Get(skill.Ability))
		if skill.Expertise {
			bonus += proficiencyBonus * 2
		} else if skill.Proficient {
			bonus += proficiencyBonus
		}
		skill.Bonus = bonus
		skills[i] = skill
	}
	attacks := make([]Attack, len(c.Attacks))
	for i, entry := range c.Attacks {
		dexterity := false
		for _, value := range entry.Properties {
			if dexterousAttackProperty(value) {
				dexterity = true
				break
			}
		}
		ability := AbilityKey("str")
		if dexterity {
			ability = "dex"
		}
		modifier := AbilityModifier(c.Abilities.Get(ability))
		// attack.damage.match(/^\d+d\d+/)?.[0] || attack.damage
		diePart := attackDamageDiePattern.FindString(entry.Damage)
		if diePart == "" {
			diePart = entry.Damage
		}
		entry.AttackBonus = proficiencyBonus + modifier
		// `${diePart}${modifier >= 0 ? '+' : ''}${modifier}`
		entry.Damage = fmt.Sprintf("%s%+d", diePart, modifier)
		attacks[i] = entry
	}
	// skills.find((skill) => skill.name === '察覺')?.bonus || abilityModifier(wis)
	// The || means a found-but-zero bonus also falls back to the wis modifier.
	perception := AbilityModifier(c.Abilities.Wis)
	for _, skill := range skills {
		if skill.Name == "察覺" {
			if skill.Bonus != 0 {
				perception = skill.Bonus
			}
			break
		}
	}
	var spellcasting *Spellcasting
	if c.Spellcasting != nil {
		casting := *c.Spellcasting
		level := casterLevel(normalizedClasses(c))
		// slotTable[level] || []
		var slotRow []int
		if level >= 0 && level < len(slotTable) {
			slotRow = slotTable[level]
		}
		castingModifier := AbilityModifier(c.Abilities.Get(casting.Ability))
		casting.AttackBonus = proficiencyBonus + castingModifier
		casting.SaveDC = 8 + proficiencyBonus + castingModifier
		if casting.Mode == "pact" {
			slotLevel := int(math.Ceil(float64(c.Level) / 2))
			if slotLevel < 1 {
				slotLevel = 1
			}
			if slotLevel > 5 {
				slotLevel = 5 // Math.min(5, Math.max(1, Math.ceil(level / 2)))
			}
			max := 1 + int(math.Floor(float64(c.Level)/5))
			if max > 4 {
				max = 4 // Math.min(4, 1 + Math.floor(level / 5))
			}
			// casting.slots[0]?.current || 0
			current := 0
			if len(c.Spellcasting.Slots) > 0 {
				current = c.Spellcasting.Slots[0].Current
			}
			if current > max {
				current = max // Math.min(current, max)
			}
			casting.Slots = []SlotPool{{Level: slotLevel, Current: current, Max: max}}
		} else {
			slots := make([]SlotPool, len(slotRow))
			for index, max := range slotRow {
				// casting.slots.find((slot) => slot.level === index + 1)?.current ?? max
				current := max
				for _, slot := range c.Spellcasting.Slots {
					if slot.Level == index+1 {
						current = slot.Current
						break
					}
				}
				if current > max {
					current = max // Math.min(current, max)
				}
				slots[index] = SlotPool{Level: index + 1, Current: current, Max: max}
			}
			casting.Slots = slots
		}
		spellcasting = &casting
	}
	classes := normalizedClasses(c)
	names := make([]string, len(classes))
	for i, entry := range classes {
		names[i] = entry.ClassName
	}
	var subclasses []string
	for _, entry := range classes {
		if entry.Subclass != "" { // .filter(Boolean)
			subclasses = append(subclasses, entry.Subclass)
		}
	}
	c.ClassName = strings.Join(names, "／")
	c.Subclass = strings.Join(subclasses, "／")
	c.ProficiencyBonus = proficiencyBonus
	c.HitDie = hitDie
	c.MaxHP = maxHP
	if c.HP > maxHP {
		c.HP = maxHP // Math.min(character.hp, maxHp)
	}
	if c.HitDice > c.Level {
		c.HitDice = c.Level // Math.min(character.hitDice, character.level)
	}
	c.MaxHitDice = c.Level
	c.Passive = 10 + perception
	c.Initiative = AbilityModifier(c.Abilities.Dex)
	c.Skills = skills
	c.Attacks = attacks
	c.Spellcasting = spellcasting
	return c
}

// BuildOptions mirrors the advancement.ts CharacterBuildOptions interface.
// Zero values stand in for the TS undefined: Level 0 defaults to 3, empty
// Species/Background keep the class template defaults, nil Abilities keeps
// the template array.
type BuildOptions struct {
	Level      int
	Species    string
	Background string
	Abilities  *AbilityScores
}

// CreateConfiguredCharacter mirrors advancement.ts createConfiguredCharacter:
// a level-3 template re-leveled to opts.Level with optional identity and
// ability overrides, then recalculated. Like the TS source, the classLevels
// entry keeps the caller-supplied className even when the template lookup
// fell back to 戰士.
func CreateConfiguredCharacter(id, name, className string, opts BuildOptions) Character {
	base := CreateLevel3Character(id, name, className)
	// Math.min(20, Math.max(1, options.level || 3))
	level := opts.Level
	if level == 0 {
		level = 3
	}
	if level < 1 {
		level = 1
	}
	if level > 20 {
		level = 20
	}
	c := base
	c.Level = level
	c.ClassLevels = []ClassLevel{{ClassName: className, Level: level, Subclass: base.Subclass}}
	// options.species?.trim() || base.species
	if species := strings.TrimFunc(opts.Species, isJSWhitespace); species != "" {
		c.Species = species
	}
	if background := strings.TrimFunc(opts.Background, isJSWhitespace); background != "" {
		c.Background = background
	}
	if opts.Abilities != nil {
		c.Abilities = *opts.Abilities
	}
	c.HitDice = level
	c.MaxHitDice = level
	c.Experience = ExperienceForLevel(level)
	c.AbilityPoints = 0
	return Recalculate(c)
}

// CustomizeCharacter mirrors advancement.ts customizeCharacter, which spreads
// a partial patch over the character and recalculates. The TS patch keys are
// optional; here the zero values (empty string, nil) mean "not provided" and
// keep the existing field.
func CustomizeCharacter(c Character, species, background string, abilities *AbilityScores) Character {
	if species != "" {
		c.Species = species
	}
	if background != "" {
		c.Background = background
	}
	if abilities != nil {
		c.Abilities = *abilities
	}
	return Recalculate(c)
}

// mergeByID mirrors the advancement.ts mergeById helper: left plus every right
// entry whose id is not already present in left (duplicates within right are
// all kept, exactly like the TS filter over left only).
func mergeByID[T any](left, right []T, id func(T) string) []T {
	merged := append(make([]T, 0, len(left)+len(right)), left...)
	for _, candidate := range right {
		exists := false
		for _, entry := range left {
			if id(entry) == id(candidate) {
				exists = true
				break
			}
		}
		if !exists {
			merged = append(merged, candidate)
		}
	}
	return merged
}

// LevelUpCharacter mirrors advancement.ts levelUpCharacter: it adds one level
// to className (multiclassing when new), grants HP, merges the starter kit of
// the class, records a progression feature, and awards ability points at the
// improvement levels. The TS throws map to errors with identical messages.
func LevelUpCharacter(c Character, className string) (Character, error) {
	if c.Level >= 20 {
		return Character{}, errors.New("角色總等級已達 20。")
	}
	if progress := ExperienceToNextLevel(c); !progress.Ready {
		return Character{}, fmt.Errorf("尚缺 %d XP 才能升級。", progress.Remaining)
	}
	classes := normalizedClasses(c)
	existingIndex := -1
	for i, entry := range classes {
		if entry.ClassName == className {
			existingIndex = i
			break
		}
	}
	starter := CreateLevel3Character(c.ID, c.Name, className)
	var classLevels []ClassLevel
	if existingIndex >= 0 {
		classLevels = make([]ClassLevel, len(classes))
		for i, entry := range classes {
			if entry.ClassName == className {
				entry.Level++
			}
			classLevels[i] = entry
		}
	} else {
		classLevels = append(append(make([]ClassLevel, 0, len(classes)+1), classes...), ClassLevel{ClassName: className, Level: 1, Subclass: starter.Subclass})
	}
	nextLevel := c.Level + 1
	// (existing?.level || 0) + 1
	nextClassLevel := 1
	if existingIndex >= 0 {
		nextClassLevel = classes[existingIndex].Level + 1
	}
	progressionFeature := Feature{
		ID:   fmt.Sprintf("progression-%s-%d", className, nextClassLevel),
		Name: fmt.Sprintf("%s %d 級進展", className, nextClassLevel),
	}
	progressionFeature.Description = fmt.Sprintf("解鎖 %s 第 %d 級進展與 %d 點能力值提升（角色成長頁分配）；生命值、熟練加值、攻擊與法術位依新等級重新計算。", className, nextClassLevel, abilityPointsPerLevel)
	// Math.max(1, Math.floor(starter.hitDie / 2) + 1 + conModifier); the
	// starter hit die is always positive, so truncation matches Math.floor.
	hpGain := starter.HitDie/2 + 1 + AbilityModifier(c.Abilities.Con)
	if hpGain < 1 {
		hpGain = 1
	}
	c.Level = nextLevel
	c.ClassLevels = classLevels
	c.HP += hpGain
	c.HitDice++
	c.Attacks = mergeByID(c.Attacks, starter.Attacks, func(a Attack) string { return a.ID })
	c.Resources = mergeByID(c.Resources, starter.Resources, func(r Resource) string { return r.ID })
	featureID := func(f Feature) string { return f.ID }
	c.Features = mergeByID(mergeByID(c.Features, starter.Features, featureID), []Feature{progressionFeature}, featureID)
	if c.Spellcasting == nil {
		c.Spellcasting = starter.Spellcasting
	}
	c.AbilityPoints += abilityPointsPerLevel
	return Recalculate(c), nil
}

// SpendAbilityPoint mirrors advancement.ts spendAbilityPoint. The TS throws
// map to errors with identical messages (the second interpolates the ability
// key, e.g. "str 已達一般上限 20。").
func SpendAbilityPoint(c Character, ability AbilityKey) (Character, error) {
	points := c.AbilityPoints // character.abilityPoints || 0
	if points < 1 {
		return Character{}, errors.New("目前沒有可分配的能力值點數。")
	}
	if c.Abilities.Get(ability) >= 20 {
		return Character{}, fmt.Errorf("%s 已達一般上限 20。", ability)
	}
	c.Abilities = c.Abilities.Set(ability, c.Abilities.Get(ability)+1)
	c.AbilityPoints = points - 1
	return Recalculate(c), nil
}

// SetPreparedSpells mirrors advancement.ts setPreparedSpells: it rebuilds the
// spell list in spell-catalog order (Object.keys(spellCatalog) → SpellIDs)
// from the selected ids plus any existing always-prepared spells, keeping the
// character's existing spell instances where present.
func SetPreparedSpells(c Character, spellIDs []string) Character {
	if c.Spellcasting == nil {
		return c
	}
	selected := make(map[string]struct{}, len(spellIDs))
	for _, id := range spellIDs {
		selected[id] = struct{}{}
	}
	// new Map(spells.map((spell) => [spell.id, spell])): later duplicates win.
	existing := make(map[string]Spell, len(c.Spellcasting.Spells))
	for _, spell := range c.Spellcasting.Spells {
		existing[spell.ID] = spell
	}
	spells := make([]Spell, 0, len(SpellIDs))
	for _, id := range SpellIDs {
		_, isSelected := selected[id]
		spell, hasExisting := existing[id]
		if !isSelected && !(hasExisting && spell.AlwaysPrepared) {
			continue
		}
		if hasExisting {
			spells = append(spells, spell)
		} else {
			spells = append(spells, mustMakeSpell(id, SpellOverrides{Prepared: boolPtr(true), InSpellbook: true}))
		}
	}
	casting := *c.Spellcasting
	casting.Spells = spells
	c.Spellcasting = &casting
	return c
}

// GetCharacterClasses mirrors advancement.ts getCharacterClasses. Like the TS
// version it may return the character's own classLevels slice — read-only.
func GetCharacterClasses(c Character) []ClassLevel {
	return normalizedClasses(c)
}
