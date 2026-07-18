package store

import (
	"database/sql"
	"errors"
	"strings"
)

// CampaignRow is one campaigns table row. The JSON document columns (Choices,
// Pending, Settings, RequiredCheck) are stored as raw JSON strings; the game
// layer owns their shapes.
type CampaignRow struct {
	ID               string
	Title            string
	Chapter          string
	Scene            string
	Round            int
	Objective        string
	ObjectiveContext string
	Stakes           string
	SetupComplete    bool
	Choices          string // JSON array
	RequiredCheck    string // JSON object, "" when none
	Pending          string // JSON object playerId -> action text
	ImagePrompt      string
	Settings         string // JSON object
	DocVersion       int
	CreatedAt        int64
	UpdatedAt        int64
}

// CampaignSummary is the list view of a campaign.
type CampaignSummary struct {
	ID        string
	Title     string
	Scene     string
	Round     int
	UpdatedAt int64
}

// CharacterRow is one characters table row; Data is the rules.Character JSON document.
type CharacterRow struct {
	CampaignID string
	PlayerID   string
	Name       string
	Data       string
	UpdatedAt  int64
}

// StoryRow is one story_entries table row (the full-fidelity UI journal,
// distinct from memory_events which feeds the AI memory pipeline).
type StoryRow struct {
	Seq       int64
	Speaker   string
	Audience  string
	Text      string
	TimeLabel string
	CreatedAt int64
}

