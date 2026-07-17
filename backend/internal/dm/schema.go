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

// Effect is a narrative sheet change. Severity is model-facing; Amount is
// filled by NormalizeTurn from the service-owned severity table.
type Effect struct {
	TargetID  string `json:"targetId"`
	Kind      string `json:"kind"`
	Severity  string `json:"severity,omitempty"` // light | moderate | heavy (damage/healing)
	Amount    int    `json:"amount"`             // service-computed HP delta
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

// ExperienceAward grants XP to a player. Tier is model-facing; Amount is
// filled by NormalizeTurn from the service-owned milestone table.
type ExperienceAward struct {
	PlayerID string `json:"playerId"`
	Tier     string `json:"tier,omitempty"` // minor | clue | social | quest
	Amount   int    `json:"amount"`         // service-computed XP
	Reason   string `json:"reason"`
}

// ArcSignal is the DM's story-pacing output: phaseComplete marks the current
// arc phase's goal as achieved; nextGoal names the following phase's goal.
type ArcSignal struct {
	PhaseComplete bool   `json:"phaseComplete"`
	NextGoal      string `json:"nextGoal"`
}

// LootItem is one physical item found in the scene, granted to a player.
type LootItem struct {
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
}

// Loot is treasure the DM hands out this turn: gold is a party total the
// server splits evenly; items go to the named players' equipment.
type Loot struct {
	Gold  int        `json:"gold"`
	Items []LootItem `json:"items"`
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
	Arc              ArcSignal
	Loot             Loot
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
			sev := strings.ToLower(strings.TrimSpace(strOr(get(e, "severity"), "")))
			turn.Effects = append(turn.Effects, Effect{
				TargetID:  targetID,
				Kind:      kind,
				Severity:  sev,
				// Amount may be present from legacy models; NormalizeTurn overwrites it.
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

	// Optional pacing signal; absent on providers without the arc schema.
	if am := asMap(v["arc"]); am != nil {
		turn.Arc = ArcSignal{
			PhaseComplete: am["phaseComplete"] == true,
			NextGoal:      jsSlice(strOr(am["nextGoal"], ""), 240),
		}
	}

	// Optional treasure: party gold plus named items for individual players.
	if lm := asMap(v["loot"]); lm != nil {
		turn.Loot.Gold = floorClampInt(numOr(lm["gold"], 0), 0, 5000)
		if arr, ok := asSlice(lm["items"]); ok {
			for _, item := range arrTake(arr, 6) {
				playerID, _ := get(item, "playerId").(string)
				name := strings.TrimSpace(strOr(get(item, "name"), ""))
				if !playerIDPattern.MatchString(playerID) || name == "" {
					continue
				}
				turn.Loot.Items = append(turn.Loot.Items, LootItem{PlayerID: playerID, Name: jsSlice(name, 60)})
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
			reason, reasonOK := get(a, "reason").(string)
			if !playerIDPattern.MatchString(playerID) || !reasonOK {
				continue
			}
			tier := strings.ToLower(strings.TrimSpace(strOr(get(a, "tier"), "")))
			// Legacy models may still send amount without tier.
			amount := 0
			if rawAmt, ok := get(a, "amount").(float64); ok && !math.IsInf(rawAmt, 0) && !math.IsNaN(rawAmt) {
				amount = floorClampInt(rawAmt, 0, 10000)
			}
			if tier == "" && amount <= 0 {
				continue
			}
			turn.ExperienceAwards = append(turn.ExperienceAwards, ExperienceAward{
				PlayerID: playerID,
				Tier:     tier,
				Amount:   amount,
				Reason:   jsSlice(reason, 200),
			})
			if len(turn.ExperienceAwards) == 4 {
				break
			}
		}
	}

	return turn, nil
}
