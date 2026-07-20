package tts

import (
	"regexp"
	"strings"
)

// PrepareText rewrites DM narration into something GPT-SoVITS reads naturally:
// game/meta suffixes are dropped, dice and rules shorthand become spoken
// Chinese, and layout symbols the TTS would stumble over are turned into
// punctuation pauses.
func PrepareText(text string) string {
	// The DM response appends check/choice blocks after the narration; they are
	// UI hints, not story, so they are not read aloud.
	for _, marker := range []string{"\n\n檢定：", "\n\n可考慮："} {
		if i := strings.Index(text, marker); i >= 0 {
			text = text[:i]
		}
	}

	// Rules shorthand into spoken Chinese, before symbol stripping.
	text = dicePattern.ReplaceAllStringFunc(text, func(m string) string {
		parts := dicePattern.FindStringSubmatch(m)
		if parts[1] == "" || parts[1] == "1" {
			return parts[2] + "面骰"
		}
		return parts[1] + "顆" + parts[2] + "面骰"
	})
	text = dcPattern.ReplaceAllString(text, "難度$1")
	text = plusPattern.ReplaceAllString(text, "加$1")
	text = statReplacer.Replace(text)

	// Layout and markdown symbols: slashes become enumeration pauses, brackets
	// become punctuation, decoration disappears.
	text = symbolReplacer.Replace(text)

	// Newlines end sentences instead of being swallowed silently.
	text = newlinePattern.ReplaceAllString(text, "。")

	// Collapse the punctuation runs the rewrites above can leave behind.
	text = punctRunPattern.ReplaceAllStringFunc(text, func(run string) string {
		r := []rune(run)
		last := string(r[len(r)-1])
		if strings.ContainsRune(run, '。') {
			return "。"
		}
		return last
	})
	text = strings.Trim(text, "，、：")
	return strings.TrimSpace(text)
}

var (
	dicePattern    = regexp.MustCompile(`(\d*)[dD](\d+)`)
	dcPattern      = regexp.MustCompile(`(?i)DC\s*(\d+)`)
	plusPattern    = regexp.MustCompile(`[+＋](\d+)`)
	newlinePattern = regexp.MustCompile(`\s*\n+\s*`)
	// Runs of mixed pause punctuation left over after bracket replacement.
	punctRunPattern = regexp.MustCompile(`[，、。：]{2,}`)

	statReplacer = strings.NewReplacer(
		"HP", "生命值", "hp", "生命值",
		"AC", "護甲等級",
		"XP", "經驗值",
	)
	symbolReplacer = strings.NewReplacer(
		"／", "、", "/", "、",
		"【", "", "】", "：",
		"（", "，", "）", "，",
		"(", "，", ")", "，",
		"*", "", "#", "", "`", "", "_", "", "~", "",
		"—", "，", "─", "，",
	)
)
