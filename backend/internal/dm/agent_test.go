package dm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"dndduet/internal/provider"
)

const validTurnJSON = `{
	"narration":"隊伍推進，燭火搖曳。",
	"scene":"禮拜堂","objective":"找到伊薩克","objectiveContext":"線索指向地下","stakes":"午夜漲潮",
	"requiresCheck":false,"check":null,
	"choices":["搜索祭壇","檢查泥痕"],
	"effects":[],"privateMessages":[],
	"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},
	"actionIssues":[],"experienceAwards":[]
}`

func TestValidateDMTurnParsesValidOutput(t *testing.T) {
	turn, err := validateDMTurn(json.RawMessage(validTurnJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Scene != "禮拜堂" || len(turn.Choices) != 2 || turn.RequiresCheck {
		t.Errorf("unexpected turn: %+v", turn)
	}
	if turn.Check != nil {
		t.Errorf("check should be nil when requiresCheck is false")
	}
}

func TestValidateDMTurnRejectsMissingFields(t *testing.T) {
	cases := map[string]string{
		"沒有產生場景敘事":      `{"narration":"","scene":"s","objective":"o","objectiveContext":"c","stakes":"x","requiresCheck":false,"check":null,"choices":["a"],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`,
		"沒有回傳場景名稱":      `{"narration":"n","scene":"  ","objective":"o","objectiveContext":"c","stakes":"x","requiresCheck":false,"check":null,"choices":["a"],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`,
		"沒有回傳檢定狀態":      `{"narration":"n","scene":"s","objective":"o","objectiveContext":"c","stakes":"x","check":null,"choices":["a"],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`,
		"沒有提供下一步選項":     `{"narration":"n","scene":"s","objective":"o","objectiveContext":"c","stakes":"x","requiresCheck":false,"check":null,"choices":[],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`,
		"要求檢定但沒有提供檢定內容": `{"narration":"n","scene":"s","objective":"o","objectiveContext":"c","stakes":"x","requiresCheck":true,"check":null,"choices":["a"],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`,
	}
	for want, raw := range cases {
		_, err := validateDMTurn(json.RawMessage(raw))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("for %q: got err %v", want, err)
		}
	}
}

func TestValidateDMTurnCoercesCollections(t *testing.T) {
	raw := `{
		"narration":"n","scene":"s","objective":"o","objectiveContext":"c","stakes":"x",
		"requiresCheck":true,
		"check":{"character":"甲","ability":"敏捷","skill":"匿蹤","dc":13,"reason":"避開守衛"},
		"choices":["a"],
		"effects":[
			{"targetId":"player1","kind":"damage","amount":999,"condition":null,"reason":"陷阱"},
			{"targetId":"enemy1","kind":"damage","amount":5,"condition":null,"reason":"不該出現"},
			{"targetId":"player2","kind":"condition","amount":null,"condition":"中毒","reason":"毒氣"}
		],
		"privateMessages":[{"playerId":"player1","text":"你聞到硫磺味"},{"playerId":"nope","text":"drop"}],
		"combat":{"starts":true,"firstTurn":"enemy","enemies":[{"name":"哥布林","ac":13,"hp":7,"initiativeBonus":2,"attackBonus":4,"damage":"1d6+2","damageType":"刺擊"}]},
		"actionIssues":[
			{"playerId":"player1","message":"a"},{"playerId":"player2","message":"b"},
			{"playerId":"player3","message":"c"},{"playerId":"player4","message":"d"},
			{"playerId":"player1","message":"e"}
		],
		"experienceAwards":[{"playerId":"player1","amount":99999,"reason":"里程碑"},{"playerId":"bad","amount":10,"reason":"drop"}]
	}`
	turn, err := validateDMTurn(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Check == nil || turn.Check.DC != 13 || turn.Check.Character != "甲" {
		t.Errorf("check not parsed: %+v", turn.Check)
	}
	if len(turn.Effects) != 2 {
		t.Fatalf("effects: got %d, want 2 (enemy1 dropped)", len(turn.Effects))
	}
	if turn.Effects[0].Amount != 500 {
		t.Errorf("damage amount not clamped to 500: %d", turn.Effects[0].Amount)
	}
	if len(turn.PrivateMessages) != 1 {
		t.Errorf("privateMessages: got %d, want 1", len(turn.PrivateMessages))
	}
	if !turn.Combat.Starts || turn.Combat.FirstTurn != "enemy" || len(turn.Combat.Enemies) != 1 || turn.Combat.Enemies[0].AC != 13 {
		t.Errorf("combat not parsed: %+v", turn.Combat)
	}
	if len(turn.ActionIssues) != 4 {
		t.Errorf("actionIssues not capped at 4: got %d", len(turn.ActionIssues))
	}
	if len(turn.ExperienceAwards) != 1 || turn.ExperienceAwards[0].Amount != 10000 {
		t.Errorf("experienceAwards not clamped/filtered: %+v", turn.ExperienceAwards)
	}
}

// fakeAPI is a provider.API stub for the DM agent tests.
type fakeAPI struct {
	status    provider.Status
	responses []string
	calls     int
}

func (f *fakeAPI) Status(context.Context) provider.Status   { return f.status }
func (f *fakeAPI) Connect(context.Context, string) error    { return nil }
func (f *fakeAPI) ConnectionState() provider.ConnState      { return provider.ConnState{Alive: true} }
func (f *fakeAPI) NormalizeModel(v string) (string, error)  { return v, nil }
func (f *fakeAPI) NormalizeEffort(v string) (string, error) { return v, nil }
func (f *fakeAPI) EffortOptions() []provider.ModelOption    { return nil }
func (f *fakeAPI) Model() string                            { return f.status.Model }
func (f *fakeAPI) ModelOptions() []provider.ModelOption     { return nil }
func (f *fakeAPI) ImageModel() string                       { return "" }
func (f *fakeAPI) RunImageGeneration(context.Context, string, provider.ImageOpts) (string, error) {
	return "", nil
}
func (f *fakeAPI) RunStructured(_ context.Context, _ string, _ provider.StructuredOpts) (json.RawMessage, error) {
	i := f.calls
	f.calls++
	if i >= len(f.responses) {
		i = len(f.responses) - 1
	}
	return json.RawMessage(f.responses[i]), nil
}

func configured() provider.Status {
	return provider.Status{Configured: true, Provider: "test", Model: "test-model"}
}

func TestRunDungeonMasterReturnsValidatedTurn(t *testing.T) {
	api := &fakeAPI{status: configured(), responses: []string{validTurnJSON}}
	turn, err := RunDungeonMaster(context.Background(), api, "input", "", "", "schema.json", ".", "story1", RulesFull)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Scene != "禮拜堂" || api.calls != 1 {
		t.Errorf("turn=%+v calls=%d", turn, api.calls)
	}
}

func TestRunDungeonMasterRequiresLogin(t *testing.T) {
	api := &fakeAPI{status: provider.Status{Configured: false, Message: "尚未登入"}, responses: []string{validTurnJSON}}
	_, err := RunDungeonMaster(context.Background(), api, "input", "", "", "schema.json", ".", "story1", RulesFull)
	if err == nil || !strings.Contains(err.Error(), "尚未登入") {
		t.Errorf("expected login error, got %v", err)
	}
}

func TestRunDungeonMasterRetriesWhenCombatUndeclared(t *testing.T) {
	// Narration announces a monster lunging, but the first result omits combat
	// data; the agent must re-prompt and accept the corrected second result.
	firstNoCombat := `{"narration":"怪獸撲向隊伍，戰鬥現在開始。","scene":"s","objective":"o","objectiveContext":"c","stakes":"x","requiresCheck":false,"check":null,"choices":["a"],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`
	secondWithCombat := `{"narration":"怪獸撲向隊伍，戰鬥現在開始。","scene":"s","objective":"o","objectiveContext":"c","stakes":"x","requiresCheck":false,"check":null,"choices":["a"],"effects":[],"privateMessages":[],"combat":{"starts":true,"firstTurn":"enemy","enemies":[{"name":"哥布林","ac":13,"hp":7,"initiativeBonus":2,"attackBonus":4,"damage":"1d6+2","damageType":"刺擊"}]},"actionIssues":[],"experienceAwards":[]}`
	api := &fakeAPI{status: configured(), responses: []string{firstNoCombat, secondWithCombat}}
	turn, err := RunDungeonMaster(context.Background(), api, "input", "", "", "schema.json", ".", "story1", RulesFull)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api.calls != 2 {
		t.Errorf("expected 2 Codex calls (retry), got %d", api.calls)
	}
	if !turn.Combat.Starts || len(turn.Combat.Enemies) != 1 {
		t.Errorf("combat not corrected: %+v", turn.Combat)
	}
}

func TestRunDungeonMasterFailsWhenCombatNeverProvided(t *testing.T) {
	noCombat := `{"narration":"敵人突襲，戰鬥開始。","scene":"s","objective":"o","objectiveContext":"c","stakes":"x","requiresCheck":false,"check":null,"choices":["a"],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`
	api := &fakeAPI{status: configured(), responses: []string{noCombat, noCombat}}
	_, err := RunDungeonMaster(context.Background(), api, "input", "", "", "schema.json", ".", "story1", RulesFull)
	if err == nil || !strings.Contains(err.Error(), "沒有提供可建立戰鬥介面的敵人資料") {
		t.Errorf("expected combat-data error, got %v", err)
	}
	if api.calls != 2 {
		t.Errorf("expected 2 Codex calls, got %d", api.calls)
	}
}
