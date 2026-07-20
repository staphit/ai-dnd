package dm

import (
	"strings"
	"testing"
)

// The English language override must appear only when the campaign asks for
// it; the default prompt stays pure Traditional Chinese.
func TestBuildDMRequestV2LanguageOverride(t *testing.T) {
	base := TurnInputV2{Title: "測試戰役", Scene: "測試場景", Round: 1}

	if body := BuildDMRequestV2(base); strings.Contains(body, "LANGUAGE OVERRIDE") {
		t.Fatalf("default prompt must not carry the English override:\n%s", body)
	}

	base.Language = "en"
	body := BuildDMRequestV2(base)
	if !strings.Contains(body, "LANGUAGE OVERRIDE: this campaign is played in English.") {
		t.Fatalf("language=en prompt is missing the English override:\n%s", body)
	}
	if !strings.Contains(body, "語言指令（優先於守則中的繁體中文要求）") {
		t.Fatalf("language=en prompt is missing the zh-side directive:\n%s", body)
	}
}
