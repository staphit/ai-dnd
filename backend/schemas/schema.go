// Package schema embeds the DM structured-output JSON schema so the binary is
// self-contained. Codex needs the schema as a file path, so WriteTempFile
// materialises it on disk at startup.
package schema

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed dm-turn.schema.json
var Raw []byte

//go:embed image-prompt.schema.json
var ImagePromptRaw []byte

//go:embed combat-tactics.schema.json
var CombatTacticsRaw []byte

// WriteTempFile writes the embedded DM-turn schema to a stable temp path and
// returns it.
func WriteTempFile() (string, error) {
	return writeTempFile("dnd-duet-dm-turn.schema.json", Raw)
}

// WriteImagePromptTempFile writes the embedded image-prompt-translation
// schema to a stable temp path and returns it.
func WriteImagePromptTempFile() (string, error) {
	return writeTempFile("dnd-duet-image-prompt.schema.json", ImagePromptRaw)
}

// WriteCombatTacticsTempFile writes the embedded enemy combat-tactics schema
// to a stable temp path and returns it.
func WriteCombatTacticsTempFile() (string, error) {
	return writeTempFile("dnd-duet-combat-tactics.schema.json", CombatTacticsRaw)
}

func writeTempFile(name string, raw []byte) (string, error) {
	dst := filepath.Join(os.TempDir(), name)
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}
