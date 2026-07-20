package game

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"dndduet/internal/apperr"
	"dndduet/internal/rules"
	"dndduet/internal/store"
)

// importedCampaign captures the frontend Campaign JSON (localStorage vault
// export). Loosely-typed fields are normalized in normalizeImport, porting
// App.tsx migrateCampaign.
type importedCampaign struct {
	ID               string               `json:"id"`
	Title            string               `json:"title"`
	Chapter          string               `json:"chapter"`
	Scene            string               `json:"scene"`
	Round            int                  `json:"round"`
	Objective        string               `json:"objective"`
	ObjectiveContext string               `json:"objectiveContext"`
	Stakes           string               `json:"stakes"`
	SetupComplete    bool                 `json:"setupComplete"`
	StoryID          string               `json:"storyId"`
	Players          []json.RawMessage    `json:"players"`
	Story            []rules.StoryEntry   `json:"story"`
	Pending          map[string]string    `json:"pending"`
	Choices          []json.RawMessage    `json:"choices"`
	RequiredCheck    *rules.RequiredCheck `json:"requiredCheck"`
	Combat           *rules.CombatState   `json:"combat"`
	ScriptState      *ScriptState         `json:"scriptState"`
	ImagePrompt      string               `json:"imagePrompt"`
	UpdatedAt        string               `json:"updatedAt"`

	SelectedModel   string          `json:"selectedModel"`
	SelectedEffort  string          `json:"selectedEffort"`
	ImageBackend    string          `json:"imageBackend"`
	ForgeSettings   json.RawMessage `json:"forgeSettings"`
	FontScale       float64         `json:"fontScale"`
	ShowStatHints   *bool           `json:"showStatHints"`
	AutoSceneImages *bool           `json:"autoSceneImages"`
	DismissedTips   []string        `json:"dismissedTips"`
	SceneImages     json.RawMessage `json:"sceneImages"`
	SceneImage      json.RawMessage `json:"sceneImage"`
}

type exportEnvelope struct {
	Format   string          `json:"format"`
	Version  int             `json:"version"`
	Campaign json.RawMessage `json:"campaign"`
}

var errInvalidImport = apperr.New(400, "這不是有效的 D&D Duet 戰役檔案。")

// Import ingests a campaign JSON document (the frontend export format
// {format,version,campaign} or a bare Campaign object), normalizes it like
// App.tsx migrateCampaign, and persists it. The original id is preserved so
// existing story memory (memory_events keyed by the same id) keeps working;
// overwrite must be true to replace an existing campaign.
func (s *Service) Import(raw []byte, overwrite bool) (View, error) {
	body := raw
	var env exportEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Campaign) > 0 {
		body = env.Campaign
	}

	var imp importedCampaign
	if err := json.Unmarshal(body, &imp); err != nil {
		return View{}, errInvalidImport
	}
	// Mirror the TS validation: title string + players/story arrays required.
	var shape struct {
		Title   *string           `json:"title"`
		Players []json.RawMessage `json:"players"`
		Story   []json.RawMessage `json:"story"`
	}
	if err := json.Unmarshal(body, &shape); err != nil || shape.Title == nil || shape.Players == nil || shape.Story == nil {
		return View{}, errInvalidImport
	}

	id := strings.TrimSpace(imp.ID)
	if id == "" {
		id = s.NewCampaignID()
	}

	unlock := s.Lock(id)
	defer unlock()
	if _, ok, err := s.store.GetCampaign(id); err != nil {
		return View{}, err
	} else if ok && !overwrite {
		return View{}, apperr.New(409, "伺服器上已有同 ID 的戰役；確認要覆蓋後再試一次。")
	}

	players, err := normalizePlayers(imp.Players)
	if err != nil {
		return View{}, err
	}

	row := store.CampaignRow{
		ID:               id,
		Title:            clampStr(imp.Title, 180),
		Chapter:          clampStr(imp.Chapter, 120),
		Scene:            clampStr(imp.Scene, 240),
		Round:            maxInt(1, imp.Round),
		Objective:        clampStr(imp.Objective, 240),
		ObjectiveContext: clampStr(imp.ObjectiveContext, 600),
		Stakes:           clampStr(imp.Stakes, 300),
		SetupComplete:    imp.SetupComplete,
		ImagePrompt:      imp.ImagePrompt,
		DocVersion:       1,
	}

	row.Choices = normalizeChoices(imp.Choices)
	if imp.Pending != nil {
		if data, err := json.Marshal(imp.Pending); err == nil {
			row.Pending = string(data)
		}
	}
	if imp.RequiredCheck != nil {
		if data, err := json.Marshal(imp.RequiredCheck); err == nil {
			row.RequiredCheck = string(data)
		}
	}
	row.Settings = buildImportSettings(imp)

	row = s.stamp(row)
	characterRows, err := s.characterRows(id, players, row.UpdatedAt)
	if err != nil {
		return View{}, err
	}

	var combatData *string
	if imp.Combat != nil && imp.Combat.Active {
		if data, err := json.Marshal(imp.Combat); err == nil {
			encoded := string(data)
			combatData = &encoded
		}
	}

	// Scripted-module progress round-trips when the module still exists and
	// the saved node is valid; otherwise the campaign continues freeform.
	var scriptData *string
	if state := imp.ScriptState; state != nil {
		if mod := scriptModules[state.ScriptID]; mod != nil && mod.node(state.NodeID) != nil {
			if data, err := json.Marshal(state); err == nil {
				encoded := string(data)
				scriptData = &encoded
			}
		}
	}

	nowMs := s.now().UnixMilli()
	entries := make([]store.StoryRow, 0, len(imp.Story))
	for i, e := range imp.Story {
		audience := e.Audience
		if audience == "" {
			audience = "public"
		}
		entries = append(entries, store.StoryRow{
			Speaker:   e.Speaker,
			Audience:  audience,
			Text:      e.Text,
			TimeLabel: e.Time,
			CreatedAt: nowMs + int64(i), // preserve ordering; display uses TimeLabel
		})
	}
	if err := s.store.SaveCampaignState(store.CampaignStateWrite{
		Campaign: row, Characters: characterRows, Combat: combatData,
		ScriptState: scriptData, Story: entries, Replace: true,
	}); err != nil {
		return View{}, err
	}

	return s.assembleView(row)
}

