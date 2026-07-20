package dm

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strings"
)

// Check is a required ability check the DM is asking a player to roll.
type Check struct {
	Character string `json:"character"`
	PlayerID  string `json:"playerId,omitempty"`
	Ability   string `json:"ability"`
	Skill     string `json:"skill"`
	DC        int    `json:"dc"`
	Reason    string `json:"reason"`
}

// Effect is a damage/healing/condition change to sync onto a character sheet.
type Effect struct {
	TargetID  string `json:"targetId"`
	Kind      string `json:"kind"`
	Amount    int    `json:"amount"`
	Condition string `json:"condition"`
	Reason    string `json:"reason"`
}

// PrivateMessage is text visible only to one player.
type PrivateMessage struct {
	PlayerID string `json:"playerId"`
	Text     string `json:"text"`
}

// Enemy is a combatant the DM introduces when combat begins.
type Enemy struct {
	Name            string `json:"name"`
	AC              int    `json:"ac"`
	HP              int    `json:"hp"`
	InitiativeBonus int    `json:"initiativeBonus"`
	AttackBonus     int    `json:"attackBonus"`
	Damage          string `json:"damage"`
	DamageType      string `json:"damageType"`
}

// Combat signals whether combat starts this turn and describes the enemies.
type Combat struct {
	Starts    bool    `json:"starts"`
	FirstTurn string  `json:"firstTurn"`
	Enemies   []Enemy `json:"enemies"`
}

// ActionIssue rejects a player's declared action with a reason.
type ActionIssue struct {
	PlayerID string `json:"playerId"`
	Message  string `json:"message"`
}

// Choice is one suggested next action, tied to the player it suits so the
// UI shows a barbarian only barbarian-appropriate suggestions.
type Choice struct {
	Text     string `json:"text"`
	PlayerID string `json:"playerId"`
}

// ExperienceAward grants XP to a player.
type ExperienceAward struct {
	PlayerID string `json:"playerId"`
	Amount   int    `json:"amount"`
	Reason   string `json:"reason"`
}

// Turn is the validated DM output.
type Turn struct {
	Narration        string
	Scene            string
	ImagePrompt      string
	Objective        string
	ObjectiveContext string
	Stakes           string
	RequiresCheck    bool
	Check            *Check
	Choices          []Choice
	Effects          []Effect
	PrivateMessages  []PrivateMessage
	Combat           Combat
	ActionIssues     []ActionIssue
	ExperienceAwards []ExperienceAward
}

var playerIDPattern = regexp.MustCompile(`^player[1-4]$`)

func nonEmptyString(v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	// JS !value.trim() — reject when only whitespace. strings.TrimSpace uses
	// unicode.IsSpace, so it also strips full-width spaces (U+3000) and other
	// Unicode whitespace that JS .trim() removes, which matters for the
	// Traditional Chinese DM output.
	if strings.TrimSpace(s) == "" {
		return "", false
	}
	return s, true
}

func floorClampInt(n float64, lo, hi int) int {
	f := math.Floor(n)
	if math.IsNaN(f) {
		f = 0
	}
	r := int(f)
	if r < lo {
		return lo
	}
	if r > hi {
		return hi
	}
	return r
}

