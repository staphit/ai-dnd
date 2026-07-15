package codex

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

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
}

func TestAppServerResetClearsFailedConnectionState(t *testing.T) {
	a := NewAppServer("codex", "/tmp")
	a.started = true
	a.initErr = errors.New("failed")
	a.nextID = 42
	a.incoming = make(chan rpcMessage, 1)

	if err := a.Reset(); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if a.started || a.initErr != nil || a.nextID != 0 || a.incoming != nil {
		t.Fatalf("Reset() did not clear state: %#v", a)
	}
}
