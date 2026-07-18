// Package game orchestrates server-authoritative campaign state: it owns the
// SQLite documents (campaigns, characters, combat, story journal) and applies
// the rules engine to them. HTTP handlers call into this service; every
// mutating method returns the same full View the frontend renders, so the
// client never runs rule logic.
package game

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"dndduet/internal/apperr"
	"dndduet/internal/rules"
	"dndduet/internal/store"
)

// Service is the game-state orchestrator. Campaign documents are read-modify-
// write JSON blobs, so all mutations for one campaign serialise on a
// per-campaign mutex.
type Service struct {
	store *store.Store
	now   func() time.Time
	dice  rules.RandomSource

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// New creates a Service backed by st. now is injectable for tests; nil means
// time.Now.
func New(st *store.Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: st, now: now, dice: rules.DefaultRandom, locks: map[string]*sync.Mutex{}}
}

// WithDice overrides the random source (tests).
func (s *Service) WithDice(dice rules.RandomSource) *Service {
	if dice != nil {
		s.dice = dice
	}
	return s
}

// Lock acquires the per-campaign mutex and returns the unlock func.
func (s *Service) Lock(id string) func() {
	s.mu.Lock()
	l, ok := s.locks[id]
	if !ok {
		l = &sync.Mutex{}
		s.locks[id] = l
	}
	s.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// View is the full campaign state the frontend renders. Field names match the
// frontend Campaign type so the client can setCampaign(view) wholesale.
type View struct {
	ID               string                      `json:"id"`
	Title            string                      `json:"title"`
	Chapter          string                      `json:"chapter"`
	Scene            string                      `json:"scene"`
	Round            int                         `json:"round"`
	Objective        string                      `json:"objective"`
	ObjectiveContext string                      `json:"objectiveContext"`
	Stakes           string                      `json:"stakes"`
	SetupComplete    bool                        `json:"setupComplete"`
	Players          []rules.Character           `json:"players"`
	Story            []rules.StoryEntry          `json:"story"`
	Pending          map[string]string           `json:"pending"`
	Choices          []rules.Choice              `json:"choices"`
	RequiredCheck    *rules.RequiredCheck        `json:"requiredCheck"`
	Combat           *rules.CombatState          `json:"combat,omitempty"`
	StoryArc         *StoryArc                   `json:"storyArc,omitempty"`
	Script           *ScriptProgress             `json:"script,omitempty"`
	ImagePrompt      string                      `json:"imagePrompt,omitempty"`
	Settings         json.RawMessage             `json:"settings"`
	XPProgress       map[string]rules.XPProgress `json:"xpProgress"`
	UpdatedAt        string                      `json:"updatedAt"`
}

// Summary is one campaign list entry.
type Summary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Scene     string `json:"scene"`
	Round     int    `json:"round"`
	UpdatedAt string `json:"updatedAt"`
}

// storyViewLimit bounds how much journal the View carries; the table keeps
// everything.
const storyViewLimit = 400

// ErrNotFound is returned when a campaign id has no row.
var ErrNotFound = apperr.New(404, "找不到這個戰役。")

