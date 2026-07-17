package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// migrateSchema upgrades legacy layouts that CREATE TABLE IF NOT EXISTS cannot
// reshape. The main break was the blob campaigns table (id/title/round/payload)
// used before server-authoritative structured campaigns.
func migrateSchema(db *sql.DB) error {
	// story_entries PK must be (campaign_id, seq) before blob migration rewrites journal rows.
	if err := migrateStoryEntriesPK(db); err != nil {
		return fmt.Errorf("migrate story_entries: %w", err)
	}
	if err := migrateCampaignsBlob(db); err != nil {
		return fmt.Errorf("migrate campaigns: %w", err)
	}
	return nil
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

func tableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// requiredCampaignColumns is the structured campaigns schema used by campaigns.go.
var requiredCampaignColumns = []string{
	"id", "title", "chapter", "scene", "round", "objective", "objective_context",
	"stakes", "setup_complete", "choices", "required_check", "pending",
	"image_prompt", "settings", "doc_version", "created_at", "updated_at",
}

func campaignsSchemaComplete(cols map[string]bool) bool {
	for _, c := range requiredCampaignColumns {
		// required_check is nullable and optional to have for reads via NullString,
		// but Save/Get always reference it — require it too.
		if !cols[c] {
			return false
		}
	}
	// Blob layout is never "complete" for the structured API.
	if cols["payload"] {
		return false
	}
	return true
}

// migrateCampaignsBlob rewrites campaigns from the payload-blob layout into the
// structured columns expected by campaigns.go / game.Service. Characters and
// combat are extracted from the blob when the dedicated tables are empty.
func migrateCampaignsBlob(db *sql.DB) error {
	ok, err := tableExists(db, "campaigns")
	if err != nil || !ok {
		return err
	}
	cols, err := tableColumns(db, "campaigns")
	if err != nil {
		return err
	}
	// Already on full structured schema.
	if campaignsSchemaComplete(cols) {
		return nil
	}
	// Partial / non-blob layout: ADD COLUMN for anything missing, then re-check.
	if !cols["payload"] {
		if err := ensureCampaignColumns(db, cols); err != nil {
			return err
		}
		cols, err = tableColumns(db, "campaigns")
		if err != nil {
			return err
		}
		if campaignsSchemaComplete(cols) {
			return nil
		}
		// Still incomplete (unexpected layout) — fall through only if payload appeared; else error.
		missing := []string{}
		for _, c := range requiredCampaignColumns {
			if !cols[c] {
				missing = append(missing, c)
			}
		}
		return fmt.Errorf("campaigns table still missing columns after ALTER: %v", missing)
	}

	log.Printf("store: migrating campaigns blob schema → structured columns")

	type legacyRow struct {
		ID        string
		Title     string
		UpdatedAt int64
		Round     int
		Payload   string
	}
	rows, err := db.Query(`SELECT id, title, updated_at, round, payload FROM campaigns`)
	if err != nil {
		return err
	}
	var legacy []legacyRow
	for rows.Next() {
		var r legacyRow
		if err := rows.Scan(&r.ID, &r.Title, &r.UpdatedAt, &r.Round, &r.Payload); err != nil {
			rows.Close()
			return err
		}
		legacy = append(legacy, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Swap tables so we can recreate the structured shape.
	if _, err := tx.Exec(`ALTER TABLE campaigns RENAME TO campaigns_blob_legacy`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
CREATE TABLE campaigns (
	id                TEXT PRIMARY KEY,
	title             TEXT NOT NULL,
	chapter           TEXT NOT NULL DEFAULT '',
	scene             TEXT NOT NULL DEFAULT '',
	round             INTEGER NOT NULL DEFAULT 1,
	objective         TEXT NOT NULL DEFAULT '',
	objective_context TEXT NOT NULL DEFAULT '',
	stakes            TEXT NOT NULL DEFAULT '',
	setup_complete    INTEGER NOT NULL DEFAULT 0,
	choices           TEXT NOT NULL DEFAULT '[]',
	required_check    TEXT,
	pending           TEXT NOT NULL DEFAULT '{}',
	image_prompt      TEXT NOT NULL DEFAULT '',
	settings          TEXT NOT NULL DEFAULT '{}',
	doc_version       INTEGER NOT NULL DEFAULT 1,
	created_at        INTEGER NOT NULL,
	updated_at        INTEGER NOT NULL
)`); err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	for _, r := range legacy {
		doc := map[string]any{}
		if strings.TrimSpace(r.Payload) != "" {
			_ = json.Unmarshal([]byte(r.Payload), &doc)
		}
		str := func(keys ...string) string {
			for _, k := range keys {
				if v, ok := doc[k].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
			return ""
		}
		title := str("title")
		if title == "" {
			title = r.Title
		}
		if title == "" {
			title = "未命名戰役"
		}
		round := r.Round
		if v, ok := doc["round"].(float64); ok && int(v) > 0 {
			round = int(v)
		}
		if round < 1 {
			round = 1
		}
		setup := 0
		if v, ok := doc["setupComplete"].(bool); ok && v {
			setup = 1
		}
		choices := "[]"
		if raw, err := json.Marshal(doc["choices"]); err == nil && string(raw) != "null" {
			choices = string(raw)
		}
		pending := "{}"
		if raw, err := json.Marshal(doc["pending"]); err == nil && string(raw) != "null" {
			pending = string(raw)
		}
		reqCheck := sql.NullString{}
		if doc["requiredCheck"] != nil {
			if raw, err := json.Marshal(doc["requiredCheck"]); err == nil && string(raw) != "null" {
				reqCheck = sql.NullString{String: string(raw), Valid: true}
			}
		}
		settings := "{}"
		// Flatten settings-like top-level fields used by the frontend Campaign.
		settingsObj := map[string]any{}
		for _, k := range []string{
			"storyId", "selectedModel", "selectedEffort", "imageBackend", "forgeSettings",
			"fontScale", "showStatHints", "autoSceneImages", "ttsEnabled", "dismissedTips",
			"sceneImages", "sceneImage", "dmProvider",
		} {
			if v, ok := doc[k]; ok && v != nil {
				settingsObj[k] = v
			}
		}
		if raw, err := json.Marshal(settingsObj); err == nil {
			settings = string(raw)
		}
		updatedAt := r.UpdatedAt
		if updatedAt == 0 {
			updatedAt = now
		}
		createdAt := updatedAt
		if _, err := tx.Exec(`
INSERT INTO campaigns (
	id, title, chapter, scene, round, objective, objective_context, stakes,
	setup_complete, choices, required_check, pending, image_prompt, settings,
	doc_version, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			r.ID, title, str("chapter"), str("scene"), round,
			str("objective"), str("objectiveContext"), str("stakes"),
			setup, choices, reqCheck, pending, str("imagePrompt"), settings,
			createdAt, updatedAt,
		); err != nil {
			return err
		}

		// Characters: only fill when empty for this campaign.
		var charCount int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM characters WHERE campaign_id = ?`, r.ID).Scan(&charCount)
		if charCount == 0 {
			if arr, ok := doc["players"].([]any); ok {
				for _, item := range arr {
					raw, err := json.Marshal(item)
					if err != nil {
						continue
					}
					var head struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}
					_ = json.Unmarshal(raw, &head)
					pid := strings.TrimSpace(head.ID)
					name := strings.TrimSpace(head.Name)
					if pid == "" {
						continue
					}
					if name == "" {
						name = pid
					}
					if _, err := tx.Exec(
						`INSERT OR REPLACE INTO characters (campaign_id, player_id, name, data, updated_at) VALUES (?, ?, ?, ?, ?)`,
						r.ID, pid, name, string(raw), updatedAt,
					); err != nil {
						return err
					}
				}
			}
		}

		// Combat blob.
		var combatCount int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM combats WHERE campaign_id = ?`, r.ID).Scan(&combatCount)
		if combatCount == 0 && doc["combat"] != nil {
			if raw, err := json.Marshal(doc["combat"]); err == nil && string(raw) != "null" {
				if _, err := tx.Exec(
					`INSERT OR REPLACE INTO combats (campaign_id, data, updated_at) VALUES (?, ?, ?)`,
					r.ID, string(raw), updatedAt,
				); err != nil {
					return err
				}
			}
		}

		// Prefer payload.story when present (authoritative journal from blob era).
		if arr, ok := doc["story"].([]any); ok && len(arr) > 0 {
			if _, err := tx.Exec(`DELETE FROM story_entries WHERE campaign_id = ?`, r.ID); err != nil {
				return err
			}
			for i, item := range arr {
				raw, err := json.Marshal(item)
				if err != nil {
					continue
				}
				var e struct {
					Speaker  string `json:"speaker"`
					Text     string `json:"text"`
					Audience string `json:"audience"`
					Time     string `json:"time"`
					TimeLabel string `json:"timeLabel"`
				}
				_ = json.Unmarshal(raw, &e)
				audience := strings.TrimSpace(e.Audience)
				if audience == "" {
					audience = "public"
				}
				label := strings.TrimSpace(e.TimeLabel)
				if label == "" {
					label = strings.TrimSpace(e.Time)
				}
				if _, err := tx.Exec(
					`INSERT INTO story_entries (campaign_id, seq, speaker, audience, text, time_label, created_at)
					 VALUES (?, ?, ?, ?, ?, ?, ?)`,
					r.ID, i, e.Speaker, audience, e.Text, label, updatedAt,
				); err != nil {
					// Old story_entries PK may still be (campaign_id, entry_id); rebuild later.
					return err
				}
			}
		}
	}

	// Keep a copy of the blob table for recovery, but drop after successful migrate
	// to avoid confusion. Operators can restore from backups if needed.
	if _, err := tx.Exec(`DROP TABLE campaigns_blob_legacy`); err != nil {
		return err
	}
	return tx.Commit()
}

// ensureCampaignColumns adds missing structured columns for partially upgraded DBs.
// It always re-reads nothing from the caller — use the provided cols snapshot and
// mark additions locally so repeated calls in one process are safe.
func ensureCampaignColumns(db *sql.DB, cols map[string]bool) error {
	if cols == nil {
		var err error
		cols, err = tableColumns(db, "campaigns")
		if err != nil {
			return err
		}
	}
	alters := []struct {
		col string
		ddl string
	}{
		{"title", `ALTER TABLE campaigns ADD COLUMN title TEXT NOT NULL DEFAULT ''`},
		{"chapter", `ALTER TABLE campaigns ADD COLUMN chapter TEXT NOT NULL DEFAULT ''`},
		{"scene", `ALTER TABLE campaigns ADD COLUMN scene TEXT NOT NULL DEFAULT ''`},
		{"round", `ALTER TABLE campaigns ADD COLUMN round INTEGER NOT NULL DEFAULT 1`},
		{"objective", `ALTER TABLE campaigns ADD COLUMN objective TEXT NOT NULL DEFAULT ''`},
		{"objective_context", `ALTER TABLE campaigns ADD COLUMN objective_context TEXT NOT NULL DEFAULT ''`},
		{"stakes", `ALTER TABLE campaigns ADD COLUMN stakes TEXT NOT NULL DEFAULT ''`},
		{"setup_complete", `ALTER TABLE campaigns ADD COLUMN setup_complete INTEGER NOT NULL DEFAULT 0`},
		{"choices", `ALTER TABLE campaigns ADD COLUMN choices TEXT NOT NULL DEFAULT '[]'`},
		{"required_check", `ALTER TABLE campaigns ADD COLUMN required_check TEXT`},
		{"pending", `ALTER TABLE campaigns ADD COLUMN pending TEXT NOT NULL DEFAULT '{}'`},
		{"image_prompt", `ALTER TABLE campaigns ADD COLUMN image_prompt TEXT NOT NULL DEFAULT ''`},
		{"settings", `ALTER TABLE campaigns ADD COLUMN settings TEXT NOT NULL DEFAULT '{}'`},
		{"doc_version", `ALTER TABLE campaigns ADD COLUMN doc_version INTEGER NOT NULL DEFAULT 1`},
		{"created_at", `ALTER TABLE campaigns ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0`},
		{"updated_at", `ALTER TABLE campaigns ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0`},
	}
	for _, a := range alters {
		if cols[a.col] {
			continue
		}
		if _, err := db.Exec(a.ddl); err != nil {
			// Ignore "duplicate column" races if another process altered first.
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				cols[a.col] = true
				continue
			}
			return fmt.Errorf("add column %s: %w", a.col, err)
		}
		cols[a.col] = true
		log.Printf("store: added campaigns.%s", a.col)
	}
	return nil
}

// migrateStoryEntriesPK rebuilds story_entries when it still uses the blob-era
// PRIMARY KEY (campaign_id, entry_id).
func migrateStoryEntriesPK(db *sql.DB) error {
	ok, err := tableExists(db, "story_entries")
	if err != nil || !ok {
		return err
	}
	cols, err := tableColumns(db, "story_entries")
	if err != nil {
		return err
	}
	if !cols["entry_id"] {
		return nil
	}
	log.Printf("store: migrating story_entries (entry_id PK → seq PK)")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`ALTER TABLE story_entries RENAME TO story_entries_legacy`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
CREATE TABLE story_entries (
	campaign_id TEXT NOT NULL,
	seq         INTEGER NOT NULL,
	speaker     TEXT NOT NULL,
	audience    TEXT NOT NULL DEFAULT 'public',
	text        TEXT NOT NULL,
	time_label  TEXT NOT NULL DEFAULT '',
	created_at  INTEGER NOT NULL,
	PRIMARY KEY (campaign_id, seq)
)`); err != nil {
		return err
	}
	// Portable path (no window functions): renumber in Go.
	return migrateStoryEntriesPKFallback(tx)
}

func migrateStoryEntriesPKFallback(tx *sql.Tx) error {
	// Called when the window-function INSERT failed; table is already renamed.
	rows, err := tx.Query(`
SELECT campaign_id, entry_id, seq, speaker, audience, text, time_label, created_at
FROM story_entries_legacy ORDER BY campaign_id, seq, entry_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		campaignID, entryID, speaker, audience, text, timeLabel string
		seq, createdAt                                          int64
	}
	var all []row
	for rows.Next() {
		var r row
		var audience sql.NullString
		if err := rows.Scan(&r.campaignID, &r.entryID, &r.seq, &r.speaker, &audience, &r.text, &r.timeLabel, &r.createdAt); err != nil {
			return err
		}
		r.audience = audience.String
		if r.audience == "" {
			r.audience = "public"
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Ensure destination table exists (window path may have failed mid-way).
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS story_entries (
	campaign_id TEXT NOT NULL,
	seq         INTEGER NOT NULL,
	speaker     TEXT NOT NULL,
	audience    TEXT NOT NULL DEFAULT 'public',
	text        TEXT NOT NULL,
	time_label  TEXT NOT NULL DEFAULT '',
	created_at  INTEGER NOT NULL,
	PRIMARY KEY (campaign_id, seq)
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM story_entries`); err != nil {
		return err
	}

	seqBy := map[string]int{}
	for _, r := range all {
		seq := seqBy[r.campaignID]
		seqBy[r.campaignID] = seq + 1
		if _, err := tx.Exec(
			`INSERT INTO story_entries (campaign_id, seq, speaker, audience, text, time_label, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			r.campaignID, seq, r.speaker, r.audience, r.text, r.timeLabel, r.createdAt,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS story_entries_legacy`); err != nil {
		return err
	}
	return tx.Commit()
}
