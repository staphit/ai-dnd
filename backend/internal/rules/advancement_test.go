package rules

// Ported 1:1 from frontend/src/rules/advancement.test.ts ("character
// advancement" suite); every vitest assertion is preserved.

import (
	"strings"
	"testing"
)

// hasClassLevel mirrors expect.arrayContaining(expect.objectContaining({...}))
// over classLevels: some entry matches both className and level.
func hasClassLevel(classLevels []ClassLevel, className string, level int) bool {
	for _, entry := range classLevels {
		if entry.ClassName == className && entry.Level == level {
			return true
		}
	}
	return false
}

// TestCreateConfiguredCharacterCustomIdentityAndScores ports "creates
// characters from level 1 through 20 with custom identity and scores".
func TestCreateConfiguredCharacterCustomIdentityAndScores(t *testing.T) {
	abilities := AbilityScores{Str: 18, Dex: 12, Con: 16, Int: 10, Wis: 13, Cha: 8}
	character := CreateConfiguredCharacter("player1", "黎恩", "戰士", BuildOptions{
		Level:      10,
		Species:    "自訂星裔",
		Background: "鐘塔守望者",
		Abilities:  &abilities,
	})
	if character.Level != 10 {
		t.Errorf("level = %d, want 10", character.Level)
	}
	if character.ProficiencyBonus != 4 {
		t.Errorf("proficiencyBonus = %d, want 4", character.ProficiencyBonus)
	}
	if character.Species != "自訂星裔" {
		t.Errorf("species = %q, want %q", character.Species, "自訂星裔")
	}
	if character.MaxHitDice != 10 {
		t.Errorf("maxHitDice = %d, want 10", character.MaxHitDice)
	}
}

// TestLevelUpCharacterMulticlass ports "adds a multiclass level without
// replacing the existing class".
func TestLevelUpCharacterMulticlass(t *testing.T) {
	fighter := CreateConfiguredCharacter("player1", "黎恩", "戰士", BuildOptions{})
	fighter.Experience = 2700
	multiclass, err := LevelUpCharacter(fighter, "法師")
	if err != nil {
		t.Fatalf("levelUpCharacter returned error: %v", err)
	}
	if multiclass.Level != 4 {
		t.Errorf("level = %d, want 4", multiclass.Level)
	}
	if !hasClassLevel(multiclass.ClassLevels, "戰士", 3) {
		t.Errorf("classLevels = %+v, want an entry {className: 戰士, level: 3}", multiclass.ClassLevels)
	}
	if !hasClassLevel(multiclass.ClassLevels, "法師", 1) {
		t.Errorf("classLevels = %+v, want an entry {className: 法師, level: 1}", multiclass.ClassLevels)
	}
	if multiclass.Spellcasting == nil {
		t.Error("spellcasting = nil, want defined")
	}
}

// TestSetPreparedSpellsExplicitConfiguration ports "allows explicit spell
// configuration".
func TestSetPreparedSpellsExplicitConfiguration(t *testing.T) {
	wizard := CreateConfiguredCharacter("player1", "米拉", "法師", BuildOptions{})
	configured := SetPreparedSpells(wizard, []string{"light", "shield", "misty_step"})
	if configured.Spellcasting == nil {
		t.Fatal("spellcasting = nil, want defined")
	}
	ids := make(map[string]bool, len(configured.Spellcasting.Spells))
	for _, spell := range configured.Spellcasting.Spells {
		ids[spell.ID] = true
	}
	for _, want := range []string{"light", "shield", "misty_step"} {
		if !ids[want] {
			t.Errorf("spell ids missing %q (got %v)", want, ids)
		}
	}
}

// TestLevelUpRequiresXPAndGrantsAbilityPoints ports "requires XP to level and
// grants ability points at level 4".
func TestLevelUpRequiresXPAndGrantsAbilityPoints(t *testing.T) {
	fighter := CreateConfiguredCharacter("player1", "黎恩", "戰士", BuildOptions{})
	if ExperienceToNextLevel(fighter).Ready {
		t.Error("experienceToNextLevel(fighter).ready = true, want false")
	}
	// expect(() => levelUpCharacter(fighter, '戰士')).toThrow(/XP/)
	if _, err := LevelUpCharacter(fighter, "戰士"); err == nil {
		t.Error("levelUpCharacter without XP: got nil error, want error matching /XP/")
	} else if !strings.Contains(err.Error(), "XP") {
		t.Errorf("levelUpCharacter without XP: error %q does not match /XP/", err.Error())
	}
	ready := GrantExperience(fighter, 1800)
	leveled, err := LevelUpCharacter(ready, "戰士")
	if err != nil {
		t.Fatalf("levelUpCharacter returned error: %v", err)
	}
	if leveled.Level != 4 {
		t.Errorf("level = %d, want 4", leveled.Level)
	}
	// House rule: every level-up grants abilityPointsPerLevel (5) points.
	if leveled.AbilityPoints != abilityPointsPerLevel {
		t.Errorf("abilityPoints = %d, want %d", leveled.AbilityPoints, abilityPointsPerLevel)
	}
	improved, err := SpendAbilityPoint(leveled, "str")
	if err != nil {
		t.Fatalf("spendAbilityPoint returned error: %v", err)
	}
	if improved.Abilities.Str != leveled.Abilities.Str+1 {
		t.Errorf("str = %d, want %d", improved.Abilities.Str, leveled.Abilities.Str+1)
	}
	if improved.AbilityPoints != abilityPointsPerLevel-1 {
		t.Errorf("abilityPoints = %d, want %d", improved.AbilityPoints, abilityPointsPerLevel-1)
	}
}
