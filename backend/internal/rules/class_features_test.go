package rules

// Tests for the server-side class feature mechanics added on top of the TS
// port: 精通重擊 (improved critical), 生命門徒 (disciple of life), 苦痛魔爆
// (agonizing blast), 強效戲法 (potent cantrip), and 萬事通 (jack of all
// trades).

import (
	"strings"
	"testing"
)

// TestFighterCritsOn19 verifies the 戰士 crit threshold: a natural 19 crits
// for a fighter combatant but stays a normal hit for everyone else.
func TestFighterCritsOn19(t *testing.T) {
	fighter := CreateLevel3Character("player1", "鐵手", "戰士")
	if fighter.CritThreshold != 19 {
		t.Fatalf("fighter critThreshold = %d, want 19", fighter.CritThreshold)
	}
	recalced := Recalculate(fighter)
	if recalced.CritThreshold != 19 {
		t.Fatalf("critThreshold lost in Recalculate: %d", recalced.CritThreshold)
	}
	cleric := CreateLevel3Character("player2", "米芮", "牧師")
	if cleric.CritThreshold != 0 {
		t.Fatalf("cleric critThreshold = %d, want 0 (default 20)", cleric.CritThreshold)
	}

	combatants := PartyCombatants([]Character{fighter})
	if combatants[0].CritThreshold != 19 {
		t.Fatalf("party combatant critThreshold = %d, want 19", combatants[0].CritThreshold)
	}
	state := CombatState{Active: true, Round: 1, TurnIndex: 0, Combatants: append(combatants, Combatant{
		ID: "goblin", Name: "哥布林", Side: "enemy", AC: 13, HP: 40, MaxHP: 40, AttackBonus: 4, Damage: "1d6+2", DamageType: "穿刺",
	})}

	// 0.9 → natural 19: crit for the fighter (threshold 19)…
	_, resolution, err := ResolveAttack(state, "player1", "goblin", seq(0.9, 0.5, 0.5), "normal")
	if err != nil {
		t.Fatalf("resolveAttack: %v", err)
	}
	if !resolution.Critical || !resolution.Hit {
		t.Fatalf("fighter natural 19 should crit: %+v", resolution)
	}
	// …and 0.999 → natural 20 still crits.
	_, resolution, err = ResolveAttack(state, "player1", "goblin", seq(0.999, 0.5, 0.5), "normal")
	if err != nil {
		t.Fatalf("resolveAttack: %v", err)
	}
	if !resolution.Critical {
		t.Fatalf("fighter natural 20 should crit: %+v", resolution)
	}
	// The enemy (default threshold) rolling 19 does not crit.
	state.TurnIndex = 1
	_, resolution, err = ResolveAttack(state, "goblin", "player1", seq(0.9, 0.5), "normal")
	if err != nil {
		t.Fatalf("resolveAttack: %v", err)
	}
	if resolution.Critical {
		t.Fatalf("default threshold natural 19 must not crit: %+v", resolution)
	}
}

// TestDiscipleOfLifeBoostsLeveledHealing verifies the 牧師（生命領域）healing
// bonus: cure wounds heals an extra 2 + spell level.
func TestDiscipleOfLifeBoostsLeveledHealing(t *testing.T) {
	cleric := CreateLevel3Character("player1", "米芮", "牧師")
	wounded := CreateLevel3Character("player2", "鐵手", "戰士")
	wounded.HP = 1
	cure := findCharacterSpell(t, cleric, "cure_wounds")
	if cure.Effect == nil {
		t.Fatal("cure_wounds has no effect")
	}
	// random 0 → each 2d8 die rolls 1: 2 + wis 3 = 5, +（2+1）門徒加值 = 8.
	result := mustResolveSpellEffect(t, []Character{cleric, wounded}, nil, cleric.ID, wounded.ID, *cure.Effect, func() float64 { return 0 }, nil)
	if result.Amount != 8 {
		t.Fatalf("disciple of life amount = %d, want 8", result.Amount)
	}
	if result.Players[1].HP != 9 {
		t.Fatalf("wounded hp = %d, want 9", result.Players[1].HP)
	}

	// A non-cleric casting the identical effect gets no bonus.
	bard := CreateLevel3Character("player1", "歌者", "吟遊詩人")
	plain := mustResolveSpellEffect(t, []Character{bard, wounded}, nil, bard.ID, wounded.ID, *cure.Effect, func() float64 { return 0 }, nil)
	if plain.Amount != 5 {
		t.Fatalf("non-cleric amount = %d, want 5", plain.Amount)
	}
}

