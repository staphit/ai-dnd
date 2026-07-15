// Package provider defines the AI Dungeon Master backend interface. The Codex
// CLI client implements it; the HTTP, DM and image layers depend only on this
// interface, which also lets tests inject a fake without spawning the CLI.
package provider

import (
	"context"
	"encoding/json"
	"time"
)

// Status reports whether a provider's CLI is usable.
type Status struct {
	Configured bool
	Provider   string
	Model      string
	Message    string
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
