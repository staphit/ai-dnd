package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateCampaignsBlobSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")
	imageDir := filepath.Join(dir, "images")

	// Build a legacy blob-era database without going through Open's migrate.
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	_, err = raw.Exec(`
CREATE TABLE campaigns (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL,
	round INTEGER NOT NULL DEFAULT 1,
	payload TEXT NOT NULL
);
CREATE TABLE characters (
	campaign_id TEXT NOT NULL,
	player_id TEXT NOT NULL,
	name TEXT NOT NULL,
	data TEXT NOT NULL,
	updated_at INTEGER NOT NULL,
	PRIMARY KEY (campaign_id, player_id)
);
CREATE TABLE combats (
	campaign_id TEXT PRIMARY KEY,
	data TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE TABLE story_entries (
	campaign_id TEXT NOT NULL,
	entry_id TEXT NOT NULL,
	seq INTEGER NOT NULL,
	speaker TEXT NOT NULL,
	text TEXT NOT NULL,
	audience TEXT NOT NULL DEFAULT 'public',
	time_label TEXT NOT NULL DEFAULT '',
	scene_slot_id TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	PRIMARY KEY (campaign_id, entry_id)
);
`)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{
		"id": "c1", "title": "測試", "chapter": "第一章", "scene": "碼頭",
		"round": 3, "objective": "找到燈塔", "objectiveContext": "背景", "stakes": "風險",
		"setupComplete": true,
		"pending":       map[string]string{"player1": "搜索"},
		"choices":       []map[string]string{{"text": "往北", "playerId": "player1"}},
		"players": []map[string]any{
			{"id": "player1", "name": "艾拉", "className": "遊俠", "level": 3},
		},
		"story": []map[string]any{
			{"id": "s1", "speaker": "dm", "text": "海風很冷。", "time": "10:00", "audience": "public"},
			{"id": "s2", "speaker": "player1", "text": "我搜索碼頭。", "time": "10:01", "audience": "public"},
		},
		"combat":      map[string]any{"active": false, "round": 0, "turnIndex": 0, "combatants": []any{}},
		"imagePrompt": "foggy dock",
	})
	_, err = raw.Exec(
		`INSERT INTO campaigns (id, title, updated_at, round, payload) VALUES (?, ?, ?, ?, ?)`,
		"c1", "測試", 1000, 3, string(payload),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(
		`INSERT INTO story_entries (campaign_id, entry_id, seq, speaker, text, audience, time_label, created_at)
		 VALUES ('c1', 'e1', 0, 'dm', 'old row', 'public', '09:00', 900)`,
	)
	if err != nil {
		t.Fatal(err)
	}
	raw.Close()

	st, err := Open(dbPath, imageDir)
	if err != nil {
		t.Fatalf("open migrate: %v", err)
	}
	defer st.Close()

	c, ok, err := st.GetCampaign("c1")
	if err != nil || !ok {
		t.Fatalf("get campaign: ok=%v err=%v", ok, err)
	}
	if c.Scene != "碼頭" || c.Chapter != "第一章" || c.Round != 3 || !c.SetupComplete {
		t.Fatalf("campaign fields: %+v", c)
	}
	if c.Objective != "找到燈塔" || c.ImagePrompt != "foggy dock" {
		t.Fatalf("campaign objective/prompt: %+v", c)
	}
	chars, err := st.Characters("c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 1 || chars[0].Name != "艾拉" {
		t.Fatalf("characters: %+v", chars)
	}
	combat, ok, err := st.Combat("c1")
	if err != nil || !ok {
		t.Fatalf("combat: ok=%v err=%v", ok, err)
	}
	if combat == "" {
		t.Fatal("empty combat")
	}
	story, err := st.StoryTail("c1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(story) != 2 || story[0].Text != "海風很冷。" {
		t.Fatalf("story: %+v", story)
	}

	// Second open is a no-op migrate.
	st2, err := Open(dbPath, imageDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	st2.Close()
}

// Partial upgrades that only added some columns (e.g. scene) used to skip
// ensureCampaignColumns and leave chapter missing.
func TestMigratePartialCampaignColumns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "partial.db")
	imageDir := filepath.Join(dir, "images")

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	_, err = raw.Exec(`
CREATE TABLE campaigns (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL DEFAULT '',
	scene TEXT NOT NULL DEFAULT '',
	round INTEGER NOT NULL DEFAULT 1,
	updated_at INTEGER NOT NULL DEFAULT 0
);
INSERT INTO campaigns (id, title, scene, round, updated_at) VALUES ('c2', '半套', '山洞', 2, 50);
`)
	if err != nil {
		t.Fatal(err)
	}
	raw.Close()

	st, err := Open(dbPath, imageDir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	c, ok, err := st.GetCampaign("c2")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if c.Scene != "山洞" || c.Title != "半套" || c.Round != 2 {
		t.Fatalf("fields: %+v", c)
	}
	// chapter must exist (empty is fine) — GetCampaign scans it.
	if c.Chapter != "" && c.Chapter != c.Chapter {
		t.Fatal("unreachable")
	}
}
