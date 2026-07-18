package game

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dndduet/internal/apperr"
	"dndduet/internal/rules"
	"dndduet/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "images"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	fixed := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	return New(st, func() time.Time { return fixed })
}

func createSample(t *testing.T, s *Service) View {
	t.Helper()
	view, err := s.Create(CreateParams{
		StoryID: "ashen-crown", Title: "灰燼王冠", Chapter: "第一章", Scene: "禮拜堂",
		Objective: "找到伊薩克", ObjectiveContext: "背景", Stakes: "風險", Opening: "門闔上了。",
		Players: []PlayerSeed{
			{Name: "賽勒恩", ClassName: "戰士"},
			{Name: "米芮", ClassName: "牧師", Level: 5},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return view
}

func TestCreateAndView(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)

	if !strings.HasPrefix(view.ID, "campaign-") || !view.SetupComplete || view.Round != 1 {
		t.Fatalf("campaign meta wrong: %+v", view)
	}
	if len(view.Players) != 2 || view.Players[0].ID != "player1" || view.Players[1].ID != "player2" {
		t.Fatalf("players wrong: %+v", view.Players)
	}
	want := rules.CreateConfiguredCharacter("player2", "米芮", "牧師", rules.BuildOptions{Level: 5})
	got := view.Players[1]
	if got.MaxHP != want.MaxHP || got.ProficiencyBonus != want.ProficiencyBonus || len(got.Spellcasting.Slots) != len(want.Spellcasting.Slots) {
		t.Fatalf("derived stats mismatch: got %d/%d prof %d, want %d/%d prof %d", got.HP, got.MaxHP, got.ProficiencyBonus, want.HP, want.MaxHP, want.ProficiencyBonus)
	}
	if len(view.Story) != 2 || view.Story[0].Speaker != "dm" || !strings.Contains(view.Story[1].Text, "隊伍已建立，共 2 位冒險者") {
		t.Fatalf("story seed wrong: %+v", view.Story)
	}
	if _, ok := view.XPProgress["player2"]; !ok {
		t.Fatalf("xpProgress missing: %+v", view.XPProgress)
	}
	if !strings.Contains(string(view.Settings), "ashen-crown") {
		t.Fatalf("settings missing storyId: %s", view.Settings)
	}

	again, err := s.View(view.ID)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if len(again.Players) != 2 || again.Players[1].MaxHP != want.MaxHP {
		t.Fatalf("reloaded view mismatch")
	}

	if _, err := s.Create(CreateParams{Title: "x"}); apperr.StatusOf(err, 0) != 400 {
		t.Fatalf("empty party should 400, got %v", err)
	}
	if _, err := s.Create(CreateParams{ID: view.ID, Title: "dup", Players: []PlayerSeed{{Name: "a", ClassName: "戰士"}}}); apperr.StatusOf(err, 0) != 409 {
		t.Fatalf("duplicate id should 409, got %v", err)
	}
	if _, err := s.View("campaign-missing"); apperr.StatusOf(err, 0) != 404 {
		t.Fatalf("missing view should 404, got %v", err)
	}
}

func TestImportNormalizesAndOverwrites(t *testing.T) {
	s := newTestService(t)

	full := rules.CreateLevel3Character("playerX", "多蘭", "法師")
	full.Experience = 0
	full.ClassLevels = nil
	full.Spellcasting.Spells[0].Description = "OLD TEXT"
	fullJSON, _ := json.Marshal(full)

	campaign := map[string]any{
		"id": "campaign-import-1", "title": "舊檔", "setupComplete": true, "round": 7,
		"players":       []any{json.RawMessage(fullJSON), map[string]any{"name": "老兵", "className": "某某戰士流派", "hp": 5}},
		"story":         []any{map[string]any{"id": "s1", "speaker": "dm", "text": "舊事", "time": "23:41"}},
		"choices":       []any{"裸字串選項", map[string]any{"text": "帶標籤", "playerId": "player1"}},
		"pending":       map[string]any{"player1": "我行動"},
		"requiredCheck": map[string]any{"character": "多蘭", "ability": "智力", "skill": "調查", "dc": 15, "reason": "r"},
		"combat":        map[string]any{"active": true, "round": 2, "turnIndex": 0, "combatants": []any{}},
		"selectedModel": "gpt-5", "fontScale": 2.0, "sceneImage": map[string]any{"url": "/generated/a.png"},
	}
	raw, _ := json.Marshal(map[string]any{"format": "dnd-duet-campaign", "version": 2, "campaign": campaign})

	view, err := s.Import(raw, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if view.ID != "campaign-import-1" || view.Round != 7 {
		t.Fatalf("meta wrong: %+v", view)
	}
	p1 := view.Players[0]
	if p1.ID != "player1" || p1.Spellcasting == nil {
		t.Fatalf("player1 wrong: %+v", p1)
	}
	if p1.Spellcasting.Spells[0].Description == "OLD TEXT" {
		t.Fatal("catalog refresh did not restore spell description")
	}
	if p1.Experience != rules.ExperienceForLevel(3) {
		t.Fatalf("experience backfill wrong: %d", p1.Experience)
	}
	if len(p1.ClassLevels) != 1 || p1.ClassLevels[0].ClassName != "法師" {
		t.Fatalf("classLevels backfill wrong: %+v", p1.ClassLevels)
	}
	p2 := view.Players[1]
	if p2.ClassName != "戰士" || p2.HP != 5 || p2.Level != 3 {
		t.Fatalf("legacy fallback wrong: %s %d/%d L%d", p2.ClassName, p2.HP, p2.MaxHP, p2.Level)
	}
	if len(view.Story) != 1 || view.Story[0].Time != "23:41" {
		t.Fatalf("story labels wrong: %+v", view.Story)
	}
	if len(view.Choices) != 2 || view.Choices[0].Text != "裸字串選項" || view.Choices[1].PlayerID != "player1" {
		t.Fatalf("choices wrong: %+v", view.Choices)
	}
	if view.RequiredCheck == nil || view.RequiredCheck.DC != 15 {
		t.Fatalf("requiredCheck wrong: %+v", view.RequiredCheck)
	}
	if view.Combat == nil || !view.Combat.Active {
		t.Fatalf("combat missing: %+v", view.Combat)
	}
	var settings map[string]any
	json.Unmarshal(view.Settings, &settings)
	if settings["selectedModel"] != "gpt-5" || settings["fontScale"] != 1.25 || settings["showStatHints"] != true {
		t.Fatalf("settings wrong: %v", settings)
	}
	if _, ok := settings["sceneImages"]; !ok {
		t.Fatalf("sceneImage not wrapped: %v", settings)
	}

	if _, err := s.Import(raw, false); apperr.StatusOf(err, 0) != 409 {
		t.Fatalf("re-import should 409, got %v", err)
	}
	if _, err := s.Import(raw, true); err != nil {
		t.Fatalf("overwrite import: %v", err)
	}

	for _, bad := range []string{`not json`, `{"title":1}`, `{"title":"t","players":{},"story":[]}`} {
		if _, err := s.Import([]byte(bad), false); apperr.StatusOf(err, 0) != 400 {
			t.Fatalf("bad import %q should 400, got %v", bad, err)
		}
	}
}

func TestExportRoundTrip(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)

	data, err := s.Export(view.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil || env["format"] != "dnd-duet-campaign" {
		t.Fatalf("envelope wrong: %v %v", env["format"], err)
	}
	campaign := env["campaign"].(map[string]any)
	if campaign["storyId"] != "ashen-crown" {
		t.Fatalf("settings not flattened: %v", campaign["storyId"])
	}
	if _, has := campaign["settings"]; has {
		t.Fatal("settings key should be removed")
	}
	if _, has := campaign["xpProgress"]; has {
		t.Fatal("xpProgress key should be removed")
	}

	imported, err := s.Import(data, true)
	if err != nil {
		t.Fatalf("re-import export: %v", err)
	}
	before, _ := json.Marshal(view.Players)
	after, _ := json.Marshal(imported.Players)
	if string(before) != string(after) {
		t.Fatalf("players not preserved through export/import:\n%s\n%s", before, after)
	}
}

func TestUpdateSettingsAndDelete(t *testing.T) {
	s := newTestService(t)
	view := createSample(t, s)

	updated, err := s.UpdateSettings(view.ID, json.RawMessage(`{"selectedModel":"gpt-5.1"}`))
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
	var settings map[string]any
	json.Unmarshal(updated.Settings, &settings)
	if settings["selectedModel"] != "gpt-5.1" || settings["storyId"] != "ashen-crown" {
		t.Fatalf("merge lost keys: %v", settings)
	}

	if err := s.Delete(view.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.View(view.ID); apperr.StatusOf(err, 0) != 404 {
		t.Fatalf("view after delete should 404, got %v", err)
	}
}
