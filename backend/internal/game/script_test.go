package game

import (
	"strings"
	"testing"

	"dndduet/internal/dm"
)

// registerTestModule injects a tiny module so tests don't depend on the
// shipped ashen-crown content.
func registerTestModule(t *testing.T) *ScriptModule {
	t.Helper()
	mod := &ScriptModule{
		ScriptID: "test-module", Entry: "entry", GoodThreshold: 1,
		StageObjectives: map[string]ScriptObjective{
			"中期": {Objective: "深入水道追查教團", Context: "測試背景", Stakes: "測試風險"},
		},
		Nodes: []ScriptNode{
			{
				ID: "entry", Stage: "前期", Type: "explore", Title: "入口",
				Directive: "描述禮拜堂入口。",
				Choices: []ScriptChoice{
					{ID: "A", Text: "搜索祭壇後的暗格", Next: "loot", Alignment: 1},
					{ID: "B", Text: "直接砸開祭壇", Next: "end_bad", Alignment: -2},
				},
			},
			{
				ID: "loot", Stage: "前期", Type: "treasure", Title: "暗格寶藏",
				Directive: "描述暗格中的發現。",
				Treasure:  &ScriptTreasure{Gold: 101, Items: []ScriptItem{{Name: "河底短刃", Damage: "1d6+1", DamageType: "穿刺"}}},
				Choices: []ScriptChoice{
					{ID: "A", Text: "沿階梯深入地下", Next: "fight", Alignment: 0},
					{ID: "B", Text: "返回入口", Next: "entry", Alignment: 0},
				},
			},
			{
				ID: "fight", Stage: "中期", Type: "combat", Title: "水道伏擊",
				Directive: "灰袍教徒撲出。",
				Combat: &ScriptCombat{
					Enemies:           []ScriptEnemy{{Name: "灰袍教徒", AC: 12, HP: 15, InitiativeBonus: 1, AttackBonus: 3, Damage: "1d6+1", DamageType: "穿刺"}},
					AddPerExtraPlayer: []ScriptEnemy{{Name: "教團走狗", AC: 11, HP: 9, InitiativeBonus: 1, AttackBonus: 2, Damage: "1d4+1", DamageType: "穿刺"}, {Name: "教團走狗二", AC: 11, HP: 9, InitiativeBonus: 1, AttackBonus: 2, Damage: "1d4+1", DamageType: "穿刺"}, {Name: "教團走狗三", AC: 11, HP: 9, InitiativeBonus: 1, AttackBonus: 2, Damage: "1d4+1", DamageType: "穿刺"}},
				},
				Choices: []ScriptChoice{
					{ID: "A", Text: "追擊逃走的教徒", Next: "end_good", Alignment: 1},
					{ID: "B", Text: "放任教徒逃走", Next: "end_bad", Alignment: -1},
				},
			},
			{ID: "end_good", Stage: "結局", Type: "ending", Title: "黎明鐘聲", EndingKind: "good", Directive: "伊薩克獲救。"},
			{ID: "end_bad", Stage: "結局", Type: "ending", Title: "沉沒之城", EndingKind: "bad", Directive: "水道吞沒一切。"},
		},
	}
	if err := mod.compile(); err != nil {
		t.Fatalf("compile test module: %v", err)
	}
	scriptModules[mod.ScriptID] = mod
	t.Cleanup(func() { delete(scriptModules, mod.ScriptID) })
	return mod
}

