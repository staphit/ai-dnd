package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dndduet/internal/apperr"
	"dndduet/internal/dm"
)

// seq returns a slice-backed random source; it repeats the last value once
// the script is exhausted so unrelated rolls stay deterministic.
func seq(rolls ...float64) func() float64 {
	i := 0
	return func() float64 {
		if i < len(rolls) {
			v := rolls[i]
			i++
			return v
		}
		return rolls[len(rolls)-1]
	}
}

func TestCombatFlowAndVictoryXP(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	id := view.ID

	// Initiative rolls: party1, party2, enemy. High rolls for players so the
	// party acts first; the enemy has 1 HP so one hit wins the fight.
	s.WithDice(seq(0.99, 0.5, 0.01))
	view2, err := s.StartCombatManual(id, []EnemySpec{{Name: "骸骨守衛", AC: 5, HP: 1, AttackBonus: 2, Damage: "1d4", DamageType: "穿刺"}})
	if err != nil {
		t.Fatalf("start combat: %v", err)
	}
	if view2.Combat == nil || !view2.Combat.Active || len(view2.Combat.Combatants) != 3 {
		t.Fatalf("combat state wrong: %+v", view2.Combat)
	}
	if !strings.Contains(view2.Story[len(view2.Story)-1].Text, "戰鬥開始。先攻順序：") {
		t.Fatalf("missing initiative log: %+v", view2.Story[len(view2.Story)-1])
	}
	current := view2.Combat.Combatants[view2.Combat.TurnIndex]
	if current.Side != "party" {
		t.Fatalf("expected party to act first, got %+v", current)
	}
	xpBefore := view2.Players[0].Experience

	// Attack roll 19 (0.9) hits AC 5, damage die kills the 1-HP enemy.
	s.WithDice(seq(0.9, 0.9))
	result, err := s.Attack(id, AttackParams{})
	if err != nil {
		t.Fatalf("attack: %v", err)
	}
	if !result.Resolution.Hit {
		t.Fatalf("expected hit: %+v", result.Resolution)
	}
	v := result.View
	if v.Players[0].Experience <= xpBefore {
		t.Fatalf("victory XP not granted: before %d after %d", xpBefore, v.Players[0].Experience)
	}
	found := false
	for _, e := range v.Story {
		if strings.Contains(e.Text, "戰鬥勝利：每位角色獲得") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing victory log")
	}

	// Second attack in the same turn must fail (action economy).
	if _, err := s.Attack(id, AttackParams{}); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("expected action-used 400, got %v", err)
	}

	conclude, err := s.Conclude(id)
	if err != nil {
		t.Fatalf("conclude: %v", err)
	}
	if conclude.Conclusion.Outcome != "victory" || !strings.Contains(conclude.Conclusion.Summary, "骸骨守衛") {
		t.Fatalf("conclusion wrong: %+v", conclude.Conclusion)
	}
	if conclude.View.Combat == nil || conclude.View.Combat.Active {
		t.Fatalf("combat should be inactive: %+v", conclude.View.Combat)
	}
	if _, err := s.Conclude(id); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("conclude twice should 400, got %v", err)
	}
}

