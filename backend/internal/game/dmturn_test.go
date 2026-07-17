package game

import (
	"strings"
	"testing"

	"dndduet/internal/apperr"
	"dndduet/internal/dm"
)

func preparedActions(t *testing.T, s *Service, id string) PreparedDMTurn {
	t.Helper()
	prepared, err := s.PrepareActionsTurn(id, map[string]string{
		"player1": "我搜索祭壇。",
		"player2": "我警戒四周。",
	}, nil)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	return prepared
}

func TestPrepareActionsTurnValidates(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)

	// Missing player2 action → 400.
	if _, err := s.PrepareActionsTurn(view.ID, map[string]string{"player1": "x"}, nil); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("want 400, got %v", err)
	}

	// Structured intent for an unknown spell → ActionIssuesError.
	_, err := s.PrepareActionsTurn(view.ID,
		map[string]string{"player1": "施法", "player2": "警戒"},
		map[string]Intent{"player1": {Type: "spell", SpellID: "nope", TargetID: "player1"}})
	var issues *ActionIssuesError
	if !asActionIssues(err, &issues) || len(issues.Issues) != 1 || issues.Issues[0].PlayerID != "player1" {
		t.Fatalf("want one player1 issue, got %v", err)
	}

	prepared := preparedActions(t, s, view.ID)
	if len(prepared.Input.Players) != 2 || !strings.Contains(prepared.Input.Players[1].Summary, "法術位") {
		t.Fatalf("digest missing: %+v", prepared.Input.Players)
	}
	prompt := dm.BuildDMRequestV2(prepared.Input)
	if !strings.Contains(prompt, "本輪宣告") || !strings.Contains(prompt, "我搜索祭壇。") {
		t.Fatalf("prompt missing actions:\n%s", prompt)
	}
	if strings.Contains(prompt, "熟練技能") {
		t.Fatal("prompt should not carry the old skill block")
	}
}

func asActionIssues(err error, target **ActionIssuesError) bool {
	if e, ok := err.(*ActionIssuesError); ok {
		*target = e
		return true
	}
	return false
}

func TestApplyDMTurnFullFlow(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	prepared := preparedActions(t, s, view.ID)

	hpBefore := view.Players[0].HP
	xpBefore := view.Players[0].Experience
	s.WithDice(seq(0.5))
	turn := &dm.Turn{
		Narration: "祭壇下的石板碎裂，陷阱的毒針擦過賽勒恩。", Scene: "祭壇密室",
		Objective: "追蹤地道", ObjectiveContext: "ctx", Stakes: "stakes",
		RequiresCheck:    true,
		Check:            &dm.Check{Character: "米芮・鐵歌", PlayerID: "player2", Ability: "感知", Skill: "察覺", DC: 13, Reason: "查覺潛伏氣息"},
		Choices:          []dm.Choice{{Text: "下地道", PlayerID: "player1"}},
		Effects:          []dm.Effect{{TargetID: "player1", Kind: "damage", Amount: 5, Reason: "毒針"}},
		PrivateMessages:  []dm.PrivateMessage{{PlayerID: "player2", Text: "你聽見低語。"}},
		ExperienceAwards: []dm.ExperienceAward{{PlayerID: "player1", Amount: 75, Reason: "發現陷阱"}},
		Combat:           dm.Combat{FirstTurn: "initiative", Enemies: []dm.Enemy{}},
	}
	applied, err := s.ApplyDMTurn(view.ID, prepared, turn)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	v := applied.View
	if v.Players[0].HP != hpBefore-5 {
		t.Fatalf("effect not applied: %d -> %d", hpBefore, v.Players[0].HP)
	}
	if v.Players[0].Experience != xpBefore+75 {
		t.Fatalf("xp not applied")
	}
	if v.Round != view.Round+1 || v.Scene != "祭壇密室" || len(v.Pending) != 0 {
		t.Fatalf("meta wrong: round %d scene %s pending %v", v.Round, v.Scene, v.Pending)
	}
	if v.RequiredCheck == nil || v.RequiredCheck.PlayerID != "player2" || v.RequiredCheck.Modifier == 0 {
		t.Fatalf("check not stored with modifier: %+v", v.RequiredCheck)
	}
	var hasPrivate, hasAward bool
	for _, e := range v.Story {
		if e.Audience == "player2" && strings.Contains(e.Text, "低語") {
			hasPrivate = true
		}
		if strings.Contains(e.Text, "獲得 75 XP") {
			hasAward = true
		}
	}
	if !hasPrivate || !hasAward {
		t.Fatalf("journal entries missing: private=%v award=%v", hasPrivate, hasAward)
	}

	// Check continuation: natural 18 + modifier vs DC 13.
	preparedCheck, err := s.PrepareCheckTurn(view.ID, 18)
	if err != nil {
		t.Fatalf("prepare check: %v", err)
	}
	if preparedCheck.Input.Resolution == nil || !preparedCheck.Input.Resolution.Success {
		t.Fatalf("resolution wrong: %+v", preparedCheck.Input.Resolution)
	}
	contTurn := &dm.Turn{
		Narration: "米芮察覺到氣息來源。", Scene: "祭壇密室", Objective: "o", ObjectiveContext: "c", Stakes: "s",
		Choices: []dm.Choice{{Text: "前進"}}, Combat: dm.Combat{FirstTurn: "initiative"},
	}
	appliedCheck, err := s.ApplyDMTurn(view.ID, preparedCheck, contTurn)
	if err != nil {
		t.Fatalf("apply check turn: %v", err)
	}
	if appliedCheck.View.RequiredCheck != nil {
		t.Fatal("check should be cleared after continuation")
	}
	if appliedCheck.View.Round != v.Round {
		t.Fatal("continuation must not advance the round")
	}
}

