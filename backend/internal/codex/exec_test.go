package codex

import (
	"strings"
	"testing"
)

func TestNormalizeModelAllowsDocumentedChoices(t *testing.T) {
	c := &Client{ConfiguredModel: ""}

	if got, err := c.NormalizeModel(""); err != nil || got != "" {
		t.Errorf("NormalizeModel(\"\") = %q, %v; want \"\", nil", got, err)
	}
	if got, err := c.NormalizeModel("gpt-5.6-terra"); err != nil || got != "gpt-5.6-terra" {
		t.Errorf("NormalizeModel(terra) = %q, %v; want gpt-5.6-terra, nil", got, err)
	}

	found := false
	for _, m := range ModelOptions {
		if m.ID == "gpt-5.6-sol" {
			found = true
		}
	}
	if !found {
		t.Error("ModelOptions must include gpt-5.6-sol")
	}
}

func TestNormalizeModelPreservesConfiguredDefault(t *testing.T) {
	c := &Client{ConfiguredModel: "gpt-5.6-luna"}
	if got, err := c.NormalizeModel(""); err != nil || got != "gpt-5.6-luna" {
		t.Errorf("NormalizeModel(\"\") = %q, %v; want gpt-5.6-luna, nil", got, err)
	}
}

func TestNormalizeModelRejectsArbitraryArguments(t *testing.T) {
	c := &Client{ConfiguredModel: ""}
	_, err := c.NormalizeModel("--dangerously-bypass-approvals-and-sandbox")
	if err == nil || !strings.Contains(err.Error(), "不支援") {
		t.Errorf("expected 不支援 error, got %v", err)
	}
}

func TestModelDisplayFallsBackToDefault(t *testing.T) {
	if got := (&Client{ConfiguredModel: ""}).Model(); got != "Codex 預設模型" {
		t.Errorf("Model() = %q, want 預設模型 fallback", got)
	}
	if got := (&Client{ConfiguredModel: "gpt-5.6"}).Model(); got != "gpt-5.6" {
		t.Errorf("Model() = %q, want gpt-5.6", got)
	}
}