// normalizePlayers ports the player branch of App.tsx migrateCampaign: full
// character documents get their spell catalog entries refreshed and
// experience/classLevels backfilled; anything else falls back to a fresh
// level-3 character of the closest class.
func normalizePlayers(raws []json.RawMessage) ([]rules.Character, error) {
	if len(raws) == 0 {
		return nil, apperr.New(400, "戰役檔案缺少隊伍角色。")
	}
	if len(raws) > 4 {
		raws = raws[:4]
	}
	players := make([]rules.Character, 0, len(raws))
	for i, raw := range raws {
		pid := fmt.Sprintf("player%d", i+1)

		var probe struct {
			Name      string               `json:"name"`
			ClassName string               `json:"className"`
			Level     int                  `json:"level"`
			HP        *float64             `json:"hp"`
			Abilities *rules.AbilityScores `json:"abilities"`
			Resources []json.RawMessage    `json:"resources"`
			Features  []json.RawMessage    `json:"features"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			return nil, errInvalidImport
		}

		if probe.Abilities != nil && probe.Resources != nil && probe.Features != nil {
			var c rules.Character
			if err := json.Unmarshal(raw, &c); err != nil {
				return nil, errInvalidImport
			}
			c.ID = pid
			refreshSpellsFromCatalog(&c)
			if c.Experience <= 0 {
				level := c.Level
				if level == 0 {
					level = 3
				}
				c.Experience = rules.ExperienceForLevel(level)
			}
			if c.AbilityPoints < 0 {
				c.AbilityPoints = 0
			}
			if len(c.ClassLevels) == 0 {
				base := closestClass(c.ClassName)
				level := c.Level
				if level == 0 {
					level = 3
				}
				c.ClassLevels = []rules.ClassLevel{{ClassName: base, Level: level, Subclass: c.Subclass}}
			}
			players = append(players, c)
			continue
		}

		className := closestClass(probe.ClassName)
		name := strings.TrimSpace(probe.Name)
		if name == "" {
			name = fmt.Sprintf("冒險者 %d", i+1)
		}
		migrated := rules.CreateLevel3Character(pid, name, className)
		if probe.HP != nil && !math.IsNaN(*probe.HP) {
			hp := int(*probe.HP)
			if hp < migrated.MaxHP && hp >= 0 {
				migrated.HP = hp
			}
		}
		migrated.ClassLevels = []rules.ClassLevel{{ClassName: className, Level: 3, Subclass: migrated.Subclass}}
		players = append(players, migrated)
	}
	return players, nil
}

// refreshSpellsFromCatalog ports the migrateCampaign spell merge: stored spell
// state (prepared/inSpellbook/freeUse) is kept while catalog text/effects win
// so old saves pick up spell data fixes.
func refreshSpellsFromCatalog(c *rules.Character) {
	if c.Spellcasting == nil {
		return
	}
	for i, sp := range c.Spellcasting.Spells {
		def, ok := rules.SpellCatalog[sp.ID]
		if !ok {
			continue
		}
		merged := sp
		merged.Name = def.Name
		merged.EnglishName = def.EnglishName
		merged.Level = def.Level
		merged.School = def.School
		merged.CastingTime = def.CastingTime
		merged.Range = def.Range
		merged.Duration = def.Duration
		merged.Description = def.Description
		merged.Concentration = def.Concentration
		merged.Ritual = def.Ritual
		if def.Effect != nil {
			merged.Effect = def.Effect
		}
		c.Spellcasting.Spells[i] = merged
	}
}

// closestClass mirrors the TS classNames.find(name.includes(candidate)) || '戰士'.
func closestClass(className string) string {
	for _, candidate := range rules.ClassNames {
		if strings.Contains(className, candidate) {
			return candidate
		}
	}
	return "戰士"
}

// normalizeChoices tolerates the legacy bare-string choice format.
func normalizeChoices(raws []json.RawMessage) string {
	choices := make([]rules.Choice, 0, len(raws))
	for _, raw := range raws {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			if strings.TrimSpace(str) != "" {
				choices = append(choices, rules.Choice{Text: str})
			}
			continue
		}
		var c rules.Choice
		if err := json.Unmarshal(raw, &c); err == nil && c.Text != "" {
			choices = append(choices, c)
		}
	}
	data, err := json.Marshal(choices)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func buildImportSettings(imp importedCampaign) string {
	settings := map[string]any{}
	put := func(k string, v any) { settings[k] = v }
	if imp.StoryID != "" {
		put("storyId", imp.StoryID)
	}
	if imp.SelectedModel != "" {
		put("selectedModel", imp.SelectedModel)
	}
	if imp.SelectedEffort != "" {
		put("selectedEffort", imp.SelectedEffort)
	}
	if imp.ImageBackend != "" {
		put("imageBackend", imp.ImageBackend)
	}
	if len(imp.ForgeSettings) > 0 && string(imp.ForgeSettings) != "null" {
		put("forgeSettings", json.RawMessage(imp.ForgeSettings))
	}
	fontScale := imp.FontScale
	if fontScale == 0 {
		fontScale = 1
	}
	put("fontScale", math.Max(0.85, math.Min(1.25, fontScale)))
	put("showStatHints", imp.ShowStatHints == nil || *imp.ShowStatHints)
	if imp.AutoSceneImages != nil {
		put("autoSceneImages", *imp.AutoSceneImages)
	}
	if imp.DismissedTips != nil {
		put("dismissedTips", imp.DismissedTips)
	} else {
		put("dismissedTips", []string{})
	}
	if len(imp.SceneImages) > 0 && string(imp.SceneImages) != "null" {
		put("sceneImages", json.RawMessage(imp.SceneImages))
	} else if len(imp.SceneImage) > 0 && string(imp.SceneImage) != "null" {
		put("sceneImages", []json.RawMessage{imp.SceneImage})
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// Export renders the campaign as the frontend export format
// ({format:'dnd-duet-campaign',version:2,campaign}) with settings flattened
// back onto the campaign object so the file also imports into old clients.
func (s *Service) Export(id string) ([]byte, error) {
	view, err := s.View(id)
	if err != nil {
		return nil, err
	}
	campaign := map[string]any{}
	viewJSON, err := json.Marshal(view)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(viewJSON, &campaign); err != nil {
		return nil, err
	}
	var settings map[string]any
	if err := json.Unmarshal(view.Settings, &settings); err == nil {
		for k, v := range settings {
			campaign[k] = v
		}
	}
	delete(campaign, "settings")
	delete(campaign, "xpProgress")
	// View.script is the spoiler-safe progress slice; the export carries the
	// full engine state instead so a scripted campaign round-trips intact.
	delete(campaign, "script")
	if data, ok, err := s.store.ScriptState(id); err == nil && ok {
		var state ScriptState
		if json.Unmarshal([]byte(data), &state) == nil {
			campaign["scriptState"] = state
		}
	}
	campaign["schemaVersion"] = 3

	return json.MarshalIndent(map[string]any{
		"format":     "dnd-duet-campaign",
		"version":    2,
		"exportedAt": s.now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		"campaign":   campaign,
	}, "", "  ")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
