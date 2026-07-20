// Package store persists narrative memory text in SQLite and generated images
// on disk. Image files live under a dedicated directory (default:
// <repo>/generated-images/) and are kept until the user deletes them ??SQLite
// never holds image bytes. The public URL contract (/generated/<filename>) is
// unchanged.
package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Image is one generated image plus its metadata. Only Filename is required for
// lookup; Prompt/Model are optional metadata on write and not re-read from disk.
type Image struct {
	Filename  string
	Mime      string
	Bytes     []byte
	Prompt    string
	Model     string
	CreatedAt int64
}

// SceneSlot is a deferred scene-image job: prompt/narration captured at DM turn
// time so the player can generate (or regenerate) that beat's illustration later.
type SceneSlot struct {
	ID          string
	StoryID     string
	Scene       string
	Title       string
	Narration   string
	ImagePrompt string
	// PlayersJSON is a JSON array of {name,className,species,appearance}.
	PlayersJSON string
	ImageURL    string
	ImageModel  string
	CreatedAt   int64
}

// Store is a SQLite-backed text store plus a filesystem image cache.
type Store struct {
	db       *sql.DB
	imageDir string
}

// Text-only schema. Legacy `images` BLOB tables are dropped on open.
const schema = `
CREATE TABLE IF NOT EXISTS memory_events (
	story_id   TEXT NOT NULL,
	seq        INTEGER NOT NULL,
	round      INTEGER NOT NULL,
	role       TEXT NOT NULL,
	text       TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (story_id, seq)
);
CREATE TABLE IF NOT EXISTS memory_summary (
	story_id    TEXT PRIMARY KEY,
	summary     TEXT NOT NULL,
	covered_seq INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS scene_slots (
	id           TEXT PRIMARY KEY,
	story_id     TEXT NOT NULL,
	scene        TEXT NOT NULL,
	title        TEXT NOT NULL,
	narration    TEXT NOT NULL,
	image_prompt TEXT NOT NULL,
	players_json TEXT NOT NULL DEFAULT '[]',
	image_url    TEXT NOT NULL DEFAULT '',
	image_model  TEXT NOT NULL DEFAULT '',
	created_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_scene_slots_story ON scene_slots(story_id, created_at);

CREATE TABLE IF NOT EXISTS campaigns (
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
);
CREATE TABLE IF NOT EXISTS characters (
	campaign_id TEXT NOT NULL,
	player_id   TEXT NOT NULL,
	name        TEXT NOT NULL,
	data        TEXT NOT NULL,
	updated_at  INTEGER NOT NULL,
	PRIMARY KEY (campaign_id, player_id)
);
CREATE TABLE IF NOT EXISTS combats (
	campaign_id TEXT PRIMARY KEY,
	data        TEXT NOT NULL,
	updated_at  INTEGER NOT NULL
);
-- Party + combat state captured at combat start, so a wiped party can retry
-- the encounter from its opening state.
CREATE TABLE IF NOT EXISTS combat_snapshots (
	campaign_id TEXT PRIMARY KEY,
	data        TEXT NOT NULL,
	updated_at  INTEGER NOT NULL
);
-- Story pacing arc: three phases with round deadlines and timed rewards.
CREATE TABLE IF NOT EXISTS story_arcs (
	campaign_id TEXT PRIMARY KEY,
	data        TEXT NOT NULL,
	updated_at  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS story_entries (
	campaign_id TEXT NOT NULL,
	seq         INTEGER NOT NULL,
	speaker     TEXT NOT NULL,
	audience    TEXT NOT NULL DEFAULT 'public',
	text        TEXT NOT NULL,
	time_label  TEXT NOT NULL DEFAULT '',
	created_at  INTEGER NOT NULL,
	PRIMARY KEY (campaign_id, seq)
);

-- Metadata for files in generated-images/ (bytes stay on disk).
CREATE TABLE IF NOT EXISTS image_meta (
	filename       TEXT PRIMARY KEY,
	campaign_id    TEXT NOT NULL DEFAULT '',
	scene          TEXT NOT NULL DEFAULT '',
	prompt         TEXT NOT NULL DEFAULT '',
	model          TEXT NOT NULL DEFAULT '',
	source_slot_id TEXT NOT NULL DEFAULT '',
	created_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_image_meta_campaign ON image_meta(campaign_id, created_at DESC);
`

// MemoryEvent is one raw entry in a story's memory log.
type MemoryEvent struct {
	Seq       int64
	Round     int
	Role      string // "player" | "dm" | "system"
	Text      string
	CreatedAt int64
}

