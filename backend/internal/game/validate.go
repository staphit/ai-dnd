package game

import (
	"encoding/json"
	"fmt"
	"strings"

	"dndduet/internal/rules"
)

// Intent is an optional structured declaration accompanying an action so the
// server can validate it precisely (the spell UI sends these).
type Intent struct {
	Type     string `json:"type"` // "spell"
	SpellID  string `json:"spellId"`
	TargetID string `json:"targetId"`
	AsRitual bool   `json:"asRitual"`
}

// ActionIssue mirrors the AI actionIssues shape so the frontend renders both
// through the same path.
type ActionIssue struct {
	PlayerID string `json:"playerId"`
	Message  string `json:"message"`
}

// ActionIssuesError carries mechanical validation failures; the handler
// returns them as 422 without any AI call.
type ActionIssuesError struct {
	Issues []ActionIssue
}

func (e *ActionIssuesError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, i := range e.Issues {
		parts = append(parts, i.Message)
	}
	return strings.Join(parts, "；")
}

// castingVerbs marks a free-text declaration as an actual cast attempt; a
// spell name alone can be narrative flavor.
var castingVerbs = []string{"施放", "施展", "吟唱", "詠唱"}

// validateActions runs mechanical legality checks before the AI sees the
// round. Structured intents are validated precisely; free text conservatively
// (exact spell-name match + casting verb + hard resource failure only), so a
// false rejection is much rarer than a miss — the AI narrative check remains
// the softer backstop.
func (s *Service) validateActions(st *gameState, actions map[string]string, intents map[string]Intent) []ActionIssue {
	var issues []ActionIssue
	for _, player := range st.players {
		text := actions[player.ID]

		if intent, ok := intents[player.ID]; ok && intent.Type == "spell" {
			if issue := validateSpellIntent(player, intent); issue != "" {
				issues = append(issues, ActionIssue{PlayerID: player.ID, Message: issue})
				continue
			}
		}

		if player.Spellcasting == nil || text == "" {
			continue
		}
		hasVerb := false
		for _, v := range castingVerbs {
			if strings.Contains(text, v) {
				hasVerb = true
				break
			}
		}
		if !hasVerb {
			continue
		}
		for _, spell := range player.Spellcasting.Spells {
			if spell.Level == 0 || spell.Name == "" || !strings.Contains(text, spell.Name) {
				continue
			}
			if !spell.Prepared && !spell.AlwaysPrepared {
				issues = append(issues, ActionIssue{PlayerID: player.ID, Message: fmt.Sprintf("「%s」目前未準備，無法施放。請先於角色成長頁調整準備法術，或改選其他行動。", spell.Name)})
				break
			}
			if _, ok := rules.SpendSpellSlot(player, spell, false); !ok {
				issues = append(issues, ActionIssue{PlayerID: player.ID, Message: fmt.Sprintf("施放「%s」需要 %d 環以上的法術位或對應的免費施法資源，目前已耗盡。請改選其他行動或先休息。", spell.Name, spell.Level)})
				break
			}
			break // first matched spell decides; it validated fine
		}
	}
	return issues
}

func validateSpellIntent(player rules.Character, intent Intent) string {
	if player.Spellcasting == nil {
		return player.Name + " 沒有施法能力。"
	}
	for _, spell := range player.Spellcasting.Spells {
		if spell.ID != intent.SpellID {
			continue
		}
		if spell.Level > 0 && !spell.Prepared && !spell.AlwaysPrepared {
			return fmt.Sprintf("「%s」目前未準備，無法施放。", spell.Name)
		}
		if intent.AsRitual && !spell.Ritual {
			return fmt.Sprintf("「%s」不是儀式法術，無法以儀式施放。", spell.Name)
		}
		if _, ok := rules.SpendSpellSlot(player, spell, intent.AsRitual); !ok {
			return fmt.Sprintf("施放「%s」需要 %d 環以上的法術位或對應的免費施法資源，目前已耗盡。", spell.Name, spell.Level)
		}
		return ""
	}
	return "找不到這個法術。"
}

// jsonMarshal/jsonUnmarshal are tiny indirection helpers so dmturn.go reads
// cleanly.
func jsonMarshal(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func jsonUnmarshal(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
