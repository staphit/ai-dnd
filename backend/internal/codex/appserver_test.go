package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dndduet/internal/provider"
)

// TestRunTurnRequiresConsent verifies a turn on an unbound connection returns
// ErrNeedsConsent instead of implicitly connecting (no process is spawned).
func TestRunTurnRequiresConsent(t *testing.T) {
	a := NewAppServer("codex", "/tmp")
	_, err := a.RunTurn(context.Background(), "story1", "prompt", "", "", "{}", time.Second)
	if !errors.Is(err, provider.ErrNeedsConsent) {
		t.Fatalf("want ErrNeedsConsent, got %v", err)
	}
	if cs := a.ConnectionState(); cs.Alive || cs.StoryID != "" {
		t.Errorf("fresh AppServer should report no connection, got %+v", cs)
	}
}

func TestAppServerErrorMessage(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"message":"gpt-5.6-sol requires a newer version","codexErrorInfo":"other"}`, "gpt-5.6-sol requires a newer version"},
		{`{"codexErrorInfo":"other"}`, `{"codexErrorInfo":"other"}`}, // no message field -> raw
		{``, "Codex app-server 回報錯誤"},
	}
	for _, c := range cases {
		if got := appServerErrorMessage(json.RawMessage(c.raw)); got != c.want {
			t.Errorf("appServerErrorMessage(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestAppServerErrorMessageNestedJSON(t *testing.T) {
	// The app-server wraps the upstream API error as a JSON string in message.
	raw := `{"message":"{\"type\":\"error\",\"status\":400,\"error\":{\"message\":\"bad model\"}}"}`
	got := appServerErrorMessage(json.RawMessage(raw))
	if !strings.Contains(got, "bad model") {
		t.Errorf("expected nested message surfaced, got %q", got)
	}
}

// TestAppServerClientImplementsAPI is a compile-time assertion made explicit.
func TestAppServerClientImplementsAPI(t *testing.T) {
	c := NewAppServerClient("/tmp")
	if c.ImageModel() != ImageModel {
		t.Errorf("AppServerClient should delegate ImageModel to the exec client")
	}
	if got, _ := c.NormalizeModel("gpt-5.6-terra"); got != "gpt-5.6-terra" {
		t.Errorf("AppServerClient should delegate NormalizeModel")
	}
	if c.story == nil || c.image == nil {
		t.Fatal("AppServerClient must own both story and image AppServers")
	}
	if c.story.label != "story" || c.image.label != "image" {
		t.Errorf("labels story=%q image=%q", c.story.label, c.image.label)
	}
}

func TestRunImageTurnRequiresConsent(t *testing.T) {
	a := NewAppServerLabeled("codex", "/tmp", "image")
	_, err := a.RunImageTurn(context.Background(), "story1", "draw a dragon", time.Second)
	if !errors.Is(err, provider.ErrNeedsConsent) {
		t.Fatalf("want ErrNeedsConsent, got %v", err)
	}
}

func TestPickImagePrefersNewestAfterCutoff(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.png")
	newPath := filepath.Join(dir, "new.png")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure distinct mtimes.
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().Add(-time.Minute)
	got, ok := pickImage([]string{oldPath, newPath}, cutoff)
	if !ok || got != newPath {
		t.Fatalf("pickImage = %q ok=%v, want %q", got, ok, newPath)
	}
	// Zero cutoff picks newest overall.
	got, ok = pickImage([]string{oldPath, newPath}, time.Time{})
	if !ok || got != newPath {
		t.Fatalf("pickImage zero cutoff = %q ok=%v", got, ok)
	}
}

func TestAppServerResetClearsFailedConnectionState(t *testing.T) {
	a := NewAppServer("codex", "/tmp")
	a.started = true
	a.alive = true
	a.boundStory = "story1"
	a.threadID = "thread1"
	a.nextID = 42
	a.pending[42] = make(chan rpcMessage, 1)

	if err := a.Reset(); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if cs := a.ConnectionState(); cs.Alive || cs.StoryID != "" {
		t.Fatalf("Reset() left a live binding: %+v", cs)
	}
	if a.started || a.alive || a.boundStory != "" || a.threadID != "" || a.nextID != 0 || len(a.pending) != 0 {
		t.Fatalf("Reset() did not clear state: %#v", a)
	}
}