func TestRetryCombatRestoresSnapshot(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	id := view.ID

	// No active combat yet: retry must 400.
	if _, err := s.RetryCombat(id); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("expected 400 without combat, got %v", err)
	}

	// Party acts first against a tough enemy.
	s.WithDice(seq(0.99, 0.5, 0.01))
	view2, err := s.StartCombatManual(id, []EnemySpec{{Name: "石像鬼", AC: 5, HP: 30, AttackBonus: 4, Damage: "1d6", DamageType: "鈍擊"}})
	if err != nil {
		t.Fatalf("start combat: %v", err)
	}
	startHP := map[string]int{}
	for _, c := range view2.Combat.Combatants {
		startHP[c.ID] = c.HP
	}
	startOrder := initiativeOrder(*view2.Combat)

	// A hit damages the enemy, diverging from the snapshot.
	s.WithDice(seq(0.9, 0.9))
	result, err := s.Attack(id, AttackParams{})
	if err != nil {
		t.Fatalf("attack: %v", err)
	}
	enemyHurt := false
	for _, c := range result.View.Combat.Combatants {
		if c.Side == "enemy" && c.HP < startHP[c.ID] {
			enemyHurt = true
		}
	}
	if !enemyHurt {
		t.Fatalf("enemy should be damaged before retry: %+v", result.View.Combat.Combatants)
	}

	retried, err := s.RetryCombat(id)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.Combat == nil || !retried.Combat.Active {
		t.Fatalf("combat should stay active after retry: %+v", retried.Combat)
	}
	for _, c := range retried.Combat.Combatants {
		if c.HP != startHP[c.ID] {
			t.Fatalf("%s HP not restored: got %d want %d", c.Name, c.HP, startHP[c.ID])
		}
	}
	if got := initiativeOrder(*retried.Combat); got != startOrder {
		t.Fatalf("initiative order changed: got %q want %q", got, startOrder)
	}
	last := retried.Story[len(retried.Story)-1]
	if !strings.Contains(last.Text, "戰鬥重來") {
		t.Fatalf("missing retry log: %q", last.Text)
	}

	// Conclude drops the snapshot: a later retry (new combat, missing
	// snapshot) is impossible only when no combat is active.
	if _, err := s.Conclude(id); err != nil {
		t.Fatalf("conclude: %v", err)
	}
	if _, err := s.RetryCombat(id); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("expected 400 after conclude, got %v", err)
	}
}

func TestShopBuySell(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	id := view.ID

	goldBefore := view.Players[0].Gold
	if goldBefore < 15 {
		t.Fatalf("expected starting gold, got %d", goldBefore)
	}
	bought, err := s.BuyItem(id, "player1", "longsword")
	if err != nil {
		t.Fatalf("buy: %v", err)
	}
	p := bought.Players[0]
	if p.Gold != goldBefore-15 {
		t.Fatalf("gold not deducted: %d -> %d", goldBefore, p.Gold)
	}
	if p.Equipment[len(p.Equipment)-1] != "長劍" {
		t.Fatalf("equipment missing: %v", p.Equipment)
	}

	sold, err := s.SellItem(id, "player1", "長劍")
	if err != nil {
		t.Fatalf("sell: %v", err)
	}
	p = sold.Players[0]
	if p.Gold != goldBefore-15+7 {
		t.Fatalf("sell refund wrong: %d", p.Gold)
	}
	for _, item := range p.Equipment {
		if item == "長劍" {
			t.Fatalf("item not removed: %v", p.Equipment)
		}
	}

	if _, err := s.BuyItem(id, "player1", "nope"); apperr.StatusOf(err, 0) != 404 {
		t.Fatalf("expected 404 unknown item, got %v", err)
	}
}

func TestReviveDownedAlly(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	id := view.ID

	// Nobody is down yet.
	if _, err := s.Revive(id, "player1", "player2"); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("expected 400 when target standing, got %v", err)
	}

	// Down player2 via import-free path: enemy first, huge damage.
	s.WithDice(seq(0.01, 0.02, 0.99))
	if _, err := s.StartCombatManual(id, []EnemySpec{{Name: "巨魔", AC: 12, HP: 40, AttackBonus: 30, Damage: "6d6+20", DamageType: "鈍擊"}}); err != nil {
		t.Fatalf("start: %v", err)
	}
	s.WithDice(seq(0.95))
	runner := func(ctx context.Context, input dm.TacticsInput) (dm.Tactic, error) {
		return dm.Tactic{TargetID: "player2", Attack: "重擊", Intent: "巨魔掄起巨棒砸向牧師。"}, nil
	}
	result, err := s.EnemyTurn(context.Background(), id, runner)
	if err != nil {
		t.Fatalf("enemy turn: %v", err)
	}
	var downed bool
	for _, p := range result.View.Players {
		if p.ID == "player2" && p.HP == 0 {
			downed = true
		}
	}
	if !downed {
		t.Fatalf("player2 should be at 0 HP: %+v", result.View.Players)
	}

	// It is now a party member's turn; player1 spends the action to revive.
	roundBefore := result.View.Round
	revived, err := s.Revive(id, "player1", "player2")
	if err != nil {
		t.Fatalf("revive: %v", err)
	}
	for _, p := range revived.Players {
		if p.ID == "player2" {
			if p.HP <= 0 {
				t.Fatalf("player2 not revived: %+v", p)
			}
			if p.Condition == "倒地" {
				t.Fatalf("condition not cleared: %+v", p)
			}
		}
	}
	for _, c := range revived.Combat.Combatants {
		if c.PlayerID == "player2" && (c.Defeated || c.HP <= 0) {
			t.Fatalf("combatant not stood up: %+v", c)
		}
	}
	if revived.Round != roundBefore {
		t.Fatalf("combat revive must not consume exploration time: %d -> %d", roundBefore, revived.Round)
	}
	// The action is spent: attacking now must fail.
	if _, err := s.Attack(id, AttackParams{}); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("expected action-used 400 after revive, got %v", err)
	}
	last := revived.Story[len(revived.Story)-1]
	if !strings.Contains(last.Text, "救援倒地的") {
		t.Fatalf("missing revive log: %q", last.Text)
	}
}

