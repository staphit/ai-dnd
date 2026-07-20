// Package provider defines the AI Dungeon Master backend interface. The Codex
// CLI client implements it; the HTTP, DM and image layers depend only on this
// interface, which also lets tests inject a fake without spawning the CLI.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNeedsConsent is returned by RunStructured when there is no live codex
// connection bound to the requested story. The HTTP layer maps it to 409 so the
// frontend can ask the player to (re)connect — connections are never
// established implicitly.
var ErrNeedsConsent = errors.New("尚未連線 Codex，或連線綁定的是其他故事；請先按「連線」。")

// Status reports whether a provider's CLI is usable.
type Status struct {
	Configured bool
	Provider   string
	Model      string
	Message    string
}

// ConnState reports the persistent connection's binding: whether a live
// connection exists and which story id it is bound to.
type ConnState struct {
	Alive   bool
	StoryID string
}

// ModelOption is one selectable model.
type ModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// StructuredOpts configures a structured (schema-constrained) run.
type StructuredOpts struct {
	CWD        string
	SchemaPath string
	Model      string
	// Effort is the reasoning-effort id (empty keeps the provider default).
	Effort  string
	Timeout time.Duration
	// StoryID is the sanitized per-story id this turn belongs to. The persistent
	// provider requires a live connection bound to this story (else
	// ErrNeedsConsent); the stateless exec provider ignores it.
	StoryID string
}

// ImageOpts configures an image-generation run.
type ImageOpts struct {
	CWD     string
	Model   string
	Timeout time.Duration
}

// API is the DM backend a provider must implement.
type API interface {
	// Status reports connectivity and the active model label.
	Status(ctx context.Context) Status
	// Connect establishes (or rebinds) the persistent connection to the given
	// story. It is the only path that (re)creates a connection and is called only
	// when the player has explicitly consented. The stateless exec provider
	// treats this as a no-op.
	Connect(ctx context.Context, storyID string) error
	// ConnectionState reports the current connection binding.
	ConnectionState() ConnState
	// NormalizeModel validates a requested model id, returning the configured
	// default for an empty request and an error for an unknown id.
	NormalizeModel(value string) (string, error)
	// NormalizeEffort validates a requested reasoning-effort id, returning the
	// configured default for an empty request and an error for an unknown id.
	NormalizeEffort(value string) (string, error)
	// EffortOptions lists the selectable reasoning-effort levels.
	EffortOptions() []ModelOption
	// Model is the human-readable active model label.
	Model() string
	// ModelOptions lists the selectable models for this provider.
	ModelOptions() []ModelOption
	// ImageModel is the image-generation model label, or "" when this provider
	// cannot generate images.
	ImageModel() string
	// RunStructured runs a schema-constrained generation and returns the JSON.
	RunStructured(ctx context.Context, prompt string, opts StructuredOpts) (json.RawMessage, error)
	// RunImageGeneration generates one image and returns its file path, or an
	// error when the provider does not support image generation.
	RunImageGeneration(ctx context.Context, prompt string, opts ImageOpts) (string, error)
}
