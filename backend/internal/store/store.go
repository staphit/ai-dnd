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
`

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
