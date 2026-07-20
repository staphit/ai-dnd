package rules

// Ported 1:1 from frontend/src/rules/effects.test.ts ("automatic effect
// settlement"), preserving every vitest assertion.

import (
	"strings"
	"testing"
)

// mustResolveSpellEffect mirrors the tests calling resolveSpellEffect
// directly (any throw fails the vitest test).
func mustResolveSpellEffect(t *testing.T, players []Character, combat *CombatState, casterID, targetID string, effect SpellEffect, random RandomSource, forced *ForcedRolls) SpellEffectResult {
	t.Helper()
	result, err := ResolveSpellEffect(players, combat, casterID, targetID, effect, random, forced)
	if err != nil {
		t.Fatalf("resolveSpellEffect threw: %v", err)
	}
	return result
}

func TestUsesTemporaryHpBeforeRegularHp(t *testing.T) {
	result := ApplyDamage(Combatant{ID: "x", Name: "x", Side: "enemy", InitiativeBonus: 0, Initiative: 0, AC: 10, HP: 10, MaxHP: 10, TemporaryHP: 5, AttackBonus: 0, Damage: "1d4", DamageType: "鈍擊"}, 8)
	if result.TemporaryHP != 0 {
		t.Fatalf("expected temporaryHp 0, got %d", result.TemporaryHP)
	}
	if result.HP != 7 {
		t.Fatalf("expected hp 7, got %d", result.HP)
	}
}

func TestAutomaticallySpendsHitDiceOnShortRestUntilFull(t *testing.T) {
	player := CreateLevel3Character("player1", "甲", "戰士")
	player.HP = 1
	result := ResolveShortRest(player, func() float64 { return 0.99 })
	if result.Character.HP != player.MaxHP {
		t.Fatalf("expected hp %d, got %d", player.MaxHP, result.Character.HP)
	}
	if !(result.DiceSpent > 0) {
		t.Fatalf("expected diceSpent > 0, got %d", result.DiceSpent)
	}
}

func TestSettlesHealingSpellsAndBoundsDmDamage(t *testing.T) {
	caster := CreateLevel3Character("player1", "牧者", "牧師")
	wounded := CreateLevel3Character("player2", "傷者", "戰士")
	wounded.HP = 1
	healed := mustResolveSpellEffect(t, []Character{caster, wounded}, nil, caster.ID, wounded.ID, SpellEffect{Kind: "healing", Target: "ally", Dice: "2d4", Flat: 2}, func() float64 { return 0 }, nil)
	if healed.Players[1].HP != 5 {
		t.Fatalf("expected healed hp 5, got %d", healed.Players[1].HP)
	}
	dmPlayers, dmLogs := ApplyDmEffects(healed.Players, []DMEffect{{TargetID: "player2", Kind: "damage", Amount: 9999, Reason: "落石"}})
	if dmPlayers[1].HP != 0 {
		t.Fatalf("expected dm-damaged hp 0, got %d", dmPlayers[1].HP)
	}
	if len(dmLogs) == 0 || !strings.Contains(dmLogs[0], "落石") {
		t.Fatalf("expected first log to match 落石, got %v", dmLogs)
	}
}

func TestKeepsCombatHpSynchronizedAfterHealingAndConsumesTemporaryHpForDmDamage(t *testing.T) {
	caster := CreateLevel3Character("player1", "牧者", "牧師")
	wounded := CreateLevel3Character("player2", "傷者", "戰士")
	wounded.HP = 1
	wounded.TemporaryHP = 3
	combat := &CombatState{
		Active: true, Round: 1, TurnIndex: 0,
		Combatants: []Combatant{
			{ID: caster.ID, PlayerID: caster.ID, Name: caster.Name, Side: "party", InitiativeBonus: 0, Initiative: 20, AC: caster.AC, HP: caster.HP, MaxHP: caster.MaxHP, AttackBonus: 0, Damage: "1d4", DamageType: "鈍擊"},
			{ID: wounded.ID, PlayerID: wounded.ID, Name: wounded.Name, Side: "party", InitiativeBonus: 0, Initiative: 10, AC: wounded.AC, HP: wounded.HP, MaxHP: wounded.MaxHP, TemporaryHP: 3, AttackBonus: 0, Damage: "1d4", DamageType: "鈍擊"},
		},
	}
	healed := mustResolveSpellEffect(t, []Character{caster, wounded}, combat, caster.ID, wounded.ID, SpellEffect{Kind: "healing", Target: "ally", Flat: 4}, func() float64 { return 0 }, nil)
	if healed.Combat == nil || healed.Combat.Combatants[1].HP != 5 {
		t.Fatalf("expected combat combatant hp 5, got %+v", healed.Combat)
	}
	dmPlayers, _ := ApplyDmEffects(healed.Players, []DMEffect{{TargetID: "player2", Kind: "damage", Amount: 5, Reason: "落石"}})
	if dmPlayers[1].TemporaryHP != 0 {
		t.Fatalf("expected temporaryHp 0, got %d", dmPlayers[1].TemporaryHP)
	}
	if dmPlayers[1].HP != 3 {
		t.Fatalf("expected hp 3, got %d", dmPlayers[1].HP)
	}
}

func TestUsesTheVisibleSpellAttackTotalInsteadOfRollingAHiddenD20(t *testing.T) {
	caster := CreateLevel3Character("player1", "術者", "術士")
	enemy := Combatant{ID: "enemy", Name: "敵人", Side: "enemy", InitiativeBonus: 0, Initiative: 10, AC: 15, HP: 10, MaxHP: 10, AttackBonus: 0, Damage: "1d4", DamageType: "鈍擊"}
	combat := &CombatState{Active: true, Round: 1, TurnIndex: 0, Combatants: []Combatant{enemy}}
	attackTotal14 := 14
	missed := mustResolveSpellEffect(t, []Character{caster}, combat, caster.ID, enemy.ID, SpellEffect{Kind: "damage", Target: "creature", Flat: 4, AttackRoll: true}, func() float64 { return 0.99 }, &ForcedRolls{AttackTotal: &attackTotal14})
	if missed.Amount != 0 {
		t.Fatalf("expected missed amount 0, got %d", missed.Amount)
	}
	if missed.Combat == nil || missed.Combat.Combatants[0].HP != 10 {
		t.Fatalf("expected missed combatant hp 10, got %+v", missed.Combat)
	}
	attackTotal15 := 15
	hit := mustResolveSpellEffect(t, []Character{caster}, combat, caster.ID, enemy.ID, SpellEffect{Kind: "damage", Target: "creature", Flat: 4, AttackRoll: true}, func() float64 { return 0 }, &ForcedRolls{AttackTotal: &attackTotal15})
	if hit.Combat == nil || hit.Combat.Combatants[0].HP != 6 {
		t.Fatalf("expected hit combatant hp 6, got %+v", hit.Combat)
	}
}