// Open opens (creating if needed) the SQLite database at dbPath and prepares
// imageDir for persisted image files. Parent directories must already exist for
// dbPath; imageDir is created when missing.
//
// SQLite allows only one writer at a time. The connection pool is capped at a
// single connection so concurrent writers serialise in-process rather than
// racing into SQLITE_BUSY.
func Open(dbPath, imageDir string) (*Store, error) {
	if strings.TrimSpace(imageDir) == "" {
		return nil, errors.New("image directory is required")
	}
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{"PRAGMA busy_timeout=5000;", "PRAGMA journal_mode=WAL;"} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	// Drop legacy image BLOBs if an older server version created them.
	if _, err := db.Exec(`DROP TABLE IF EXISTS images`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	// CREATE TABLE IF NOT EXISTS never reshapes existing tables; upgrade blob
	// campaigns / story_entries PK from earlier local builds.
	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, imageDir: imageDir}, nil
}

// ImageDir returns the absolute path used for persisted image files.
func (s *Store) ImageDir() string { return s.imageDir }

// Close closes the database. Images on disk are left intact.
func (s *Store) Close() error { return s.db.Close() }

// SaveImage writes image bytes to disk under imageDir and best-effort metadata to SQLite.
func (s *Store) SaveImage(img Image) error {
	if img.Filename == "" {
		return errors.New("image filename is required")
	}
	if err := validateImageFilename(img.Filename); err != nil {
		return err
	}
	path := filepath.Join(s.imageDir, img.Filename)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, img.Bytes, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// Metadata is optional for callers that only care about the file.
	if img.Prompt != "" || img.Model != "" {
		_ = s.UpsertImageMeta(ImageMeta{
			Filename:  img.Filename,
			Prompt:    img.Prompt,
			Model:     img.Model,
			CreatedAt: img.CreatedAt,
		})
	}
	return nil
}

// GetImage reads an image file from disk. The bool is false when no such file exists.
func (s *Store) GetImage(filename string) (Image, bool, error) {
	if err := validateImageFilename(filename); err != nil {
		return Image{}, false, nil
	}
	path := filepath.Join(s.imageDir, filename)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Image{}, false, nil
	}
	if err != nil {
		return Image{}, false, err
	}
	return Image{
		Filename: filename,
		Mime:     mimeForExt(filepath.Ext(filename)),
		Bytes:    data,
	}, true, nil
}

// DeleteImage removes one persisted image file. Missing files are not an error
// (idempotent delete). Filename must pass validateImageFilename.
func (s *Store) DeleteImage(filename string) error {
	if err := validateImageFilename(filename); err != nil {
		return err
	}
	path := filepath.Join(s.imageDir, filename)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// SaveSceneSlot upserts a deferred scene-image placeholder (prompt captured at turn time).
func (s *Store) SaveSceneSlot(slot SceneSlot) error {
	if strings.TrimSpace(slot.ID) == "" || strings.TrimSpace(slot.StoryID) == "" {
		return errors.New("scene slot id and story_id are required")
	}
	if slot.PlayersJSON == "" {
		slot.PlayersJSON = "[]"
	}
	_, err := s.db.Exec(`
INSERT INTO scene_slots (id, story_id, scene, title, narration, image_prompt, players_json, image_url, image_model, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	scene=excluded.scene,
	title=excluded.title,
	narration=excluded.narration,
	image_prompt=excluded.image_prompt,
	players_json=excluded.players_json,
	image_url=excluded.image_url,
	image_model=excluded.image_model
`, slot.ID, slot.StoryID, slot.Scene, slot.Title, slot.Narration, slot.ImagePrompt, slot.PlayersJSON, slot.ImageURL, slot.ImageModel, slot.CreatedAt)
	return err
}

// GetSceneSlot loads one deferred scene-image placeholder.
func (s *Store) GetSceneSlot(id string) (SceneSlot, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return SceneSlot{}, false, nil
	}
	var slot SceneSlot
	err := s.db.QueryRow(`
SELECT id, story_id, scene, title, narration, image_prompt, players_json, image_url, image_model, created_at
FROM scene_slots WHERE id = ?`, id).Scan(
		&slot.ID, &slot.StoryID, &slot.Scene, &slot.Title, &slot.Narration,
		&slot.ImagePrompt, &slot.PlayersJSON, &slot.ImageURL, &slot.ImageModel, &slot.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SceneSlot{}, false, nil
	}
	if err != nil {
		return SceneSlot{}, false, err
	}
	return slot, true, nil
}

// BindSceneSlotImage records the generated image URL/model on a scene slot.
func (s *Store) BindSceneSlotImage(id, imageURL, imageModel string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("scene slot id is required")
	}
	res, err := s.db.Exec(`UPDATE scene_slots SET image_url = ?, image_model = ? WHERE id = ?`, imageURL, imageModel, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("scene slot not found")
	}
	return nil
}

