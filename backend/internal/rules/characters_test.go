package rules

// Ported 1:1 from frontend/src/rules/characters.test.ts ("2024 level 3 class
// rules"), preserving every vitest assertion.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// findCharacterSpell mirrors the tests' spells.find((spell) => spell.id === id).
func findCharacterSpell(t *testing.T, c Character, id string) Spell {
	t.Helper()
	if c.Spellcasting == nil {
		t.Fatalf("expected %s to have spellcasting", c.ClassName)
	}
	for _, spell := range c.Spellcasting.Spells {
		if spell.ID == id {
			return spell
		}
	}
	t.Fatalf("expected %s to know spell %s", c.ClassName, id)
	return Spell{}
}

// mustSpendSpellSlot mirrors the tests' non-null assertion on spendSpellSlot(...)!.
func mustSpendSpellSlot(t *testing.T, c Character, spell Spell, asRitual bool) Character {
	t.Helper()
	next, ok := SpendSpellSlot(c, spell, asRitual)
	if !ok {
		t.Fatalf("expected spendSpellSlot(%s) to succeed", spell.ID)
	}
	return next
}

// it('builds all 6 classes with a complete common character sheet')
func TestBuildsAll6ClassesWithCompleteCommonSheet(t *testing.T) {
	if len(ClassNames) != 6 {
		t.Fatalf("expected 6 class names, got %d", len(ClassNames))
	}
	for index, className := range ClassNames {
		character := CreateLevel3Character(fmt.Sprintf("player%d", index+1), "測試"+className, className)
		if character.ClassName != className {
			t.Errorf("%s: className = %q", className, character.ClassName)
		}
		if character.Subclass == "" {
			t.Errorf("%s: subclass is empty", className)
		}
		if character.Level != 3 {
			t.Errorf("%s: level = %d, want 3", className, character.Level)
		}
		if character.ProficiencyBonus != 2 {
			t.Errorf("%s: proficiencyBonus = %d, want 2", className, character.ProficiencyBonus)
		}
		// expect(Object.keys(character.abilities)).toHaveLength(6)
		raw, err := json.Marshal(character.Abilities)
		if err != nil {
			t.Fatalf("%s: marshal abilities: %v", className, err)
		}
		abilityKeys := map[string]int{}
		if err := json.Unmarshal(raw, &abilityKeys); err != nil {
			t.Fatalf("%s: unmarshal abilities: %v", className, err)
		}
		if len(abilityKeys) != 6 {
			t.Errorf("%s: abilities has %d keys, want 6", className, len(abilityKeys))
		}
		if character.MaxHP <= 0 {
			t.Errorf("%s: maxHp = %d, want > 0", className, character.MaxHP)
		}
		if character.AC < 10 {
			t.Errorf("%s: ac = %d, want >= 10", className, character.AC)
		}
		if len(character.Skills) != 18 {
			t.Errorf("%s: skills length = %d, want 18", className, len(character.Skills))
		}
		if len(character.Attacks) == 0 {
			t.Errorf("%s: attacks is empty", className)
		}
		if len(character.Features) == 0 {
			t.Errorf("%s: features is empty", className)
		}
		if len(character.Equipment) == 0 {
			t.Errorf("%s: equipment is empty", className)
		}
	}
}

// it.each(['吟遊詩人', '牧師', '法師'])('%s has 4 first-level and 2 second-level slots')
func TestFullCastersHaveFourFirstAndTwoSecondLevelSlots(t *testing.T) {
	for _, className := range []string{"吟遊詩人", "牧師", "法師"} {
		t.Run(className, func(t *testing.T) {
			character := CreateLevel3Character("player1", "施法者", className)
			if character.Spellcasting == nil {
				t.Fatalf("expected %s to have spellcasting", className)
			}
			want := []SlotPool{
				{Level: 1, Current: 4, Max: 4},
				{Level: 2, Current: 2, Max: 2},
			}
			if !reflect.DeepEqual(character.Spellcasting.Slots, want) {
				t.Errorf("slots = %+v, want %+v", character.Spellcasting.Slots, want)
			}
		})
	}
}

