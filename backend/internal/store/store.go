// Package store persists generated images in SQLite. The original Node backend
// wrote image files under campaign-data/images/; here the bytes and metadata
// live in a SQLite database so the server has a real database of record while
// the public URL contract (/generated/<filename>) is unchanged.
package store

import (
	"database/sql"
	"errors"

	_ "modernc.org/sqlite"
)

// Image is one generated image plus its metadata.
type Image struct {
	Filename  string
	Mime      string
	Bytes     []byte
	Prompt    string
	Model     string
	CreatedAt int64
}

// Store is a SQLite-backed image repository.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS images (
	filename   TEXT PRIMARY KEY,
	mime       TEXT NOT NULL,
	bytes      BLOB NOT NULL,
	prompt     TEXT NOT NULL,
	model      TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
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
`

// MemoryEvent is one raw entry in a story's memory log.
type MemoryEvent struct {
	Seq       int64
	Round     int
	Role      string // "player" | "dm" | "system"
	Text      string
	CreatedAt int64
}

// Open opens (creating if needed) the SQLite database at path. The parent
// directory must already exist.
//
// SQLite allows only one writer at a time. The connection pool is capped at a
// single connection so concurrent SaveImage/GetImage calls serialise in-process
// rather than racing into SQLITE_BUSY, and busy_timeout covers any other writer
// (e.g. a second server instance).
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
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
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// SaveImage inserts or replaces an image row.
func (s *Store) SaveImage(img Image) error {
	if img.Filename == "" {
		return errors.New("image filename is required")
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO images (filename, mime, bytes, prompt, model, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		img.Filename, img.Mime, img.Bytes, img.Prompt, img.Model, img.CreatedAt,
	)
	return err
}

// GetImage returns the image with the given filename. The bool is false when no
// such row exists.
func (s *Store) GetImage(filename string) (Image, bool, error) {
	row := s.db.QueryRow(
		`SELECT filename, mime, bytes, prompt, model, created_at FROM images WHERE filename = ?`,
		filename,
	)
	var img Image
	err := row.Scan(&img.Filename, &img.Mime, &img.Bytes, &img.Prompt, &img.Model, &img.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, false, nil
	}
	if err != nil {
		return Image{}, false, err
	}
	return img, true, nil
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
