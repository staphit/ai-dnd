package grok

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dndduet/internal/provider"
)

func TestNewClientFromEnvRequiresKey(t *testing.T) {
	t.Setenv("XAI_API_KEY", "")
	t.Setenv("GROK_API_KEY", "")
	if NewClientFromEnv() != nil {
		t.Fatal("expected nil without key")
	}
	t.Setenv("XAI_API_KEY", "test-key")
	c := NewClientFromEnv()
	if c == nil {
		t.Fatal("expected client")
	}
	if !c.Status(context.Background()).Configured {
		t.Fatal("should be configured")
	}
}

func TestConnectAndConsent(t *testing.T) {
	c := &Client{APIKey: "k", ChatModel: "grok-4", BaseURL: "http://example", HTTPClient: http.DefaultClient}
	if _, err := c.RunStructured(context.Background(), "hi", provider.StructuredOpts{StoryID: "campaign-1", SchemaPath: "x"}); err != provider.ErrNeedsConsent {
		t.Fatalf("want ErrNeedsConsent, got %v", err)
	}
	if err := c.Connect(context.Background(), "campaign-1"); err != nil {
		t.Fatal(err)
	}
	cs := c.ConnectionState()
	if !cs.Alive || cs.StoryID != "campaign-1" {
		t.Fatalf("conn: %+v", cs)
	}
}

func TestRunStructuredJSONSchema(t *testing.T) {
	schemaPath := filepath.Join(t.TempDir(), "s.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("missing bearer")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["response_format"] == nil {
			t.Error("expected response_format")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"ok":true}`}},
			},
		})
	}))
	defer srv.Close()

	c := &Client{
		APIKey:     "k",
		BaseURL:    srv.URL,
		ChatModel:  "grok-4",
		HTTPClient: srv.Client(),
	}
	_ = c.Connect(context.Background(), "s1")
	raw, err := c.RunStructured(context.Background(), "test", provider.StructuredOpts{
		SchemaPath: schemaPath,
		StoryID:    "s1",
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out["ok"] != true {
		t.Fatalf("out=%s err=%v", string(raw), err)
	}
}

func TestGenerateImageBytesB64(t *testing.T) {
	// 1x1 PNG
	pngB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": pngB64}},
		})
	}))
	defer srv.Close()
	c := &Client{APIKey: "k", BaseURL: srv.URL, ConfiguredImageModel: "grok-imagine-image-quality", HTTPClient: srv.Client()}
	data, ext, err := c.GenerateImageBytes(context.Background(), "a dragon", "")
	if err != nil {
		t.Fatal(err)
	}
	if ext != ".png" || len(data) < 20 {
		t.Fatalf("ext=%s len=%d", ext, len(data))
	}
}

func TestNormalizeModel(t *testing.T) {
	c := &Client{ChatModel: "grok-4.5"}
	if got, err := c.NormalizeModel(""); err != nil || got != "grok-4.5" {
		t.Fatalf("empty got %q %v", got, err)
	}
	if got, err := c.NormalizeModel("grok-4.5"); err != nil || got != "grok-4.5" {
		t.Fatalf("4.5 got %q %v", got, err)
	}
	// Legacy / other Grok ids are coerced to 4.5 so old campaign settings keep working.
	if got, err := c.NormalizeModel("grok-3-mini"); err != nil || got != "grok-4.5" {
		t.Fatalf("legacy got %q %v", got, err)
	}
	if _, err := c.NormalizeModel("gpt-4"); err == nil {
		t.Fatal("expected reject non-grok")
	}
}