// SaveCampaign upserts a campaign row. CreatedAt is preserved on update.
func (s *Store) SaveCampaign(c CampaignRow) error {
	if c.ID == "" {
		return errors.New("campaign id is required")
	}
	requiredCheck := sql.NullString{String: c.RequiredCheck, Valid: c.RequiredCheck != ""}
	_, err := s.db.Exec(
		`INSERT INTO campaigns (id, title, chapter, scene, round, objective, objective_context, stakes,
			setup_complete, choices, required_check, pending, image_prompt, settings, doc_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			title = excluded.title, chapter = excluded.chapter, scene = excluded.scene, round = excluded.round,
			objective = excluded.objective, objective_context = excluded.objective_context, stakes = excluded.stakes,
			setup_complete = excluded.setup_complete, choices = excluded.choices, required_check = excluded.required_check,
			pending = excluded.pending, image_prompt = excluded.image_prompt, settings = excluded.settings,
			doc_version = excluded.doc_version, updated_at = excluded.updated_at`,
		c.ID, c.Title, c.Chapter, c.Scene, c.Round, c.Objective, c.ObjectiveContext, c.Stakes,
		boolToInt(c.SetupComplete), jsonOr(c.Choices, "[]"), requiredCheck, jsonOr(c.Pending, "{}"),
		c.ImagePrompt, jsonOr(c.Settings, "{}"), c.DocVersion, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// GetCampaign returns the campaign row with the given id.
func (s *Store) GetCampaign(id string) (CampaignRow, bool, error) {
	row := s.db.QueryRow(
		`SELECT id, title, chapter, scene, round, objective, objective_context, stakes,
			setup_complete, choices, required_check, pending, image_prompt, settings, doc_version, created_at, updated_at
		 FROM campaigns WHERE id = ?`, id)
	var c CampaignRow
	var setupComplete int
	var requiredCheck sql.NullString
	err := row.Scan(&c.ID, &c.Title, &c.Chapter, &c.Scene, &c.Round, &c.Objective, &c.ObjectiveContext, &c.Stakes,
		&setupComplete, &c.Choices, &requiredCheck, &c.Pending, &c.ImagePrompt, &c.Settings, &c.DocVersion, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return CampaignRow{}, false, nil
	}
	if err != nil {
		return CampaignRow{}, false, err
	}
	c.SetupComplete = setupComplete != 0
	c.RequiredCheck = requiredCheck.String
	return c, true, nil
}

// ListCampaigns returns campaign summaries, most recently updated first.
func (s *Store) ListCampaigns() ([]CampaignSummary, error) {
	rows, err := s.db.Query(`SELECT id, title, scene, round, updated_at FROM campaigns ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CampaignSummary
	for rows.Next() {
		var c CampaignSummary
		if err := rows.Scan(&c.ID, &c.Title, &c.Scene, &c.Round, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteCampaign removes a campaign and all dependent rows (characters,
// combat, story journal). Memory events are left untouched: they belong to
// the AI memory pipeline and are keyed by the same id if reimported later.
func (s *Store) DeleteCampaign(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM story_entries WHERE campaign_id = ?`,
		`DELETE FROM combats WHERE campaign_id = ?`,
		`DELETE FROM combat_snapshots WHERE campaign_id = ?`,
		`DELETE FROM story_arcs WHERE campaign_id = ?`,
		`DELETE FROM script_states WHERE campaign_id = ?`,
		`DELETE FROM characters WHERE campaign_id = ?`,
		`DELETE FROM campaigns WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SaveCharacter upserts one character document.
func (s *Store) SaveCharacter(c CharacterRow) error {
	if c.CampaignID == "" || c.PlayerID == "" {
		return errors.New("campaign id and player id are required")
	}
	_, err := s.db.Exec(
		`INSERT INTO characters (campaign_id, player_id, name, data, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(campaign_id, player_id) DO UPDATE SET name = excluded.name, data = excluded.data, updated_at = excluded.updated_at`,
		c.CampaignID, c.PlayerID, c.Name, c.Data, c.UpdatedAt,
	)
	return err
}

// Characters returns all character rows for a campaign ordered by player id
// (player1..player4).
func (s *Store) Characters(campaignID string) ([]CharacterRow, error) {
	rows, err := s.db.Query(
		`SELECT campaign_id, player_id, name, data, updated_at FROM characters WHERE campaign_id = ? ORDER BY player_id ASC`,
		campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CharacterRow
	for rows.Next() {
		var c CharacterRow
		if err := rows.Scan(&c.CampaignID, &c.PlayerID, &c.Name, &c.Data, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteCharacter removes one character document.
func (s *Store) DeleteCharacter(campaignID, playerID string) error {
	_, err := s.db.Exec(`DELETE FROM characters WHERE campaign_id = ? AND player_id = ?`, campaignID, playerID)
	return err
}

// SaveCombat upserts the active combat document for a campaign.
func (s *Store) SaveCombat(campaignID, data string, updatedAt int64) error {
	if campaignID == "" {
		return errors.New("campaign id is required")
	}
	_, err := s.db.Exec(
		`INSERT INTO combats (campaign_id, data, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(campaign_id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		campaignID, data, updatedAt,
	)
	return err
}

// Combat returns the combat document for a campaign; ok is false when none.
func (s *Store) Combat(campaignID string) (string, bool, error) {
	row := s.db.QueryRow(`SELECT data FROM combats WHERE campaign_id = ?`, campaignID)
	var data string
	err := row.Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return data, true, nil
}

// DeleteCombat clears the combat document for a campaign.
func (s *Store) DeleteCombat(campaignID string) error {
	_, err := s.db.Exec(`DELETE FROM combats WHERE campaign_id = ?`, campaignID)
	return err
}

// SaveCombatSnapshot upserts the combat-start snapshot for a campaign.
func (s *Store) SaveCombatSnapshot(campaignID, data string, updatedAt int64) error {
	if campaignID == "" {
		return errors.New("campaign id is required")
	}
	_, err := s.db.Exec(
		`INSERT INTO combat_snapshots (campaign_id, data, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(campaign_id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		campaignID, data, updatedAt,
	)
	return err
}

// CombatSnapshot returns the combat-start snapshot; ok is false when none.
func (s *Store) CombatSnapshot(campaignID string) (string, bool, error) {
	row := s.db.QueryRow(`SELECT data FROM combat_snapshots WHERE campaign_id = ?`, campaignID)
	var data string
	err := row.Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return data, true, nil
}

// DeleteCombatSnapshot clears the combat-start snapshot for a campaign.
func (s *Store) DeleteCombatSnapshot(campaignID string) error {
	_, err := s.db.Exec(`DELETE FROM combat_snapshots WHERE campaign_id = ?`, campaignID)
	return err
}

// SaveStoryArc upserts the story-pacing arc for a campaign.
func (s *Store) SaveStoryArc(campaignID, data string, updatedAt int64) error {
	if campaignID == "" {
		return errors.New("campaign id is required")
	}
	_, err := s.db.Exec(
		`INSERT INTO story_arcs (campaign_id, data, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(campaign_id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		campaignID, data, updatedAt,
	)
	return err
}

// StoryArc returns the story-pacing arc; ok is false when none.
func (s *Store) StoryArc(campaignID string) (string, bool, error) {
	row := s.db.QueryRow(`SELECT data FROM story_arcs WHERE campaign_id = ?`, campaignID)
	var data string
	err := row.Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return data, true, nil
}

// SaveScriptState upserts the scripted-module progress for a campaign.
func (s *Store) SaveScriptState(campaignID, data string, updatedAt int64) error {
	if campaignID == "" {
		return errors.New("campaign id is required")
	}
	_, err := s.db.Exec(
		`INSERT INTO script_states (campaign_id, data, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(campaign_id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		campaignID, data, updatedAt,
	)
	return err
}

// ScriptState returns the scripted-module progress; ok is false when none.
func (s *Store) ScriptState(campaignID string) (string, bool, error) {
	row := s.db.QueryRow(`SELECT data FROM script_states WHERE campaign_id = ?`, campaignID)
	var data string
	err := row.Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return data, true, nil
}

// AppendStoryEntries appends journal entries, assigning per-campaign
// monotonically increasing seq numbers (same pattern as AppendMemoryEvents).
func (s *Store) AppendStoryEntries(campaignID string, entries []StoryRow) error {
	if campaignID == "" || len(entries) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var next int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM story_entries WHERE campaign_id = ?`, campaignID).Scan(&next); err != nil {
		return err
	}
	for _, e := range entries {
		next++
		audience := e.Audience
		if audience == "" {
			audience = "public"
		}
		if _, err := tx.Exec(
			`INSERT INTO story_entries (campaign_id, seq, speaker, audience, text, time_label, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			campaignID, next, e.Speaker, audience, e.Text, e.TimeLabel, e.CreatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReplaceLastPublicDMText rewrites the most recent public DM journal entry's text.
// Used by story revision so mechanical state (round, HP, XP) is untouched.
func (s *Store) ReplaceLastPublicDMText(campaignID, text string) error {
	if campaignID == "" || strings.TrimSpace(text) == "" {
		return errors.New("campaign id and text are required")
	}
	var seq int64
	err := s.db.QueryRow(
		`SELECT seq FROM story_entries
		 WHERE campaign_id = ? AND speaker = 'dm' AND (audience = '' OR audience = 'public')
		 ORDER BY seq DESC LIMIT 1`, campaignID).Scan(&seq)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("no public DM entry to revise")
	}
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE story_entries SET text = ? WHERE campaign_id = ? AND seq = ?`, text, campaignID, seq)
	return err
}

// StoryTail returns the last limit journal entries, oldest first.
func (s *Store) StoryTail(campaignID string, limit int) ([]StoryRow, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(
		`SELECT seq, speaker, audience, text, time_label, created_at FROM story_entries WHERE campaign_id = ? ORDER BY seq DESC LIMIT ?`,
		campaignID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoryRow
	for rows.Next() {
		var e StoryRow
		if err := rows.Scan(&e.Seq, &e.Speaker, &e.Audience, &e.Text, &e.TimeLabel, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// jsonOr substitutes a default document when the stored string is empty, so
// NOT NULL JSON columns never receive "".
func jsonOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