// TestSneakAttackRiderWithAllyStanding verifies the 盜賊 偷襲 damage rider:
// it fires while another ally stands and stays quiet when the rogue is alone.
func TestSneakAttackRiderWithAllyStanding(t *testing.T) {
	rogue := CreateLevel3Character("player1", "影刃", "盜賊")
	if rogue.SneakAttackDice != 2 {
		t.Fatalf("sneakAttackDice = %d, want 2 at level 3", rogue.SneakAttackDice)
	}
	party := PartyCombatants([]Character{rogue, CreateLevel3Character("player2", "同伴", "戰士")})
	enemy := Combatant{ID: "enemy", Name: "敵人", Side: "enemy", AC: 5, HP: 30, MaxHP: 30, Damage: "1d4", DamageType: "鈍擊"}
	state := CombatState{Active: true, Round: 1, Combatants: append(party, enemy)}
	// random 0 → d20 rolls 1? Die maps 0 → 1; use 0.5 for a mid roll that hits AC 5.
	_, res, err := ResolveAttack(state, party[0].ID, "enemy", func() float64 { return 0.5 }, "normal")
	if err != nil {
		t.Fatalf("attack: %v", err)
	}
	// Weapon 1d6 mid-roll 4 + DEX bonus, plus 2d6 sneak (mid-roll 4 each).
	if res.Damage <= 0 || !strings.Contains(res.Text, "偷襲") {
		t.Fatalf("sneak rider missing: dmg=%d text=%q", res.Damage, res.Text)
	}

	// Solo rogue (ally defeated): no sneak rider.
	soloParty := PartyCombatants([]Character{rogue, CreateLevel3Character("player2", "同伴", "戰士")})
	soloParty[1].Defeated = true
	soloState := CombatState{Active: true, Round: 1, Combatants: append(soloParty, enemy)}
	_, soloRes, err := ResolveAttack(soloState, soloParty[0].ID, "enemy", func() float64 { return 0.5 }, "normal")
	if err != nil {
		t.Fatalf("solo attack: %v", err)
	}
	if strings.Contains(soloRes.Text, "偷襲") {
		t.Fatalf("solo rogue must not sneak attack: %q", soloRes.Text)
	}
}

// TestPotentCantripHalvesDamageOnSave verifies the 法師 potent cantrip: a
// damage cantrip still deals half damage when the target saves; other classes
// keep the save-negates default.
func TestPotentCantripHalvesDamageOnSave(t *testing.T) {
	wizard := CreateLevel3Character("player1", "梅林", "法師")
	wizard = SetPreparedSpells(wizard, []string{"sacred_flame"})
	flame := findCharacterSpell(t, wizard, "sacred_flame")
	if flame.Effect == nil || flame.Effect.SaveAbility == "" {
		t.Fatalf("sacred_flame should be a save-based cantrip: %+v", flame.Effect)
	}
	enemy := Combatant{ID: "enemy", Name: "敵人", Side: "enemy", AC: 12, HP: 20, MaxHP: 20, AttackBonus: 0, Damage: "1d4", DamageType: "鈍擊"}
	combat := &CombatState{Active: true, Round: 1, TurnIndex: 0, Combatants: []Combatant{enemy}}
	saved := 30
	// 0.99 → 1d8 rolls 8; the save succeeds, potent cantrip halves to 4.
	result := mustResolveSpellEffect(t, []Character{wizard}, combat, wizard.ID, enemy.ID, *flame.Effect, seq(0.99), &ForcedRolls{SaveTotal: &saved})
	if result.Amount != 4 {
		t.Fatalf("potent cantrip amount = %d, want 4", result.Amount)
	}
	if !strings.Contains(result.Text, "豁免成功") {
		t.Fatalf("text should mention the save: %q", result.Text)
	}

	// The cleric owns the same cantrip but has no potent cantrip: save negates.
	cleric := CreateLevel3Character("player1", "米芮", "牧師")
	negated := mustResolveSpellEffect(t, []Character{cleric}, combat, cleric.ID, enemy.ID, *flame.Effect, seq(0.99), &ForcedRolls{SaveTotal: &saved})
	if negated.Amount != 0 {
		t.Fatalf("non-wizard save should negate: %d", negated.Amount)
	}
}

// TestJackOfAllTradesAddsHalfProficiency verifies the 吟遊詩人 half
// proficiency bonus on unproficient checks.
func TestJackOfAllTradesAddsHalfProficiency(t *testing.T) {
	bard := CreateLevel3Character("player1", "歌者", "吟遊詩人")
	// 調查 is unproficient: int 10 → +0, plus floor(2/2) = 1.
	if got := GetCheckBonus(bard, "調查"); got != 1 {
		t.Fatalf("bard 調查 bonus = %d, want 1", got)
	}
	// Raw ability checks are unproficient too: 力量 8 → -1 + 1 = 0.
	if got := GetCheckBonus(bard, "力量"); got != 0 {
		t.Fatalf("bard 力量 bonus = %d, want 0", got)
	}
	// Proficient/expertise skills are unchanged: 表演 cha 17 → +3 + 4 = 7.
	if got := GetCheckBonus(bard, "表演"); got != 7 {
		t.Fatalf("bard 表演 bonus = %d, want 7", got)
	}
	// Non-bards get no half proficiency: fighter 調查 int 10 → +0.
	fighter := CreateLevel3Character("player2", "鐵手", "戰士")
	if got := GetCheckBonus(fighter, "調查"); got != 0 {
		t.Fatalf("fighter 調查 bonus = %d, want 0", got)
	}
}