func createScripted(t *testing.T, s *Service) View {
	t.Helper()
	view, err := s.Create(CreateParams{
		StoryID: "test-module", Title: "測試劇本", Chapter: "第一章", Scene: "入口",
		Objective: "找到伊薩克", ObjectiveContext: "背景", Stakes: "風險", Opening: "開場。",
		Players: []PlayerSeed{
			{Name: "賽勒恩", ClassName: "戰士"},
			{Name: "米芮", ClassName: "牧師"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return view
}

func scriptedTurn(chosen string) *dm.Turn {
	return &dm.Turn{
		Narration: "n", Scene: "s", Objective: "o", ObjectiveContext: "c", Stakes: "st",
		Choices: []dm.Choice{{Text: "自由建議", PlayerID: "player1"}},
		Combat:  dm.Combat{FirstTurn: "initiative"},
		Script:  dm.ScriptSignal{ChosenOption: chosen},
	}
}

func TestScriptCampaignWalkthrough(t *testing.T) {
	registerTestModule(t)
	s := newTestService(t)
	view := createScripted(t, s)

	if view.Script == nil || view.Script.NodeTitle != "入口" || view.Script.Stage != "前期" {
		t.Fatalf("script progress missing on create: %+v", view.Script)
	}

	// Turn 1: DM signals option A → advance to the treasure node, loot lands.
	prepared := preparedActions(t, s, view.ID)
	applied, err := s.ApplyDMTurn(view.ID, prepared, scriptedTurn("A"))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	v := applied.View
	if v.Script == nil || v.Script.NodeTitle != "暗格寶藏" {
		t.Fatalf("did not advance to loot node: %+v", v.Script)
	}
	if v.Players[0].Gold != 100+51 || v.Players[1].Gold != 100+50 {
		t.Fatalf("treasure gold not split: %d / %d", v.Players[0].Gold, v.Players[1].Gold)
	}
	foundAttack := false
	for _, a := range v.Players[0].Attacks {
		if a.Name == "河底短刃" {
			foundAttack = true
		}
	}
	if !foundAttack {
		t.Fatal("treasure weapon did not become an attack")
	}
	// The node's scripted options are pinned to the front of the choices.
	if len(v.Choices) < 2 || v.Choices[0].Text != "沿階梯深入地下" || v.Choices[1].Text != "返回入口" {
		t.Fatalf("script choices not pinned: %+v", v.Choices)
	}

	// Turn 2: entering the combat node auto-starts the scaled encounter
	// (1 base enemy + 1 extra for the second player).
	prepared = preparedActions(t, s, view.ID)
	applied, err = s.ApplyDMTurn(view.ID, prepared, scriptedTurn("A"))
	if err != nil {
		t.Fatalf("apply combat node: %v", err)
	}
	// loot(前期) → fight(中期) crosses an act boundary: popup event, no
	// journal transition line.
	if applied.StageClear == nil || applied.StageClear.Cleared != "前期" || applied.StageClear.Next != "中期" {
		t.Fatalf("stage clear missing: %+v", applied.StageClear)
	}
	for _, e := range applied.View.Story {
		if strings.Contains(e.Text, "劇本推進") {
			t.Fatalf("transition line must stay out of the feed: %q", e.Text)
		}
	}
	v = applied.View
	if v.Combat == nil || !v.Combat.Active {
		t.Fatalf("scripted combat did not start: %+v", v.Combat)
	}
	enemies := 0
	for _, c := range v.Combat.Combatants {
		if c.PlayerID == "" {
			enemies++
		}
	}
	if enemies != 2 {
		t.Fatalf("want 2 scaled enemies for 2 players, got %d", enemies)
	}
	// While combat is active the DM gets director notes but no chosenOption
	// advancement should happen from stale signals: node stays put.
	if v.Script.NodeTitle != "水道伏擊" {
		t.Fatalf("should sit on combat node: %+v", v.Script)
	}
	// This turn carried no arc signal — the self-healing sync alone must pin
	// the mission panel to the node's stage (中期) with module deadlines.
	if v.StoryArc == nil || v.StoryArc.Current != 1 {
		t.Fatalf("arc should self-heal to 中期: %+v", v.StoryArc)
	}
	if v.StoryArc.Phases[0].DeadlineRound != 15 {
		t.Fatalf("deadline should be module-derived (min 15): %+v", v.StoryArc.Phases[0])
	}
}

func TestScriptBadEndingAndTextMatchAdvance(t *testing.T) {
	registerTestModule(t)
	s := newTestService(t)
	view := createScripted(t, s)

	// No DM signal; player1's declared action is the verbatim choice text.
	prepared, err := s.PrepareActionsTurn(view.ID, map[string]string{
		"player1": "直接砸開祭壇",
		"player2": "警戒四周。",
	}, nil)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	applied, err := s.ApplyDMTurn(view.ID, prepared, scriptedTurn(""))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	v := applied.View
	if v.Script == nil || !v.Script.Ended || v.Script.Ending != "bad" {
		t.Fatalf("bad ending not reached: %+v", v.Script)
	}
	if v.StoryArc == nil || !v.StoryArc.Ended {
		t.Fatalf("arc should end with the script: %+v", v.StoryArc)
	}
	foundLog := false
	for _, e := range v.Story {
		if strings.Contains(e.Text, "沉沒結局") {
			foundLog = true
		}
	}
	if !foundLog {
		t.Fatal("missing ending system log")
	}
}

func TestScriptSuppressesFreeformDMCombat(t *testing.T) {
	registerTestModule(t)
	s := newTestService(t)
	view := createScripted(t, s)

	prepared := preparedActions(t, s, view.ID)
	turn := scriptedTurn("")
	turn.Combat = dm.Combat{Starts: true, FirstTurn: "enemy", Enemies: []dm.Enemy{
		{Name: "野狗", AC: 10, HP: 5, InitiativeBonus: 0, AttackBonus: 2, Damage: "1d4", DamageType: "穿刺"},
	}}
	applied, err := s.ApplyDMTurn(view.ID, prepared, turn)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.View.Combat != nil && applied.View.Combat.Active {
		t.Fatalf("freeform DM combat must be suppressed on scripted campaigns: %+v", applied.View.Combat)
	}
}

func TestShippedModulesCompile(t *testing.T) {
	if _, ok := scriptModules["ashen-crown"]; !ok {
		t.Fatal("ashen-crown module missing from embedded scripts")
	}
	for id, mod := range scriptModules {
		if err := mod.compile(); err != nil {
			t.Fatalf("module %s: %v", id, err)
		}
	}
}

func TestScriptCreateSeedsEntryChoices(t *testing.T) {
	registerTestModule(t)
	s := newTestService(t)
	view := createScripted(t, s)
	// The scripted UI has no free-text input, so round one must already carry
	// the entry node's options or no player could ever lock an action.
	if len(view.Choices) != 2 || view.Choices[0].Text != "搜索祭壇後的暗格" || view.Choices[1].Text != "直接砸開祭壇" {
		t.Fatalf("entry choices not seeded at create: %+v", view.Choices)
	}
}

func TestScriptFreeformModeSkipsModule(t *testing.T) {
	registerTestModule(t)
	s := newTestService(t)
	view, err := s.Create(CreateParams{
		StoryID: "test-module", StoryMode: "freeform", Title: "自由", Chapter: "c", Scene: "s",
		Objective: "o", ObjectiveContext: "oc", Stakes: "st", Opening: "op",
		Players: []PlayerSeed{{Name: "獨行者", ClassName: "戰士"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if view.Script != nil {
		t.Fatalf("freeform mode must not attach the module: %+v", view.Script)
	}
	if len(view.Choices) != 0 {
		t.Fatalf("freeform create should not seed script choices: %+v", view.Choices)
	}
}

func TestScriptNoAdvanceWhenCheckOpensOrMidCombat(t *testing.T) {
	registerTestModule(t)
	s := newTestService(t)
	view := createScripted(t, s)

	// A turn that opens a required check must not advance even with a signal.
	prepared := preparedActions(t, s, view.ID)
	turn := scriptedTurn("A")
	turn.RequiresCheck = true
	turn.Check = &dm.Check{Character: "賽勒恩", PlayerID: "player1", Ability: "智力", Skill: "調查", DC: 12, Reason: "檢視暗格"}
	applied, err := s.ApplyDMTurn(view.ID, prepared, turn)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.View.Script.NodeTitle != "入口" {
		t.Fatalf("must not advance while the check is pending: %+v", applied.View.Script)
	}

	// The check resolution turn settles the branch and may advance.
	preparedCheck, err := s.PrepareCheckTurn(view.ID, 15)
	if err != nil {
		t.Fatalf("prepare check: %v", err)
	}
	applied, err = s.ApplyDMTurn(view.ID, preparedCheck, scriptedTurn("A"))
	if err != nil {
		t.Fatalf("apply resolution: %v", err)
	}
	if applied.View.Script.NodeTitle != "暗格寶藏" {
		t.Fatalf("resolution turn should advance: %+v", applied.View.Script)
	}

	// Walk into the combat node, then a stale signal mid-combat must not move.
	prepared = preparedActions(t, s, view.ID)
	if applied, err = s.ApplyDMTurn(view.ID, prepared, scriptedTurn("A")); err != nil {
		t.Fatalf("apply combat entry: %v", err)
	}
	if applied.View.Combat == nil || !applied.View.Combat.Active {
		t.Fatal("scripted combat should be active")
	}
	preparedCheck, err = s.PrepareCheckTurn(view.ID, 10)
	if err == nil {
		if applied, err = s.ApplyDMTurn(view.ID, preparedCheck, scriptedTurn("A")); err == nil &&
			applied.View.Script.NodeTitle != "水道伏擊" {
			t.Fatalf("mid-combat signal must not advance: %+v", applied.View.Script)
		}
	}
}

func TestBuildScriptTurnResolvesLocally(t *testing.T) {
	registerTestModule(t)
	s := newTestService(t)
	view := createScripted(t, s)

	prepared, err := s.PrepareActionsTurn(view.ID, map[string]string{
		"player1": "搜索祭壇後的暗格",
		"player2": "警戒四周。",
	}, nil)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	turn, scripted, err := s.BuildScriptTurn(view.ID, prepared)
	if err != nil || !scripted {
		t.Fatalf("want local scripted turn, got scripted=%v err=%v", scripted, err)
	}
	if turn.Script.ChosenOption != "A" || turn.Scene != "暗格寶藏" {
		t.Fatalf("local turn wrong: %+v", turn)
	}
	if !strings.Contains(turn.Narration, "描述暗格中的發現") && !strings.Contains(turn.Narration, "暗格") {
		t.Fatalf("narration missing node text: %q", turn.Narration)
	}
	if len(turn.ExperienceAwards) != 2 || turn.ExperienceAwards[0].Amount != scriptAdvanceXP {
		t.Fatalf("advance XP missing: %+v", turn.ExperienceAwards)
	}
	applied, err := s.ApplyDMTurn(view.ID, prepared, turn)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.View.Script.NodeTitle != "暗格寶藏" {
		t.Fatalf("local turn did not advance: %+v", applied.View.Script)
	}

	// Freeform campaigns fall through to the AI.
	free, err := s.Create(CreateParams{
		StoryID: "custom", Title: "自由", Chapter: "c", Scene: "s",
		Objective: "o", ObjectiveContext: "oc", Stakes: "st", Opening: "op",
		Players: []PlayerSeed{{Name: "獨行者", ClassName: "戰士"}},
	})
	if err != nil {
		t.Fatalf("create freeform: %v", err)
	}
	preparedFree, err := s.PrepareActionsTurn(free.ID, map[string]string{"player1": "探索。"}, nil)
	if err != nil {
		t.Fatalf("prepare freeform: %v", err)
	}
	if _, scripted, err := s.BuildScriptTurn(free.ID, preparedFree); err != nil || scripted {
		t.Fatalf("freeform must not resolve locally: scripted=%v err=%v", scripted, err)
	}
}

func TestScriptStageCrossingInstallsObjectiveAndArc(t *testing.T) {
	registerTestModule(t)
	s := newTestService(t)
	view := createScripted(t, s)

	// entry(前期) → loot(前期): no stage change.
	prepared, err := s.PrepareActionsTurn(view.ID, map[string]string{
		"player1": "搜索祭壇後的暗格",
		"player2": "警戒四周。",
	}, nil)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	turn, scripted, err := s.BuildScriptTurn(view.ID, prepared)
	if err != nil || !scripted {
		t.Fatalf("build: %v", err)
	}
	if turn.Arc.PhaseComplete || turn.Objective != "" {
		t.Fatalf("no stage change expected: %+v", turn.Arc)
	}
	if _, err = s.ApplyDMTurn(view.ID, prepared, turn); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// loot(前期) → fight(中期): installs the 中期 mission and completes phase 1.
	prepared, err = s.PrepareActionsTurn(view.ID, map[string]string{
		"player1": "沿階梯深入地下",
		"player2": "警戒四周。",
	}, nil)
	if err != nil {
		t.Fatalf("prepare 2: %v", err)
	}
	turn, _, err = s.BuildScriptTurn(view.ID, prepared)
	if err != nil {
		t.Fatalf("build 2: %v", err)
	}
	if !turn.Arc.PhaseComplete || turn.Objective != "深入水道追查教團" {
		t.Fatalf("stage crossing should switch objective + complete phase: obj=%q arc=%+v", turn.Objective, turn.Arc)
	}
	applied, err := s.ApplyDMTurn(view.ID, prepared, turn)
	if err != nil {
		t.Fatalf("apply 2: %v", err)
	}
	if applied.View.Objective != "深入水道追查教團" {
		t.Fatalf("campaign objective not switched: %q", applied.View.Objective)
	}
	if applied.View.StoryArc == nil || applied.View.StoryArc.Current != 1 {
		t.Fatalf("arc should advance to phase 2: %+v", applied.View.StoryArc)
	}
}
