package rules

// Ported 1:1 from frontend/src/rules/party.ts: dynamic-party turn readiness
// and the ordered action payload sent to the DM.

import "strings"

// Action mirrors the payload objects built by party.ts createActionPayload:
// { playerId, text }.
type Action struct {
	PlayerID string `json:"playerId"`
	Text     string `json:"text"`
}

// AreAllActionsReady mirrors party.ts areAllActionsReady: true only when the
// party is non-empty and every member has a pending action that is non-empty
// after a JS-style trim.
func AreAllActionsReady(players []Character, pending map[string]string) bool {
	if len(players) == 0 {
		return false
	}
	for _, player := range players {
		// Boolean(pending[player.id]?.trim())
		if strings.TrimFunc(pending[player.ID], isJSWhitespace) == "" {
			return false
		}
	}
	return true
}

// CreateActionPayload mirrors party.ts createActionPayload: one entry per
// player in party order, with the pending text trimmed. Missing entries become
// the empty string, matching the TS optional-chained trim with its
// empty-string fallback.
func CreateActionPayload(players []Character, pending map[string]string) []Action {
	payload := make([]Action, len(players))
	for i, player := range players {
		payload[i] = Action{PlayerID: player.ID, Text: strings.TrimFunc(pending[player.ID], isJSWhitespace)}
	}
	return payload
}
