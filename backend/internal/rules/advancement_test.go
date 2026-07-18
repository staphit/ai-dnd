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

// TestLevelUpCharacterNoMulticlass: level-ups may only advance the
// character's own class; asking for another class errors, and the own-class
// (or empty) form advances normally.
func TestLevelUpCharacterNoMulticlass(t *testing.T) {
	fighter := CreateConfiguredCharacter("player1", "黎恩", "戰士", BuildOptions{})
	fighter.Experience = 2700
	if _, err := LevelUpCharacter(fighter, "法師"); err == nil {
		t.Fatal("multiclassing should be rejected")
	}
	leveled, err := LevelUpCharacter(fighter, "")
	if err != nil {
		t.Fatalf("own-class level up: %v", err)
	}
	if leveled.Level != 4 || !hasClassLevel(leveled.ClassLevels, "戰士", 4) {
		t.Errorf("level = %d classLevels = %+v, want 戰士 4", leveled.Level, leveled.ClassLevels)
	}
}

// TestSpendAbilityPointNoCap: banked points may push abilities past 20.
func TestSpendAbilityPointNoCap(t *testing.T) {
	c := CreateConfiguredCharacter("player1", "黎恩", "戰士", BuildOptions{})
	c.Abilities = c.Abilities.Set("str", 20)
	c.AbilityPoints = 2
	next, err := SpendAbilityPoint(c, "str")
	if err != nil {
		t.Fatalf("spend past 20: %v", err)
	}
	if next.Abilities.Get("str") != 21 || next.AbilityPoints != 1 {
		t.Fatalf("str = %d points = %d, want 21 / 1", next.Abilities.Get("str"), next.AbilityPoints)
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

// TestDexRaisesAC: raising DEX must raise AC for light-armor classes; heavy
// armor ignores DEX entirely.
func TestDexRaisesAC(t *testing.T) {
	bard := CreateConfiguredCharacter("p1", "小影", "吟遊詩人", BuildOptions{})
	base := bard.AC
	bard.AbilityPoints = 2
	step1, err := SpendAbilityPoint(bard, "dex")
	if err != nil {
		t.Fatalf("spend 1: %v", err)
	}
	step2, err := SpendAbilityPoint(step1, "dex")
	if err != nil {
		t.Fatalf("spend 2: %v", err)
	}
	wantDelta := AbilityModifier(step2.Abilities.Dex) - AbilityModifier(bard.Abilities.Dex)
	if wantDelta < 1 {
		t.Fatalf("test setup: +2 DEX should raise the modifier (dex %d -> %d)", bard.Abilities.Dex, step2.Abilities.Dex)
	}
	if step2.AC != base+wantDelta {
		t.Fatalf("AC did not follow DEX: base %d, got %d, want %d", base, step2.AC, base+wantDelta)
	}

	fighter := CreateConfiguredCharacter("p2", "鐵手", "戰士", BuildOptions{})
	fBase := fighter.AC
	fighter.AbilityPoints = 2
	f1, _ := SpendAbilityPoint(fighter, "dex")
	f2, _ := SpendAbilityPoint(f1, "dex")
	if f2.AC != fBase {
		t.Fatalf("heavy armor must ignore DEX: %d -> %d", fBase, f2.AC)
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