// ListSceneSlots returns recent slots for a story (newest first, capped).
func (s *Store) ListSceneSlots(storyID string, limit int) ([]SceneSlot, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
SELECT id, story_id, scene, title, narration, image_prompt, players_json, image_url, image_model, created_at
FROM scene_slots WHERE story_id = ? ORDER BY created_at DESC LIMIT ?`, storyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SceneSlot
	for rows.Next() {
		var slot SceneSlot
		if err := rows.Scan(
			&slot.ID, &slot.StoryID, &slot.Scene, &slot.Title, &slot.Narration,
			&slot.ImagePrompt, &slot.PlayersJSON, &slot.ImageURL, &slot.ImageModel, &slot.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, slot)
	}
	return out, rows.Err()
}

// ClearImages removes every file in the image directory. Intended for tests /
// explicit maintenance ??the server no longer calls this on start or shutdown.
func (s *Store) ClearImages() error {
	entries, err := os.ReadDir(s.imageDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var first error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// Keep .gitkeep so the repo folder survives a full wipe in tests.
		if e.Name() == ".gitkeep" {
			continue
		}
		if err := os.Remove(filepath.Join(s.imageDir, e.Name())); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func validateImageFilename(name string) error {
	if name != filepath.Base(name) || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return errors.New("invalid image filename")
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		return nil
	default:
		return errors.New("unsupported image extension")
	}
}

func mimeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// AppendMemoryEvents appends raw events to a story's memory log in one
// transaction, assigning monotonically increasing per-story sequence numbers.
func (s *Store) AppendMemoryEvents(storyID string, events []MemoryEvent) error {
	if storyID == "" || len(events) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var next int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM memory_events WHERE story_id = ?`, storyID).Scan(&next); err != nil {
		return err
	}
	for _, e := range events {
		next++
		if _, err := tx.Exec(
			`INSERT INTO memory_events (story_id, seq, round, role, text, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			storyID, next, e.Round, e.Role, e.Text, e.CreatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MemorySummary returns the compacted summary for a story and the highest event
// seq it covers. The bool is false when no summary exists yet.
func (s *Store) MemorySummary(storyID string) (summary string, coveredSeq int64, ok bool, err error) {
	row := s.db.QueryRow(`SELECT summary, covered_seq FROM memory_summary WHERE story_id = ?`, storyID)
	err = row.Scan(&summary, &coveredSeq)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	return summary, coveredSeq, true, nil
}

// MemoryEventsAfter returns up to limit events with seq greater than afterSeq,
// oldest first.
func (s *Store) MemoryEventsAfter(storyID string, afterSeq int64, limit int) ([]MemoryEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT seq, round, role, text, created_at FROM memory_events WHERE story_id = ? AND seq > ? ORDER BY seq ASC LIMIT ?`,
		storyID, afterSeq, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryEvent
	for rows.Next() {
		var e MemoryEvent
		if err := rows.Scan(&e.Seq, &e.Round, &e.Role, &e.Text, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MemoryTail returns the last limit events for a story, oldest first.
func (s *Store) MemoryTail(storyID string, limit int) ([]MemoryEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT seq, round, role, text, created_at FROM memory_events WHERE story_id = ? ORDER BY seq DESC LIMIT ?`,
		storyID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryEvent
	for rows.Next() {
		var e MemoryEvent
		if err := rows.Scan(&e.Seq, &e.Round, &e.Role, &e.Text, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Reverse to oldest-first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// CountMemoryEventsAfter counts events with seq greater than afterSeq.
func (s *Store) CountMemoryEventsAfter(storyID string, afterSeq int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM memory_events WHERE story_id = ? AND seq > ?`, storyID, afterSeq).Scan(&n)
	return n, err
}

// MaxMemorySeq returns the highest event seq for a story (0 when empty).
func (s *Store) MaxMemorySeq(storyID string) (int64, error) {
	var seq int64
	err := s.db.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM memory_events WHERE story_id = ?`, storyID).Scan(&seq)
	return seq, err
}

// SaveMemorySummary upserts a story's compacted summary and the seq it covers.
func (s *Store) SaveMemorySummary(storyID, summary string, coveredSeq, updatedAt int64) error {
	_, err := s.db.Exec(
		`INSERT INTO memory_summary (story_id, summary, covered_seq, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(story_id) DO UPDATE SET summary = excluded.summary, covered_seq = excluded.covered_seq, updated_at = excluded.updated_at`,
		storyID, summary, coveredSeq, updatedAt,
	)
	return err
}
