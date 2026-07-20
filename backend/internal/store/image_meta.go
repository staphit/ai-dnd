package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ImageMeta is SQLite metadata for a file under imageDir (bytes stay on disk).
type ImageMeta struct {
	Filename     string
	CampaignID   string
	Scene        string
	Prompt       string
	Model        string
	SourceSlotID string
	CreatedAt    int64
}

// UpsertImageMeta records metadata for a generated image file.
func (s *Store) UpsertImageMeta(m ImageMeta) error {
	if strings.TrimSpace(m.Filename) == "" {
		return errors.New("image filename is required")
	}
	if m.CreatedAt == 0 {
		m.CreatedAt = time.Now().UnixMilli()
	}
	_, err := s.db.Exec(`
INSERT INTO image_meta (filename, campaign_id, scene, prompt, model, source_slot_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(filename) DO UPDATE SET
	campaign_id=excluded.campaign_id,
	scene=excluded.scene,
	prompt=excluded.prompt,
	model=excluded.model,
	source_slot_id=excluded.source_slot_id,
	created_at=excluded.created_at
`, m.Filename, m.CampaignID, m.Scene, m.Prompt, m.Model, m.SourceSlotID, m.CreatedAt)
	return err
}

// GetImageMeta loads metadata for one generated file.
func (s *Store) GetImageMeta(filename string) (ImageMeta, bool, error) {
	var m ImageMeta
	err := s.db.QueryRow(`
SELECT filename, campaign_id, scene, prompt, model, source_slot_id, created_at
FROM image_meta WHERE filename = ?`, filename).Scan(
		&m.Filename, &m.CampaignID, &m.Scene, &m.Prompt, &m.Model, &m.SourceSlotID, &m.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ImageMeta{}, false, nil
	}
	if err != nil {
		return ImageMeta{}, false, err
	}
	return m, true, nil
}

// ListImageMeta returns recent image metadata (optional campaign filter).
func (s *Store) ListImageMeta(campaignID string, limit int) ([]ImageMeta, error) {
	if limit <= 0 {
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if strings.TrimSpace(campaignID) == "" {
		rows, err = s.db.Query(`
SELECT filename, campaign_id, scene, prompt, model, source_slot_id, created_at
FROM image_meta ORDER BY created_at DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`
SELECT filename, campaign_id, scene, prompt, model, source_slot_id, created_at
FROM image_meta WHERE campaign_id = ? ORDER BY created_at DESC LIMIT ?`, campaignID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ImageMeta
	for rows.Next() {
		var m ImageMeta
		if err := rows.Scan(&m.Filename, &m.CampaignID, &m.Scene, &m.Prompt, &m.Model, &m.SourceSlotID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
