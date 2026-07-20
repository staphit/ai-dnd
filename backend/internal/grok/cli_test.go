package grok

import (
	"encoding/json"
	"testing"
)

func TestExtractCLIStructuredJSONFromEnvelope(t *testing.T) {
	stdout := `{
  "text": "{\"hello\":\"world\"}",
  "structuredOutput": {"hello": "world"},
  "stopReason": "EndTurn"
}`
	raw := extractCLIStructuredJSON(stdout)
	if raw == nil {
		t.Fatal("nil")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m["hello"] != "world" {
		t.Fatalf("got %s err=%v", string(raw), err)
	}
}

func TestExtractCLIStructuredJSONNarration(t *testing.T) {
	stdout := `{"narration":"場景","scene":"s","choices":[{"text":"a","playerId":"player1"}]}`
	raw := extractCLIStructuredJSON(stdout)
	if raw == nil {
		t.Fatal("nil")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m["narration"] != "場景" {
		t.Fatalf("got %s", string(raw))
	}
}

func TestResolveCLICommandNonEmpty(t *testing.T) {
	if ResolveCLICommand() == "" {
		t.Fatal("empty command")
	}
}
