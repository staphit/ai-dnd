package store

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "images"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCampaignCRUD(t *testing.T) {
	s := openTestStore(t)

	c := CampaignRow{
		ID: "campaign-1", Title: "灰燼王冠", Scene: "邊境哨站", Round: 3,
		Objective: "找到失蹤的商隊", SetupComplete: true,
		Choices:    `[{"text":"追蹤足跡","playerId":"player1"}]`,
		Pending:    `{"player1":"我搜索房間"}`,
		Settings:   `{"selectedModel":"gpt-5"}`,
		DocVersion: 1, CreatedAt: 100, UpdatedAt: 200,
	}
	if err := s.SaveCampaign(c); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, ok, err := s.GetCampaign("campaign-1")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Title != c.Title || got.Round != 3 || !got.SetupComplete || got.Choices != c.Choices || got.Pending != c.Pending {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.RequiredCheck != "" {
		t.Fatalf("expected empty required check, got %q", got.RequiredCheck)
	}

	// Update preserves created_at, replaces fields.
	c.Round = 4
	c.RequiredCheck = `{"character":"阿爾","dc":15}`
	c.UpdatedAt = 300
	if err := s.SaveCampaign(c); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _, _ = s.GetCampaign("campaign-1")
	if got.Round != 4 || got.CreatedAt != 100 || got.UpdatedAt != 300 || got.RequiredCheck != c.RequiredCheck {
		t.Fatalf("update mismatch: %+v", got)
	}

	// Empty JSON columns fall back to their default documents.
	if err := s.SaveCampaign(CampaignRow{ID: "campaign-2", Title: "b", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatalf("save minimal: %v", err)
	}
	minimal, _, _ := s.GetCampaign("campaign-2")
	if minimal.Choices != "[]" || minimal.Pending != "{}" || minimal.Settings != "{}" {
		t.Fatalf("defaults mismatch: %+v", minimal)
	}

	list, err := s.ListCampaigns()
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].ID != "campaign-1" { // updated_at 300 > 1
		t.Fatalf("expected campaign-1 first, got %v", list[0].ID)
	}

	if _, ok, _ := s.GetCampaign("missing"); ok {
		t.Fatal("expected missing campaign")
	}
}

func TestSaveCampaignStateIsAtomicAndReplaceClearsDocuments(t *testing.T) {
	s := openTestStore(t)
	row := CampaignRow{ID: "atomic-1", Title: "原始", Round: 1, CreatedAt: 1, UpdatedAt: 1}

	// The invalid character fails after the campaign insert; the transaction
	// must roll that insert back as well.
	err := s.SaveCampaignState(CampaignStateWrite{
		Campaign:   row,
		Characters: []CharacterRow{{CampaignID: row.ID, Name: "缺少 ID", Data: `{}`, UpdatedAt: 1}},
	})
	if err == nil {
		t.Fatal("expected invalid character to fail")
	}
	if _, ok, getErr := s.GetCampaign(row.ID); getErr != nil || ok {
		t.Fatalf("partial campaign survived rollback: ok=%v err=%v", ok, getErr)
	}

	combat, arc, script := `{"active":true}`, `{"current":0}`, `{"scriptId":"demo"}`
	if err := s.SaveCampaignState(CampaignStateWrite{
		Campaign:   row,
		Characters: []CharacterRow{{CampaignID: row.ID, PlayerID: "player1", Name: "艾拉", Data: `{}`, UpdatedAt: 1}},
		Combat:     &combat, StoryArc: &arc, ScriptState: &script,
		Story: []StoryRow{{Speaker: "dm", Text: "舊故事", CreatedAt: 1}},
	}); err != nil {
		t.Fatalf("save full state: %v", err)
	}

	row.Title, row.UpdatedAt = "覆蓋後", 2
	if err := s.SaveCampaignState(CampaignStateWrite{
		Campaign: row, Replace: true,
		Story: []StoryRow{{Speaker: "dm", Text: "新故事", CreatedAt: 2}},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if chars, _ := s.Characters(row.ID); len(chars) != 0 {
		t.Fatalf("replace retained characters: %+v", chars)
	}
	if _, ok, _ := s.Combat(row.ID); ok {
		t.Fatal("replace retained combat")
	}
	if _, ok, _ := s.StoryArc(row.ID); ok {
		t.Fatal("replace retained story arc")
	}
	if _, ok, _ := s.ScriptState(row.ID); ok {
		t.Fatal("replace retained script state")
	}
	story, err := s.StoryTail(row.ID, 10)
	if err != nil || len(story) != 1 || story[0].Text != "新故事" || story[0].Seq != 1 {
		t.Fatalf("replace story mismatch: %+v err=%v", story, err)
	}
}

func TestCharacterAndCombatCRUD(t *testing.T) {
	s := openTestStore(t)

	if err := s.SaveCharacter(CharacterRow{CampaignID: "c1", PlayerID: "player2", Name: "莉恩", Data: `{"id":"player2"}`, UpdatedAt: 2}); err != nil {
		t.Fatalf("save char: %v", err)
	}
	if err := s.SaveCharacter(CharacterRow{CampaignID: "c1", PlayerID: "player1", Name: "阿爾", Data: `{"id":"player1"}`, UpdatedAt: 1}); err != nil {
		t.Fatalf("save char: %v", err)
	}
	chars, err := s.Characters("c1")
	if err != nil || len(chars) != 2 {
		t.Fatalf("characters: %v %v", chars, err)
	}
	if chars[0].PlayerID != "player1" || chars[1].PlayerID != "player2" {
		t.Fatalf("expected player-id order, got %+v", chars)
	}

	// Upsert replaces the document.
	if err := s.SaveCharacter(CharacterRow{CampaignID: "c1", PlayerID: "player1", Name: "阿爾", Data: `{"id":"player1","hp":10}`, UpdatedAt: 5}); err != nil {
		t.Fatalf("upsert char: %v", err)
	}
	chars, _ = s.Characters("c1")
	if chars[0].Data != `{"id":"player1","hp":10}` {
		t.Fatalf("upsert mismatch: %+v", chars[0])
	}

	if _, ok, _ := s.Combat("c1"); ok {
		t.Fatal("expected no combat yet")
	}
	if err := s.SaveCombat("c1", `{"active":true}`, 9); err != nil {
		t.Fatalf("save combat: %v", err)
	}
	data, ok, err := s.Combat("c1")
	if err != nil || !ok || data != `{"active":true}` {
		t.Fatalf("combat: %q %v %v", data, ok, err)
	}
	if err := s.DeleteCombat("c1"); err != nil {
		t.Fatalf("delete combat: %v", err)
	}
	if _, ok, _ := s.Combat("c1"); ok {
		t.Fatal("expected combat deleted")
	}
}

func TestStoryEntriesAndCascadeDelete(t *testing.T) {
	s := openTestStore(t)

	if err := s.SaveCampaign(CampaignRow{ID: "c1", Title: "t", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatalf("save campaign: %v", err)
	}
	if err := s.SaveCharacter(CharacterRow{CampaignID: "c1", PlayerID: "player1", Name: "n", Data: "{}", UpdatedAt: 1}); err != nil {
		t.Fatalf("save char: %v", err)
	}
	if err := s.SaveCombat("c1", "{}", 1); err != nil {
		t.Fatalf("save combat: %v", err)
	}
	entries := []StoryRow{
		{Speaker: "dm", Text: "夜幕降臨。", CreatedAt: 1},
		{Speaker: "player1", Text: "我點燃火把。", CreatedAt: 2},
		{Speaker: "dm", Audience: "player1", Text: "你聽見低語。", CreatedAt: 3},
	}
	if err := s.AppendStoryEntries("c1", entries); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := s.AppendStoryEntries("c1", []StoryRow{{Speaker: "system", Text: "戰鬥開始", CreatedAt: 4}}); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	tail, err := s.StoryTail("c1", 10)
	if err != nil || len(tail) != 4 {
		t.Fatalf("tail: %v %v", tail, err)
	}
	if tail[0].Seq != 1 || tail[3].Seq != 4 || tail[0].Text != "夜幕降臨。" {
		t.Fatalf("seq/order mismatch: %+v", tail)
	}
	if tail[2].Audience != "player1" || tail[0].Audience != "public" {
		t.Fatalf("audience mismatch: %+v", tail)
	}

	limited, _ := s.StoryTail("c1", 2)
	if len(limited) != 2 || limited[0].Seq != 3 {
		t.Fatalf("limit mismatch: %+v", limited)
	}

	if err := s.DeleteCampaign("c1"); err != nil {
		t.Fatalf("delete campaign: %v", err)
	}
	if _, ok, _ := s.GetCampaign("c1"); ok {
		t.Fatal("campaign should be gone")
	}
	if chars, _ := s.Characters("c1"); len(chars) != 0 {
		t.Fatal("characters should cascade")
	}
	if _, ok, _ := s.Combat("c1"); ok {
		t.Fatal("combat should cascade")
	}
	if tail, _ := s.StoryTail("c1", 10); len(tail) != 0 {
		t.Fatal("story should cascade")
	}
}