func isoTime(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// timeLabel renders the zh-TW HH:mm display label the frontend shows next to
// story entries (App.tsx now()).
func timeLabel(t time.Time) string {
	return t.Format("15:04")
}

func randomID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	h := hex.EncodeToString(buf)
	// uuid-ish grouping to match the frontend campaign-<ms>-<uuid> shape.
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// NewCampaignID mirrors the frontend campaignId(): campaign-<ms>-<uuid>.
func (s *Service) NewCampaignID() string {
	return fmt.Sprintf("campaign-%d-%s", s.now().UnixMilli(), randomID())
}

// View assembles the full campaign view from the store.
func (s *Service) View(id string) (View, error) {
	row, ok, err := s.store.GetCampaign(id)
	if err != nil {
		return View{}, err
	}
	if !ok {
		return View{}, ErrNotFound
	}
	return s.assembleView(row)
}

func (s *Service) assembleView(row store.CampaignRow) (View, error) {
	players, err := s.loadCharacters(row.ID)
	if err != nil {
		return View{}, err
	}
	// Read path sees the same backfill as the write path (loadState), so a
	// weapon bought before shop weapons granted attacks shows up immediately;
	// it persists on the next state-mutating call.
	for i := range players {
		ensureShopWeaponAttacks(&players[i])
		rules.EnsureDerivedDefaults(&players[i])
	}

	var combat *rules.CombatState
	if data, ok, err := s.store.Combat(row.ID); err != nil {
		return View{}, err
	} else if ok {
		combat = &rules.CombatState{}
		if err := json.Unmarshal([]byte(data), combat); err != nil {
			return View{}, fmt.Errorf("combat document for %s is corrupt: %w", row.ID, err)
		}
	}

	var arc *StoryArc
	if data, ok, err := s.store.StoryArc(row.ID); err != nil {
		return View{}, err
	} else if ok {
		arc = &StoryArc{}
		if err := json.Unmarshal([]byte(data), arc); err != nil {
			arc = nil
		}
	}

	var script *ScriptProgress
	if data, ok, err := s.store.ScriptState(row.ID); err != nil {
		return View{}, err
	} else if ok {
		state := &ScriptState{}
		if err := json.Unmarshal([]byte(data), state); err == nil {
			script = scriptProgress(state)
			// The read path shows the same module-pinned arc as the write
			// path (loadState); it persists on the next mutating call.
			if mod := scriptModuleFor(state); mod != nil && arc != nil {
				syncScriptArc(arc, mod, state, row.Round)
			}
		}
	}

	tail, err := s.store.StoryTail(row.ID, storyViewLimit)
	if err != nil {
		return View{}, err
	}
	story := make([]rules.StoryEntry, 0, len(tail))
	for _, e := range tail {
		label := e.TimeLabel
		if label == "" {
			label = timeLabel(time.UnixMilli(e.CreatedAt))
		}
		audience := e.Audience
		if audience == "public" {
			audience = ""
		}
		story = append(story, rules.StoryEntry{
			ID:       fmt.Sprintf("%s-%d", row.ID, e.Seq),
			Speaker:  e.Speaker,
			Text:     e.Text,
			Time:     label,
			Audience: audience,
		})
	}

	choices := []rules.Choice{}
	if row.Choices != "" {
		if err := json.Unmarshal([]byte(row.Choices), &choices); err != nil {
			choices = []rules.Choice{}
		}
	}
	pending := map[string]string{}
	if row.Pending != "" {
		if err := json.Unmarshal([]byte(row.Pending), &pending); err != nil {
			pending = map[string]string{}
		}
	}
	var check *rules.RequiredCheck
	if row.RequiredCheck != "" {
		check = &rules.RequiredCheck{}
		if err := json.Unmarshal([]byte(row.RequiredCheck), check); err != nil {
			check = nil
		}
	}

	xp := make(map[string]rules.XPProgress, len(players))
	for _, p := range players {
		xp[p.ID] = rules.ExperienceToNextLevel(p)
	}

	settings := json.RawMessage(row.Settings)
	if len(settings) == 0 {
		settings = json.RawMessage("{}")
	}

	return View{
		ID:               row.ID,
		Title:            row.Title,
		Chapter:          row.Chapter,
		Scene:            row.Scene,
		Round:            row.Round,
		Objective:        row.Objective,
		ObjectiveContext: row.ObjectiveContext,
		Stakes:           row.Stakes,
		SetupComplete:    row.SetupComplete,
		Players:          players,
		Story:            story,
		Pending:          pending,
		Choices:          choices,
		RequiredCheck:    check,
		Combat:           combat,
		StoryArc:         arc,
		Script:           script,
		ImagePrompt:      row.ImagePrompt,
		Settings:         settings,
		XPProgress:       xp,
		UpdatedAt:        isoTime(row.UpdatedAt),
	}, nil
}

func (s *Service) loadCharacters(campaignID string) ([]rules.Character, error) {
	crows, err := s.store.Characters(campaignID)
	if err != nil {
		return nil, err
	}
	players := make([]rules.Character, 0, len(crows))
	for _, cr := range crows {
		var c rules.Character
		if err := json.Unmarshal([]byte(cr.Data), &c); err != nil {
			return nil, fmt.Errorf("character %s/%s is corrupt: %w", campaignID, cr.PlayerID, err)
		}
		c.ID = cr.PlayerID
		players = append(players, c)
	}
	return players, nil
}

// saveCharacters persists the full party.
func (s *Service) saveCharacters(campaignID string, players []rules.Character) error {
	nowMs := s.now().UnixMilli()
	for _, p := range players {
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		if err := s.store.SaveCharacter(store.CharacterRow{
			CampaignID: campaignID, PlayerID: p.ID, Name: p.Name, Data: string(data), UpdatedAt: nowMs,
		}); err != nil {
			return err
		}
	}
	return nil
}

// touch persists row with a fresh updated_at.
func (s *Service) touch(row store.CampaignRow) (store.CampaignRow, error) {
	row.UpdatedAt = s.now().UnixMilli()
	if row.CreatedAt == 0 {
		row.CreatedAt = row.UpdatedAt
	}
	return row, s.store.SaveCampaign(row)
}

// mustCampaign loads a campaign row or returns ErrNotFound.
func (s *Service) mustCampaign(id string) (store.CampaignRow, error) {
	row, ok, err := s.store.GetCampaign(id)
	if err != nil {
		return store.CampaignRow{}, err
	}
	if !ok {
		return store.CampaignRow{}, ErrNotFound
	}
	return row, nil
}

// List returns campaign summaries, most recent first.
func (s *Service) List() ([]Summary, error) {
	rows, err := s.store.ListCampaigns()
	if err != nil {
		return nil, err
	}
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, Summary{ID: r.ID, Title: r.Title, Scene: r.Scene, Round: r.Round, UpdatedAt: isoTime(r.UpdatedAt)})
	}
	return out, nil
}