func TestApplyDMTurnCombatStartAndRejection(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	prepared := preparedActions(t, s, view.ID)

	s.WithDice(seq(0.9, 0.5, 0.1))
	turn := &dm.Turn{
		Narration: "骸骨從陰影中撲出！", Scene: "s", Objective: "o", ObjectiveContext: "c", Stakes: "st",
		Choices: []dm.Choice{{Text: "應戰"}},
		Combat: dm.Combat{Starts: true, FirstTurn: "enemy", Enemies: []dm.Enemy{
			{Name: "骸骨守衛", AC: 13, HP: 13, InitiativeBonus: 2, AttackBonus: 4, Damage: "1d6+2", DamageType: "穿刺"},
		}},
	}
	applied, err := s.ApplyDMTurn(view.ID, prepared, turn)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.View.Combat == nil || !applied.View.Combat.Active || len(applied.View.Combat.Combatants) != 3 {
		t.Fatalf("combat not started: %+v", applied.View.Combat)
	}

	// AI narrative rejection unlocks the player and does not advance.
	s2 := newTestService(t)
	view2 := createSample(t, s2)
	prepared2 := preparedActions(t, s2, view2.ID)
	rejection := &dm.Turn{
		Narration: "n", Scene: "s", Objective: "o", ObjectiveContext: "c", Stakes: "st",
		Choices:      []dm.Choice{{Text: "x"}},
		ActionIssues: []dm.ActionIssue{{PlayerID: "player1", Message: "場景中沒有祭壇。"}},
		Combat:       dm.Combat{FirstTurn: "initiative"},
	}
	applied2, err := s2.ApplyDMTurn(view2.ID, prepared2, rejection)
	if err != nil {
		t.Fatalf("apply rejection: %v", err)
	}
	if len(applied2.Rejected) != 1 || applied2.View.Round != view2.Round {
		t.Fatalf("rejection should not advance: %+v round %d", applied2.Rejected, applied2.View.Round)
	}
	if _, locked := applied2.View.Pending["player1"]; locked {
		t.Fatal("player1 should be unlocked")
	}
	last := applied2.View.Story[len(applied2.View.Story)-1]
	if !strings.Contains(last.Text, "【行動駁回】") {
		t.Fatalf("missing rejection entry: %q", last.Text)
	}
}