// validateDMTurn validates and coerces raw Codex output, mirroring
// validateDmTurn in dm-agent.mjs.
func validateDMTurn(raw json.RawMessage) (*Turn, error) {
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil || v == nil {
		return nil, errors.New("Codex DM 輸出格式錯誤")
	}

	narration, ok := nonEmptyString(v["narration"])
	if !ok {
		return nil, errors.New("Codex DM 沒有產生場景敘事")
	}
	scene, ok := nonEmptyString(v["scene"])
	if !ok {
		return nil, errors.New("Codex DM 沒有回傳場景名稱")
	}
	objective, ok := nonEmptyString(v["objective"])
	if !ok {
		return nil, errors.New("Codex DM 沒有回傳當前目標")
	}
	objectiveContext, ok := nonEmptyString(v["objectiveContext"])
	if !ok {
		return nil, errors.New("Codex DM 沒有回傳任務背景")
	}
	stakes, ok := nonEmptyString(v["stakes"])
	if !ok {
		return nil, errors.New("Codex DM 沒有回傳任務風險")
	}
	requiresCheck, ok := v["requiresCheck"].(bool)
	if !ok {
		return nil, errors.New("Codex DM 沒有回傳檢定狀態")
	}
	choicesRaw, ok := asSlice(v["choices"])
	if !ok || len(choicesRaw) < 1 {
		return nil, errors.New("Codex DM 沒有提供下一步選項")
	}
	if requiresCheck && !truthy(v["check"]) {
		return nil, errors.New("Codex DM 要求檢定但沒有提供檢定內容")
	}

	// imagePrompt is the DM-produced English SD prompt for the scene. Not
	// hard-required here so a model that omits it still yields a playable turn;
	// image generation falls back when it's empty.
	imagePrompt, _ := nonEmptyString(v["imagePrompt"])

	turn := &Turn{
		Narration:        narration,
		Scene:            scene,
		ImagePrompt:      imagePrompt,
		Objective:        objective,
		ObjectiveContext: objectiveContext,
		Stakes:           stakes,
		RequiresCheck:    requiresCheck,
	}

	if requiresCheck {
		if m := asMap(v["check"]); m != nil {
			playerID := strOf(m["playerId"])
			if !playerIDPattern.MatchString(playerID) {
				playerID = ""
			}
			turn.Check = &Check{
				Character: strOf(m["character"]),
				PlayerID:  playerID,
				Ability:   strOf(m["ability"]),
				Skill:     strOf(m["skill"]),
				DC:        int(toNum(m["dc"])),
				Reason:    strOf(m["reason"]),
			}
		}
	}

	// Accept both the structured {text, playerId} form and a bare string
	// (legacy / when the model omits the tag), so parsing never fails on a
	// plain suggestion; untagged choices apply to the whole party.
	turn.Choices = make([]Choice, 0, len(choicesRaw))
	for _, c := range choicesRaw {
		if m := asMap(c); m != nil {
			text := strOf(m["text"])
			if text == "" {
				continue
			}
			turn.Choices = append(turn.Choices, Choice{Text: text, PlayerID: strOf(m["playerId"])})
			continue
		}
		if s := strOf(c); s != "" {
			turn.Choices = append(turn.Choices, Choice{Text: s})
		}
	}

	turn.Effects = []Effect{}
	if arr, ok := asSlice(v["effects"]); ok {
		for _, e := range arr {
			targetID, _ := get(e, "targetId").(string)
			kind, _ := get(e, "kind").(string)
			reason, reasonOK := get(e, "reason").(string)
			if !playerIDPattern.MatchString(targetID) || !(kind == "damage" || kind == "healing" || kind == "condition") || !reasonOK {
				continue
			}
			turn.Effects = append(turn.Effects, Effect{
				TargetID:  targetID,
				Kind:      kind,
				Amount:    floorClampInt(numOr(get(e, "amount"), 0), 0, 500),
				Condition: jsSlice(strOr(get(e, "condition"), ""), 40),
				Reason:    jsSlice(reason, 160),
			})
		}
	}

	turn.PrivateMessages = []PrivateMessage{}
	if arr, ok := asSlice(v["privateMessages"]); ok {
		for _, p := range arr {
			playerID, _ := get(p, "playerId").(string)
			text, textOK := get(p, "text").(string)
			if !playerIDPattern.MatchString(playerID) || !textOK {
				continue
			}
			turn.PrivateMessages = append(turn.PrivateMessages, PrivateMessage{PlayerID: playerID, Text: text})
		}
	}

	turn.Combat = Combat{FirstTurn: "initiative", Enemies: []Enemy{}}
	if cm := asMap(v["combat"]); cm != nil {
		turn.Combat.Starts = cm["starts"] == true
		if ft, _ := cm["firstTurn"].(string); ft == "enemy" {
			turn.Combat.FirstTurn = "enemy"
		}
		if enemies, ok := asSlice(cm["enemies"]); ok {
			for _, e := range arrTake(enemies, 8) {
				turn.Combat.Enemies = append(turn.Combat.Enemies, Enemy{
					Name:            strOf(get(e, "name")),
					AC:              int(toNum(get(e, "ac"))),
					HP:              int(toNum(get(e, "hp"))),
					InitiativeBonus: int(toNum(get(e, "initiativeBonus"))),
					AttackBonus:     int(toNum(get(e, "attackBonus"))),
					Damage:          strOf(get(e, "damage")),
					DamageType:      strOf(get(e, "damageType")),
				})
			}
		}
	}

	turn.ActionIssues = []ActionIssue{}
	if arr, ok := asSlice(v["actionIssues"]); ok {
		for _, a := range arr {
			playerID, _ := get(a, "playerId").(string)
			message, messageOK := get(a, "message").(string)
			if !playerIDPattern.MatchString(playerID) || !messageOK {
				continue
			}
			turn.ActionIssues = append(turn.ActionIssues, ActionIssue{PlayerID: playerID, Message: message})
			if len(turn.ActionIssues) == 4 {
				break
			}
		}
	}

	turn.ExperienceAwards = []ExperienceAward{}
	if arr, ok := asSlice(v["experienceAwards"]); ok {
		for _, a := range arr {
			playerID, _ := get(a, "playerId").(string)
			amount, amountOK := get(a, "amount").(float64)
			reason, reasonOK := get(a, "reason").(string)
			if !playerIDPattern.MatchString(playerID) || !amountOK || math.IsInf(amount, 0) || math.IsNaN(amount) || !reasonOK {
				continue
			}
			turn.ExperienceAwards = append(turn.ExperienceAwards, ExperienceAward{
				PlayerID: playerID,
				Amount:   floorClampInt(amount, 0, 10000),
				Reason:   jsSlice(reason, 200),
			})
			if len(turn.ExperienceAwards) == 4 {
				break
			}
		}
	}

	return turn, nil
}