func TestEnemyTurnAIAndFallback(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	id := view.ID

	// Enemy wins initiative (party low, enemy high).
	s.WithDice(seq(0.01, 0.02, 0.99))
	view2, err := s.StartCombatManual(id, []EnemySpec{{Name: "灰狼", AC: 12, HP: 11, AttackBonus: 4, Damage: "1d6+1", DamageType: "穿刺"}})
	if err != nil {
		t.Fatalf("start combat: %v", err)
	}
	if view2.Combat.Combatants[view2.Combat.TurnIndex].Side != "enemy" {
		t.Fatalf("expected enemy first: %+v", view2.Combat.Combatants[view2.Combat.TurnIndex])
	}

	// AI picks player2 explicitly; server rolls a miss (0.05 → natural 2).
	s.WithDice(seq(0.05))
	called := false
	runner := func(ctx context.Context, input dm.TacticsInput) (dm.Tactic, error) {
		called = true
		if input.EnemyName != "灰狼" || len(input.Targets) != 2 {
			t.Fatalf("tactics input wrong: %+v", input)
		}
		return dm.Tactic{TargetID: "player2", Attack: "撕咬", Intent: "灰狼繞過盾牆，撲向後排的牧師。"}, nil
	}
	result, err := s.EnemyTurn(context.Background(), id, runner)
	if err != nil {
		t.Fatalf("enemy turn: %v", err)
	}
	if !called || result.Fallback {
		t.Fatalf("AI path not used: called=%v fallback=%v", called, result.Fallback)
	}
	last := result.View.Story[len(result.View.Story)-1]
	if !strings.Contains(last.Text, "【敵方】灰狼繞過盾牆") {
		t.Fatalf("enemy log wrong: %q", last.Text)
	}
	if result.View.Combat.Combatants[result.View.Combat.TurnIndex].Side != "party" {
		t.Fatalf("turn should advance to party: %+v", result.View.Combat)
	}
	if result.EnemyTurnPending {
		t.Fatal("no further enemy turn expected")
	}

	// Not the enemy's turn anymore.
	if _, err := s.EnemyTurn(context.Background(), id, runner); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("expected 400 when not enemy turn, got %v", err)
	}
}

func TestEnemyTurnFallbackTargetsLowestHP(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	id := view.ID

	// Wound player1 via import? Simpler: enemy first, runner errors, fallback
	// picks lowest-HP player. player2 (cleric L5) HP 24 < player1 (ranger) 28.
	s.WithDice(seq(0.01, 0.02, 0.99))
	if _, err := s.StartCombatManual(id, []EnemySpec{{Name: "潛伏者", AC: 12, HP: 9, AttackBonus: 3, Damage: "1d4", DamageType: "鈍擊"}}); err != nil {
		t.Fatalf("start: %v", err)
	}
	s.WithDice(seq(0.95, 0.5))
	failing := func(ctx context.Context, input dm.TacticsInput) (dm.Tactic, error) {
		return dm.Tactic{}, errors.New("provider down")
	}
	result, err := s.EnemyTurn(context.Background(), id, failing)
	if err != nil {
		t.Fatalf("enemy turn: %v", err)
	}
	if !result.Fallback {
		t.Fatal("expected fallback")
	}
	if !strings.Contains(result.Intent, "米芮") {
		t.Fatalf("fallback should target lowest-HP 米芮, intent: %q", result.Intent)
	}
	// The hit (natural 20 at 0.95) damaged the cleric and synced the sheet.
	var cleric = result.View.Players[1]
	if cleric.HP >= 24 {
		t.Fatalf("cleric HP should drop below 24, got %d", cleric.HP)
	}
}