// Delete removes a campaign and its documents.
func (s *Service) Delete(id string) error {
	unlock := s.Lock(id)
	defer unlock()
	if _, err := s.mustCampaign(id); err != nil {
		return err
	}
	return s.store.DeleteCampaign(id)
}

// PlayerSeed is one party member in a create request.
type PlayerSeed struct {
	Name       string               `json:"name"`
	ClassName  string               `json:"className"`
	Level      int                  `json:"level"`
	Species    string               `json:"species"`
	Background string               `json:"background"`
	Abilities  *rules.AbilityScores `json:"abilities"`
}

// CreateParams seeds a new campaign. The story-preset fields come from the
// client (presets are display data, not rules); the server clamps them.
type CreateParams struct {
	ID               string          `json:"id"`
	StoryID          string          `json:"storyId"`
	StoryMode        string          `json:"storyMode"` // scripted | freeform ("" = scripted when a module exists)
	Title            string          `json:"title"`
	Chapter          string          `json:"chapter"`
	Scene            string          `json:"scene"`
	Objective        string          `json:"objective"`
	ObjectiveContext string          `json:"objectiveContext"`
	Stakes           string          `json:"stakes"`
	Opening          string          `json:"opening"`
	Players          []PlayerSeed    `json:"players"`
	Settings         json.RawMessage `json:"settings"`
}

func clampStr(v string, max int) string {
	r := []rune(strings.TrimSpace(v))
	if len(r) > max {
		return string(r[:max])
	}
	return string(r)
}

