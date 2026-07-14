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

// WriteTempFile writes the embedded schema to a stable temp path and returns it.
func WriteTempFile() (string, error) {
	dst := filepath.Join(os.TempDir(), "dnd-duet-dm-turn.schema.json")
	if err := os.WriteFile(dst, Raw, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}
