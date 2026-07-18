package game

import (
	"strings"
	"testing"

	"dndduet/internal/apperr"
	"dndduet/internal/dm"
	"dndduet/internal/rules"
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

func TestDMTurnLeaseRejectsConcurrentAndStaleApply(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	prepared := preparedActions(t, s, view.ID)

	if _, err := s.PrepareActionsTurn(view.ID, prepared.Actions, nil); apperr.StatusOf(err, 0) != 409 {
		t.Fatalf("concurrent prepare should be 409, got %v", err)
	}
	if _, err := s.UpdateSettings(view.ID, []byte(`{"fontScale":1.1}`)); err != nil {
		t.Fatalf("mutate while provider is running: %v", err)
	}

	turn := BuildDemoTurn(prepared)
	if _, err := s.ApplyDMTurn(view.ID, prepared, turn); apperr.StatusOf(err, 0) != 409 {
		t.Fatalf("stale apply should be 409, got %v", err)
	}
	current, err := s.View(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Round != view.Round {
		t.Fatalf("stale turn advanced round: got %d want %d", current.Round, view.Round)
	}

	// ApplyDMTurn releases the lease even on rejection, so the user can retry.
	retry, err := s.PrepareActionsTurn(view.ID, prepared.Actions, nil)
	if err != nil {
		t.Fatalf("retry prepare: %v", err)
	}
	s.AbortDMTurn(view.ID, retry.TurnToken)
}

func TestDemoTurnPersistsServerAuthoritativeState(t *testing.T) {
	s := newTestService(t)
	view, err := s.Create(CreateParams{
		StoryID: "custom", StoryMode: "freeform", Title: "示範冒險", Scene: "舊塔",
		Objective: "找出鐘聲來源", Players: []PlayerSeed{{Name: "艾拉", ClassName: "遊俠"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := s.PrepareActionsTurn(view.ID, map[string]string{"player1": "檢查塔門。"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := s.ApplyDMTurn(view.ID, prepared, BuildDemoTurn(prepared))
	if err != nil {
		t.Fatal(err)
	}
	if applied.View.Round != 2 || applied.View.Players[0].Experience != view.Players[0].Experience+75 {
		t.Fatalf("demo mechanics not applied: round=%d xp=%d", applied.View.Round, applied.View.Players[0].Experience)
	}

	reloaded, err := s.View(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Round != applied.View.Round || reloaded.Players[0].Experience != applied.View.Players[0].Experience {
		t.Fatalf("reloaded demo state diverged: %+v", reloaded)
	}
	if len(reloaded.Story) < 3 || !strings.Contains(reloaded.Story[len(reloaded.Story)-2].Text, "隊伍的宣告") {
		t.Fatalf("demo narration was not persisted: %+v", reloaded.Story)
	}
}

func TestStoryRevisionTargetsExactVersionAndEntry(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	prepared, err := s.PrepareStoryRevision(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareActionsTurn(view.ID, map[string]string{"player1": "前進", "player2": "警戒"}, nil); apperr.StatusOf(err, 0) != 409 {
		t.Fatalf("DM turn should be blocked by revision lease: %v", err)
	}
	if _, err := s.UpdateSettings(view.ID, []byte(`{"fontScale":1.2}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyStoryRevision(view.ID, prepared, "不應寫入"); apperr.StatusOf(err, 0) != 409 {
		t.Fatalf("stale revision should be 409: %v", err)
	}

	retry, err := s.PrepareStoryRevision(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := s.ApplyStoryRevision(view.ID, retry, "門後傳來清楚的鐘聲。")
	if err != nil {
		t.Fatal(err)
	}
	if got := updated.Story[len(updated.Story)-2].Text; got != "門後傳來清楚的鐘聲。" {
		t.Fatalf("revised text = %q", got)
	}
}

func asActionIssues(err error, target **ActionIssuesError) bool {
	if e, ok := err.(*ActionIssuesError); ok {
		*target = e
		return true
	}
	return false
}

func TestStoryArcPhaseCompletion(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	id := view.ID

	prepared := preparedActions(t, s, id)
	if len(prepared.Input.ArcLines) == 0 || !strings.Contains(prepared.Input.ArcLines[0], "前期") {
		t.Fatalf("arc lines missing from prompt input: %+v", prepared.Input.ArcLines)
	}

	xpBefore := view.Players[0].Experience
	turn := &dm.Turn{
		Narration: "你們找到了失蹤的燈塔守。", Scene: "北岬燈塔",
		Objective: "護送燈塔守回鎮上", ObjectiveContext: "ctx", Stakes: "stakes",
		Choices: []dm.Choice{{Text: "出發"}},
		Combat:  dm.Combat{FirstTurn: "initiative", Enemies: []dm.Enemy{}},
		Arc:     dm.ArcSignal{PhaseComplete: true, NextGoal: "查明燈塔熄滅的幕後黑手"},
	}
	applied, err := s.ApplyDMTurn(id, prepared, turn)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	v := applied.View
	if v.StoryArc == nil {
		t.Fatal("view missing storyArc")
	}
	if v.StoryArc.Current != 1 {
		t.Fatalf("arc should advance to 中期: %+v", v.StoryArc)
	}
	first := v.StoryArc.Phases[0]
	if first.CompletedRound == 0 || !first.RewardGranted {
		t.Fatalf("phase 1 not stamped/rewarded: %+v", first)
	}
	// Scripted campaigns pin phase goals and deadlines to the module: the
	// AI-suggested nextGoal is overridden by stageObjectives, and deadlines
	// derive from stage size instead of the generic 20/40/60.
	if !strings.Contains(v.StoryArc.Phases[1].Goal, "下城區") {
		t.Fatalf("phase 2 goal should come from the module: %+v", v.StoryArc.Phases[1])
	}
	if v.StoryArc.Phases[0].DeadlineRound == 20 {
		t.Fatalf("deadline should be module-derived, not the generic 20: %+v", v.StoryArc.Phases[0])
	}
	if v.Players[0].Experience != xpBefore+first.RewardXP {
		t.Fatalf("timed reward XP missing: before %d after %d want +%d", xpBefore, v.Players[0].Experience, first.RewardXP)
	}
	var hasLog bool
	for _, e := range v.Story {
		if strings.Contains(e.Text, "階段達成") {
			hasLog = true
		}
	}
	if !hasLog {
		t.Fatal("missing 階段達成 journal entry")
	}
}

func TestLootWeaponBecomesAttack(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)
	id := view.ID
	prepared := preparedActions(t, s, id)

	turn := &dm.Turn{
		Narration: "冰層下露出一柄纏著霜紋的鈎矛。", Scene: "冰洞",
		Objective: "obj", ObjectiveContext: "ctx", Stakes: "stakes",
		Choices: []dm.Choice{{Text: "前進"}},
		Combat:  dm.Combat{FirstTurn: "initiative", Enemies: []dm.Enemy{}},
		Loot: dm.Loot{Gold: 30, Items: []dm.LootItem{
			{PlayerID: "player1", Name: "結霜鈎矛", Damage: "1d10", DamageType: "穿刺", Properties: []string{"雙手"}},
		}},
	}
	applied, err := s.ApplyDMTurn(id, prepared, turn)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	p := applied.View.Players[0]
	var loot *rules.Attack
	for i, a := range p.Attacks {
		if a.Name == "結霜鈎矛" {
			loot = &p.Attacks[i]
		}
	}
	if loot == nil {
		t.Fatalf("loot weapon attack missing: %+v", p.Attacks)
	}
	if loot.AttackBonus == 0 || !strings.HasPrefix(loot.Damage, "1d10") {
		t.Fatalf("loot weapon numbers not derived: %+v", loot)
	}
	found := false
	for _, e := range p.Equipment {
		if e == "結霜鈎矛" {
			found = true
		}
	}
	if !found {
		t.Fatalf("loot item missing from equipment: %v", p.Equipment)
	}
	if p.Gold != 100+15 { // 30 gold split across two players
		t.Fatalf("gold split wrong: %d", p.Gold)
	}
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
	// Freeform campaign (no scripted module): the DM may start combat itself.
	s := newTestService(t)
	view, err := s.Create(CreateParams{
		StoryID: "custom", Title: "自由冒險", Chapter: "第一章", Scene: "禮拜堂",
		Objective: "找到伊薩克", ObjectiveContext: "背景", Stakes: "風險", Opening: "門闔上了。",
		Players: []PlayerSeed{
			{Name: "賽勒恩", ClassName: "戰士"},
			{Name: "米芮", ClassName: "牧師", Level: 5},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
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

func TestClampCheckDCKeepsSuccessBetween30And90Percent(t *testing.T) {
	cases := []struct{ dc, modifier, want int }{
		{30, 2, 17},  // impossible DC pulled down to 30% success (needs 15)
		{25, 5, 20},  // high DC clamped to modifier+15
		{5, 6, 9},    // trivial DC raised to 90% success (needs 3)
		{12, 3, 12},  // in-range DC untouched
		{10, -1, 10}, // negative modifier, in range (needs 11)
		{20, 0, 15},  // no modifier: DC capped at 15
	}
	for _, c := range cases {
		if got := clampCheckDC(c.dc, c.modifier); got != c.want {
			t.Fatalf("clampCheckDC(%d, %d) = %d, want %d", c.dc, c.modifier, got, c.want)
		}
		needed := clampCheckDC(c.dc, c.modifier) - c.modifier
		if needed < 3 || needed > 15 {
			t.Fatalf("clampCheckDC(%d, %d): needed natural %d outside [3,15]", c.dc, c.modifier, needed)
		}
	}
}