// Create builds a new campaign with server-created characters and the opening
// journal entries (mirrors App.tsx completeSetup).
func (s *Service) Create(p CreateParams) (View, error) {
	if len(p.Players) < 1 || len(p.Players) > 4 {
		return View{}, apperr.New(400, "隊伍需要 1 到 4 位冒險者。")
	}
	if strings.TrimSpace(p.Title) == "" {
		return View{}, apperr.New(400, "戰役需要標題。")
	}

	id := strings.TrimSpace(p.ID)
	if id == "" {
		id = s.NewCampaignID()
	}
	unlock := s.Lock(id)
	defer unlock()
	if _, ok, err := s.store.GetCampaign(id); err != nil {
		return View{}, err
	} else if ok {
		return View{}, apperr.New(409, "這個戰役 ID 已存在。")
	}

	players := make([]rules.Character, 0, len(p.Players))
	for i, seed := range p.Players {
		pid := fmt.Sprintf("player%d", i+1)
		name := clampStr(seed.Name, 100)
		if name == "" {
			name = fmt.Sprintf("冒險者 %d", i+1)
		}
		c := rules.CreateConfiguredCharacter(pid, name, seed.ClassName, rules.BuildOptions{
			Level:      seed.Level,
			Species:    clampStr(seed.Species, 60),
			Background: clampStr(seed.Background, 60),
			Abilities:  seed.Abilities,
		})
		c.Gold = 100 // starting purse; chests and quest rewards add more
		players = append(players, c)
	}

	settings, err := mergeSettings(p.Settings, map[string]any{"storyId": strings.TrimSpace(p.StoryID)})
	if err != nil {
		return View{}, apperr.New(400, "settings 格式錯誤。")
	}

	// Scripted mode starts at the module's entry node; its choices must be in
	// the campaign row from round one, because the scripted UI has no free-text
	// input — without seeded choices no player could ever lock an action.
	var scriptState *ScriptState
	choices := "[]"
	if state := newScriptState(p.StoryID); state != nil && p.StoryMode != "freeform" {
		scriptState = state
		if mod := scriptModuleFor(state); mod != nil {
			if node := mod.node(state.NodeID); node != nil {
				seeded := make([]rules.Choice, 0, len(node.Choices))
				for _, c := range node.Choices {
					seeded = append(seeded, rules.Choice{Text: c.Text})
				}
				if data, err := json.Marshal(seeded); err == nil {
					choices = string(data)
				}
			}
		}
	}

	row := store.CampaignRow{
		ID:               id,
		Title:            clampStr(p.Title, 180),
		Chapter:          clampStr(p.Chapter, 120),
		Scene:            clampStr(p.Scene, 240),
		Round:            1,
		Objective:        clampStr(p.Objective, 240),
		ObjectiveContext: clampStr(p.ObjectiveContext, 600),
		Stakes:           clampStr(p.Stakes, 300),
		SetupComplete:    true,
		Choices:          choices,
		Pending:          "{}",
		Settings:         settings,
		DocVersion:       1,
	}
	if row, err = s.touch(row); err != nil {
		return View{}, err
	}
	if err := s.saveCharacters(id, players); err != nil {
		return View{}, err
	}

	if scriptState != nil {
		data, err := json.Marshal(scriptState)
		if err != nil {
			return View{}, err
		}
		if err := s.store.SaveScriptState(id, string(data), s.now().UnixMilli()); err != nil {
			return View{}, err
		}
	}

	nowMs := s.now().UnixMilli()
	label := timeLabel(s.now())
	entries := []store.StoryRow{}
	if opening := strings.TrimSpace(p.Opening); opening != "" {
		entries = append(entries, store.StoryRow{Speaker: "dm", Text: clampStr(opening, 4000), TimeLabel: label, CreatedAt: nowMs})
	}
	entries = append(entries, store.StoryRow{
		Speaker:   "system",
		Text:      fmt.Sprintf("隊伍已建立，共 %d 位冒險者。目前目標：%s。", len(players), row.Objective),
		TimeLabel: label,
		CreatedAt: nowMs,
	})
	if err := s.store.AppendStoryEntries(id, entries); err != nil {
		return View{}, err
	}

	return s.assembleView(row)
}

// mergeSettings merges extra keys into a raw settings JSON object.
func mergeSettings(raw json.RawMessage, extra map[string]any) (string, error) {
	base := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &base); err != nil {
			return "", err
		}
	}
	for k, v := range extra {
		if str, ok := v.(string); ok && str == "" {
			continue
		}
		base[k] = v
	}
	out, err := json.Marshal(base)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// UpdateSettings shallow-merges a settings patch (model/effort/UI prefs) into
// the campaign settings document.
func (s *Service) UpdateSettings(id string, patch json.RawMessage) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	row, err := s.mustCampaign(id)
	if err != nil {
		return View{}, err
	}
	extra := map[string]any{}
	if len(patch) > 0 {
		if err := json.Unmarshal(patch, &extra); err != nil {
			return View{}, apperr.New(400, "settings 格式錯誤。")
		}
	}
	merged, err := mergeSettings(json.RawMessage(row.Settings), extra)
	if err != nil {
		return View{}, apperr.New(400, "settings 格式錯誤。")
	}
	row.Settings = merged
	if row, err = s.touch(row); err != nil {
		return View{}, err
	}
	return s.assembleView(row)
}

// AppendStory adds journal entries (used by DM turns and system logs) with
// fresh timestamps.
func (s *Service) AppendStory(id string, entries []store.StoryRow) error {
	nowMs := s.now().UnixMilli()
	label := timeLabel(s.now())
	for i := range entries {
		if entries[i].CreatedAt == 0 {
			entries[i].CreatedAt = nowMs
		}
		if entries[i].TimeLabel == "" {
			entries[i].TimeLabel = label
		}
	}
	return s.store.AppendStoryEntries(id, entries)
}

// ReplaceLastPublicDM rewrites the most recent public DM narration without
// advancing the round or changing mechanical state.
func (s *Service) ReplaceLastPublicDM(id, text string) error {
	unlock := s.Lock(id)
	defer unlock()
	if _, err := s.mustCampaign(id); err != nil {
		return err
	}
	return s.store.ReplaceLastPublicDMText(id, text)
}