// it('models half casters and Pact Magic separately')
func TestModelsHalfCastersAndPactMagicSeparately(t *testing.T) {
	for _, className := range []string{"聖武士"} {
		character := CreateLevel3Character("player1", "半施法者", className)
		if character.Spellcasting == nil {
			t.Fatalf("expected %s to have spellcasting", className)
		}
		want := []SlotPool{{Level: 1, Current: 3, Max: 3}}
		if !reflect.DeepEqual(character.Spellcasting.Slots, want) {
			t.Errorf("%s: slots = %+v, want %+v", className, character.Spellcasting.Slots, want)
		}
		if character.Spellcasting.Mode != "standard" {
			t.Errorf("%s: mode = %q, want standard", className, character.Spellcasting.Mode)
		}
	}
	warlock := CreateLevel3Character("player1", "契約者", "魔契師")
	if warlock.Spellcasting == nil {
		t.Fatal("expected 魔契師 to have spellcasting")
	}
	if warlock.Spellcasting.Mode != "pact" {
		t.Errorf("mode = %q, want pact", warlock.Spellcasting.Mode)
	}
	if warlock.Spellcasting.PactSlotLevel != 2 {
		t.Errorf("pactSlotLevel = %d, want 2", warlock.Spellcasting.PactSlotLevel)
	}
	want := []SlotPool{{Level: 2, Current: 2, Max: 2}}
	if !reflect.DeepEqual(warlock.Spellcasting.Slots, want) {
		t.Errorf("slots = %+v, want %+v", warlock.Spellcasting.Slots, want)
	}
}

// it('gives the Evoker cantrips, a twelve-spell book, six prepared spells, and Arcane Recovery')
func TestEvokerCantripsSpellbookPreparedAndArcaneRecovery(t *testing.T) {
	wizard := CreateLevel3Character("player1", "梅林", "法師")
	var spells []Spell
	if wizard.Spellcasting != nil {
		spells = wizard.Spellcasting.Spells
	}
	cantrips := 0
	inSpellbook := 0
	prepared := 0
	for _, spell := range spells {
		if spell.Level == 0 {
			cantrips++
		}
		if spell.InSpellbook {
			inSpellbook++
		}
		if spell.Level > 0 && spell.Prepared {
			prepared++
		}
	}
	if cantrips != 4 {
		t.Errorf("cantrip count = %d, want 4", cantrips)
	}
	if inSpellbook != 12 {
		t.Errorf("spellbook count = %d, want 12", inSpellbook)
	}
	if prepared != 6 {
		t.Errorf("prepared leveled spell count = %d, want 6", prepared)
	}
	arcaneRecovery := -1
	for _, entry := range wizard.Resources {
		if entry.ID == "arcane_recovery" {
			arcaneRecovery = entry.Current
			break
		}
	}
	if arcaneRecovery != 1 {
		t.Errorf("arcane_recovery current = %d, want 1", arcaneRecovery)
	}
	if wizard.Spellcasting == nil || wizard.Spellcasting.Ability != "int" {
		t.Errorf("spellcasting ability = %v, want int", wizard.Spellcasting)
	}
}

// it('preserves spell attack and target rules on character spells')
func TestPreservesSpellAttackAndTargetRules(t *testing.T) {
	warlock := CreateLevel3Character("player1", "契約者", "魔契師")
	blast := findCharacterSpell(t, warlock, "eldritch_blast")
	// expect(blast?.effect).toMatchObject({ target: 'creature', attackRoll: true, dice: '1d10' })
	if blast.Effect == nil {
		t.Fatal("eldritch_blast has no effect")
	}
	if blast.Effect.Target != "creature" {
		t.Errorf("effect.target = %q, want creature", blast.Effect.Target)
	}
	if !blast.Effect.AttackRoll {
		t.Error("effect.attackRoll = false, want true")
	}
	if blast.Effect.Dice != "1d10" {
		t.Errorf("effect.dice = %q, want 1d10", blast.Effect.Dice)
	}
}

