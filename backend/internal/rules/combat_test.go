package rules

// Ported 1:1 from frontend/src/rules/combat.test.ts. The rollExpression
// assertions target dice.go's RollExpression, where that helper now lives.

import (
	"strings"
	"testing"
)

// seq mirrors the vitest random fixture `() => rolls.shift() || 0`: it
// returns the queued rolls in order and 0 once exhausted.
func seq(rolls ...float64) RandomSource {
	index := 0
	return func() float64 {
		if index < len(rolls) {
			value := rolls[index]
			index++
			return value
		}
		return 0
	}
}

var combatFighter = Combatant{ID: "fighter", Name: "戰士", Side: "party", InitiativeBonus: 3, Initiative: 0, AC: 18, HP: 25, MaxHP: 25, AttackBonus: 6, Damage: "1d8+3", DamageType: "揮砍"}
var combatGoblin = Combatant{ID: "goblin", Name: "哥布林", Side: "enemy", InitiativeBonus: 2, Initiative: 0, AC: 13, HP: 12, MaxHP: 12, AttackBonus: 4, Damage: "1d6+2", DamageType: "穿刺"}

func TestLetsAnAmbushingEnemyTakeTheFirstStoryDirectedTurn(t *testing.T) {
	state := StartCombat([]Combatant{
		{ID: "hero", Name: "英雄", Side: "party", InitiativeBonus: 20, Initiative: 0, AC: 15, HP: 10, MaxHP: 10, AttackBonus: 3, Damage: "1d6", DamageType: "揮砍"},
		{ID: "beast", Name: "伏擊獸", Side: "enemy", InitiativeBonus: -5, Initiative: 0, AC: 12, HP: 8, MaxHP: 8, AttackBonus: 3, Damage: "1d4", DamageType: "穿刺"},
	}, func() float64 { return 0.5 }, "enemy")
	if got := state.Combatants[state.TurnIndex].ID; got != "beast" {
		t.Fatalf("first turn combatant id = %q, want %q", got, "beast")
	}
}

func TestRollsInitiativeAndWrapsRoundsInSequence(t *testing.T) {
	state := StartCombat([]Combatant{combatFighter, combatGoblin}, seq(0.9, 0.1), "initiative")
	if got := state.Combatants[0].Name; got != "戰士" {
		t.Fatalf("combatants[0].Name = %q, want %q", got, "戰士")
	}
	if got := AdvanceTurn(AdvanceTurn(state)).Round; got != 2 {
		t.Fatalf("round after two advances = %d, want 2", got)
	}
}

func TestTracksActionBonusActionAndReactionSeparatelyUntilTheTurnEnds(t *testing.T) {
	state := StartCombat([]Combatant{combatFighter, combatGoblin}, func() float64 { return 0.5 }, "initiative")
	actor := state.Combatants[state.TurnIndex]
	afterAction, err := SpendCombatResource(state, actor.ID, "action")
	if err != nil {
		t.Fatalf("spend action: %v", err)
	}
	afterBonus, err := SpendCombatResource(afterAction, actor.ID, CombatResourceForCastingTime("附贈動作"))
	if err != nil {
		t.Fatalf("spend bonus action: %v", err)
	}
	usage := afterBonus.TurnEconomy[actor.ID]
	if want := (TurnEconomy{ActionUsed: true, BonusActionUsed: true, ReactionUsed: false}); usage != want {
		t.Fatalf("turnEconomy[%s] = %+v, want %+v", actor.ID, usage, want)
	}
	if _, err := SpendCombatResource(afterBonus, actor.ID, "action"); err == nil || !strings.Contains(err.Error(), "已使用") {
		t.Fatalf("respending action: error = %v, want error containing 已使用", err)
	}
	next := AdvanceTurn(afterBonus)
	nextUsage := next.TurnEconomy[next.Combatants[next.TurnIndex].ID]
	if want := (TurnEconomy{ActionUsed: false, BonusActionUsed: false, ReactionUsed: false}); nextUsage != want {
		t.Fatalf("next actor turnEconomy = %+v, want %+v", nextUsage, want)
	}
}

func TestAutomaticallyResolvesHitAndDamageAgainstAC(t *testing.T) {
	state := CombatState{Active: true, Round: 1, TurnIndex: 0, Combatants: []Combatant{combatFighter, combatGoblin}}
	nextState, resolution, err := ResolveAttack(state, "fighter", "goblin", seq(0.7, 0.5), "normal") // d20=15, d8=5
	if err != nil {
		t.Fatalf("ResolveAttack: %v", err)
	}
	if !resolution.Hit {
		t.Fatalf("resolution.Hit = false, want true")
	}
	if resolution.Damage != 8 {
		t.Fatalf("resolution.Damage = %d, want 8", resolution.Damage)
	}
	if got := nextState.Combatants[1].HP; got != 4 {
		t.Fatalf("combatants[1].HP = %d, want 4", got)
	}
}

func TestDoublesOnlyDamageDiceOnACriticalHit(t *testing.T) {
	state := CombatState{Active: true, Round: 1, TurnIndex: 0, Combatants: []Combatant{combatFighter, combatGoblin}}
	_, resolution, err := ResolveAttack(state, "fighter", "goblin", seq(0.999, 0, 0.2), "normal") // natural 20; d8 1+2; +3
	if err != nil {
		t.Fatalf("ResolveAttack: %v", err)
	}
	if !resolution.Critical {
		t.Fatalf("resolution.Critical = false, want true")
	}
	if resolution.Damage != 6 {
		t.Fatalf("resolution.Damage = %d, want 6", resolution.Damage)
	}
	got, err := RollExpression("2d6+2", seq(), false)
	if err != nil {
		t.Fatalf("RollExpression: %v", err)
	}
	if got != 4 {
		t.Fatalf("RollExpression(2d6+2, always-0) = %d, want 4", got)
	}
}