// it('uses the paladin free Divine Smite before spending a spell slot')
func TestPaladinFreeDivineSmiteBeforeSlot(t *testing.T) {
	paladin := CreateLevel3Character("player1", "聖騎士", "聖武士")
	smite := findCharacterSpell(t, paladin, "divine_smite")
	afterFreeSmite := mustSpendSpellSlot(t, paladin, smite, false)
	freeSmite := -1
	for _, entry := range afterFreeSmite.Resources {
		if entry.ID == "free_divine_smite" {
			freeSmite = entry.Current
			break
		}
	}
	if freeSmite != 0 {
		t.Errorf("free_divine_smite current = %d, want 0", freeSmite)
	}
	if got := afterFreeSmite.Spellcasting.Slots[0].Current; got != 3 {
		t.Errorf("slots[0].current after free smite = %d, want 3", got)
	}
	afterPaidSmite := mustSpendSpellSlot(t, afterFreeSmite, smite, false)
	if got := afterPaidSmite.Spellcasting.Slots[0].Current; got != 2 {
		t.Errorf("slots[0].current after paid smite = %d, want 2", got)
	}
}

// it('spends a slot, tracks concentration, and never allows negative slots')
func TestSpendsSlotTracksConcentrationNoNegativeSlots(t *testing.T) {
	wizard := CreateLevel3Character("player1", "梅林", "法師")
	sleep := findCharacterSpell(t, wizard, "sleep") // expect(sleep).toBeDefined()
	afterCast := mustSpendSpellSlot(t, wizard, sleep, false)
	if got := afterCast.Spellcasting.Slots[0].Current; got != 3 {
		t.Errorf("slots[0].current = %d, want 3", got)
	}
	if afterCast.Concentration != "睡眠術" {
		t.Errorf("concentration = %q, want 睡眠術", afterCast.Concentration)
	}

	exhausted := afterCast
	exhausted = mustSpendSpellSlot(t, exhausted, sleep, false)
	exhausted = mustSpendSpellSlot(t, exhausted, sleep, false)
	exhausted = mustSpendSpellSlot(t, exhausted, sleep, false)
	if got := exhausted.Spellcasting.Slots[0].Current; got != 0 {
		t.Errorf("slots[0].current after exhaustion = %d, want 0", got)
	}
	overflow := mustSpendSpellSlot(t, exhausted, sleep, false)
	if got := overflow.Spellcasting.Slots[1].Current; got != 1 {
		t.Errorf("slots[1].current after overflow cast = %d, want 1", got)
	}
}

// it('recovers Pact Magic on a short rest and all resources on a long rest')
func TestRecoversPactMagicOnShortRestAndAllOnLongRest(t *testing.T) {
	warlock := CreateLevel3Character("player1", "契約者", "魔契師")
	hex := findCharacterSpell(t, warlock, "hex")
	spent := mustSpendSpellSlot(t, warlock, hex, false)
	if got := spent.Spellcasting.Slots[0].Current; got != 1 {
		t.Errorf("slots[0].current after hex = %d, want 1", got)
	}
	if got := RestCharacter(spent, "short").Spellcasting.Slots[0].Current; got != 2 {
		t.Errorf("slots[0].current after short rest = %d, want 2", got)
	}

	wounded := spent
	wounded.HP = 1
	wounded.Condition = "中毒"
	wounded.HitDice = 0
	rested := RestCharacter(wounded, "long")
	if rested.HP != rested.MaxHP {
		t.Errorf("hp = %d, want maxHp %d", rested.HP, rested.MaxHP)
	}
	if rested.HitDice != rested.MaxHitDice {
		t.Errorf("hitDice = %d, want maxHitDice %d", rested.HitDice, rested.MaxHitDice)
	}
	if rested.Condition != "正常" {
		t.Errorf("condition = %q, want 正常", rested.Condition)
	}
}
