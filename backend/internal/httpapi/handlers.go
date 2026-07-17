package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dndduet/internal/dm"
	"dndduet/internal/game"
	"dndduet/internal/images"
	"dndduet/internal/jsutil"
	"dndduet/internal/provider"
	"dndduet/internal/store"
	"dndduet/internal/tts"
)

// storyIDPattern validates the untrusted campaign id. The frontend id format is
// campaign-<ms>-<uuid> (campaign-storage.ts), which fits this superset.
var storyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func sanitizeStoryID(v string) string {
	v = strings.TrimSpace(v)
	if storyIDPattern.MatchString(v) {
		return v
	}
	return ""
}

type connectionBody struct {
	Alive   bool   `json:"alive"`
	StoryId string `json:"storyId"`
}

type needsConsentBody struct {
	Error        string `json:"error"`
	NeedsConsent bool   `json:"needsConsent"`
}

// handleCodexConnection reports the current persistent-connection binding.
func (s *Server) handleCodexConnection(w http.ResponseWriter, r *http.Request) {
	cs := s.Provider.ConnectionState()
	writeJSON(w, http.StatusOK, connectionBody{Alive: cs.Alive, StoryId: cs.StoryID})
}

// handleCodexConnect establishes (or rebinds) the persistent connection to the
// given story. This is the explicit player-consent action; it is the only path
// that (re)creates a connection.
func (s *Server) handleCodexConnect(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	body, err := readJSONBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	storyID := sanitizeStoryID(jsutil.StrOr(jsutil.Get(body, "campaignId"), ""))
	if storyID == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "缺少有效的 campaignId。"})
		return
	}
	if err := s.Provider.Connect(ctx, storyID); err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	cs := s.Provider.ConnectionState()
	writeJSON(w, http.StatusOK, connectionBody{Alive: cs.Alive, StoryId: cs.StoryID})
}

type statusResponse struct {
	ForgeDefaults map[string]images.ForgeOptions
	Connected     bool                   `json:"connected"`
	Provider      string                 `json:"provider"`
	Model         string                 `json:"model"`
	Models        []provider.ModelOption `json:"models"`
	Efforts       []provider.ModelOption `json:"efforts"`
	ImageModel    string                 `json:"imageModel"`
	ImageBackends []provider.ModelOption `json:"imageBackends"`
	ImageBackend  string                 `json:"imageBackend"`
	Message       string                 `json:"message,omitempty"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.Provider.Status(r.Context())
	imageModel := s.Provider.ImageModel()
	defaultBackend := s.DefaultImageBackend
	if defaultBackend == "" {
		defaultBackend = "codex"
	}
	if renderer, err := s.imageRenderer(""); err == nil {
		imageModel = renderer.Model()
	}
	writeJSON(w, http.StatusOK, statusResponse{
		ForgeDefaults: s.forgeDefaults(),
		Connected:     status.Configured,
		Provider:      status.Provider,
		Model:         status.Model,
		Models:        s.Provider.ModelOptions(),
		Efforts:       s.Provider.EffortOptions(),
		ImageModel:    imageModel,
		ImageBackends: s.imageBackendOptions(),
		ImageBackend:  defaultBackend,
		Message:       status.Message,
	})
}

type dmResponse struct {
	View            *game.View          `json:"view,omitempty"`
	Text            string              `json:"text"`
	Choices         []dm.Choice         `json:"choices"`
	RequiresCheck   bool                `json:"requiresCheck"`
	Check           *dm.Check           `json:"check"`
	PrivateMessages []dm.PrivateMessage `json:"privateMessages"`
	ActionIssues    []game.ActionIssue  `json:"actionIssues"`
	Model           string              `json:"model"`
}

// dmRequest is the slim /api/dm body: everything else (characters, combat,
// history, campaign meta) now comes from the server's own store.
type dmRequest struct {
	CampaignID string `json:"campaignId"`
	Model      string `json:"model"`
	Effort     string `json:"effort"`
	Actions    []struct {
		PlayerID string `json:"playerId"`
		Text     string `json:"text"`
	} `json:"actions"`
	Intents   map[string]game.Intent `json:"intents"`
	CheckRoll *struct {
		Natural int `json:"natural"`
	} `json:"checkRoll"`
	CombatConclusion *struct {
		Outcome string `json:"outcome"`
		Summary string `json:"summary"`
	} `json:"combatConclusion"`
}

func (s *Server) handleDm(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 210*time.Second)
	defer cancel()

	raw, err := readRawBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	var req dmRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "請求格式錯誤。"})
		return
	}
	storyID := sanitizeStoryID(req.CampaignID)
	if storyID == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "缺少有效的 campaignId。"})
		return
	}

	// Prepare the turn from server-authoritative state. Mechanical validation
	// failures return 422 without ever calling the AI.
	var prepared game.PreparedDMTurn
	switch {
	case req.CheckRoll != nil:
		prepared, err = s.Game.PrepareCheckTurn(storyID, req.CheckRoll.Natural)
	case req.CombatConclusion != nil:
		prepared, err = s.Game.PrepareConclusionTurn(storyID, req.CombatConclusion.Outcome, req.CombatConclusion.Summary)
	default:
		actions := map[string]string{}
		for _, a := range req.Actions {
			actions[a.PlayerID] = a.Text
		}
		prepared, err = s.Game.PrepareActionsTurn(storyID, actions, req.Intents)
	}
	if err != nil {
		var issues *game.ActionIssuesError
		if errors.As(err, &issues) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":        "有行動未通過規則驗證，故事尚未推進。",
				"actionIssues": issues.Issues,
			})
			return
		}
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}

	// Delta mode: only when a live connection is bound to this story and memory
	// is enabled, so Codex has the persistent thread and can read the memory +
	// rules files for prior context and the full ruleset. Otherwise fall back
	// to the full inline preamble.
	if s.Memory != nil {
		if cs := s.Provider.ConnectionState(); cs.Alive && cs.StoryID == storyID {
			if err := s.Memory.Materialise(storyID); err == nil {
				prepared.Input.DeltaMode = true
				prepared.Input.MemRef = s.Memory.Ref(storyID)
				if dossier, err := s.Game.BuildRulesDossier(storyID); err == nil {
					if err := s.Memory.MaterialiseRules(storyID, dossier); err == nil {
						prepared.Input.RulesRef = s.Memory.RulesRef(storyID)
					}
				}
			}
		}
	}

	prompt := dm.BuildDMRequestV2(prepared.Input)
	selectedModel, err := s.Provider.NormalizeModel(req.Model)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	selectedEffort, err := s.Provider.NormalizeEffort(req.Effort)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	// The mini-preamble is safe only when the full ruleset is readable from
	// the rules file (delta mode); otherwise the complete preamble ships inline.
	slim := prepared.Input.DeltaMode && prepared.Input.RulesRef != ""
	output, err := dm.RunDungeonMaster(ctx, s.Provider, prompt, selectedModel, selectedEffort, s.SchemaPath, s.ProviderCWD, storyID, slim)
	if err != nil {
		log.Printf("[dm] generation failed: %v", err)
		if errors.Is(err, provider.ErrNeedsConsent) {
			writeJSON(w, http.StatusConflict, needsConsentBody{Error: err.Error(), NeedsConsent: true})
			return
		}
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}

	applied, err := s.Game.ApplyDMTurn(storyID, prepared, output)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}

	// Persist this turn to memory (raw events sync; compaction is async). The
	// AI-vetoed case did not advance the story, so nothing is recorded.
	if s.Memory != nil && len(applied.Rejected) == 0 {
		s.recordMemory(storyID, prepared, output)
	}

	checkText := ""
	if output.RequiresCheck && output.Check != nil {
		checkText = "\n\n檢定：" + output.Check.Character + " 進行 DC " + strconv.Itoa(output.Check.DC) + " 的" + output.Check.Ability + "（" + output.Check.Skill + "）檢定。" + output.Check.Reason
	}
	choiceText := ""
	if len(output.Choices) > 0 {
		texts := make([]string, len(output.Choices))
		for i, c := range output.Choices {
			texts[i] = c.Text
		}
		choiceText = "\n\n可考慮：" + strings.Join(texts, "／")
	}

	status := s.Provider.Status(ctx)
	model := selectedModel
	if model == "" {
		model = status.Model
	}

	text := output.Narration + checkText + choiceText
	if len(applied.Rejected) > 0 {
		text = ""
	}
	writeJSON(w, http.StatusOK, dmResponse{
		View:            &applied.View,
		Text:            text,
		Choices:         output.Choices,
		RequiresCheck:   output.RequiresCheck,
		Check:           output.Check,
		PrivateMessages: output.PrivateMessages,
		ActionIssues:    applied.Rejected,
		Model:           model,
	})
}

func parseForgeOptions(body map[string]any, renderer images.Renderer) (*images.ForgeOptions, error) {
	raw := jsutil.AsMap(body[`forge`])
	enabled, _ := raw[`enabled`].(bool)
	if !enabled {
		return nil, nil
	}
	configurable, ok := renderer.(interface{ SceneDefaults() images.ForgeOptions })
	if !ok {
		return nil, nil
	}
	opts := configurable.SceneDefaults()
	if value, ok := raw[`positivePrompt`].(string); ok {
		opts.PositivePrompt = strings.TrimSpace(value)
	}
	if value, ok := raw[`negativePrompt`].(string); ok {
		opts.NegativePrompt = strings.TrimSpace(value)
	}
	if value, ok := raw[`sampler`].(string); ok && len(strings.TrimSpace(value)) > 0 {
		opts.Sampler = jsutil.JSSlice(strings.TrimSpace(value), 80)
	}
	if value, ok := raw[`scheduler`].(string); ok && len(strings.TrimSpace(value)) > 0 {
		opts.Scheduler = jsutil.JSSlice(strings.TrimSpace(value), 80)
	}
	number := func(key string, fallback float64) float64 {
		if value, ok := raw[key].(float64); ok {
			return value
		}
		return fallback
	}
	steps := number(`steps`, float64(opts.Steps))
	cfg := number(`cfgScale`, opts.CFGScale)
	width := number(`width`, float64(opts.Width))
	height := number(`height`, float64(opts.Height))
	seed := number(`seed`, -1)
	if steps < 1 || steps > 150 || steps != float64(int(steps)) {
		return nil, fmt.Errorf(`Forge steps 必須是 1–150 的整數`)
	}
	if cfg <= 1 || cfg > 30 {
		return nil, fmt.Errorf(`Forge CFG 必須大於 1 且不超過 30，negative prompt 才會生效`)
	}
	if width < 256 || width > 2048 || width != float64(int(width)) || int(width)%8 != 0 {
		return nil, fmt.Errorf(`Forge 寬度必須是 256–2048 間且可被 8 整除`)
	}
	if height < 256 || height > 2048 || height != float64(int(height)) || int(height)%8 != 0 {
		return nil, fmt.Errorf(`Forge 高度必須是 256–2048 間且可被 8 整除`)
	}
	if seed < -1 || seed > 2147483647 || seed != float64(int64(seed)) {
		return nil, fmt.Errorf(`Forge seed 必須是 -1 或 0–2147483647 的整數`)
	}
	seedValue := int64(seed)
	opts.Steps, opts.CFGScale = int(steps), cfg
	opts.Width, opts.Height, opts.Seed = int(width), int(height), &seedValue
	return &opts, nil
}

// recordMemory writes this turn's player actions and DM narration into the
// story's memory log. Continuation turns (check resolution / combat conclusion)
// carry no new player declaration, so only the narration is recorded.
func (s *Server) recordMemory(storyID string, prepared game.PreparedDMTurn, output *dm.Turn) {
	round := prepared.Round
	if round < 1 {
		round = 1
	}
	var events []store.MemoryEvent
	if !prepared.IsContinuation {
		for _, p := range prepared.Players {
			if txt := strings.TrimSpace(prepared.Actions[p.ID]); txt != "" {
				events = append(events, store.MemoryEvent{Round: round, Role: "player", Text: p.Name + "：" + txt})
			}
		}
	}
	if txt := strings.TrimSpace(output.Narration); txt != "" {
		events = append(events, store.MemoryEvent{Round: round, Role: "dm", Text: txt})
	}
	_ = s.Memory.Record(storyID, events)
}

func (s *Server) handleSceneImage(w http.ResponseWriter, r *http.Request) {
	if err := s.imgGate.acquire(imageGateMinGap); err != nil {
		writeJSON(w, http.StatusTooManyRequests, errorBody{Error: err.Error()})
		return
	}
	defer s.imgGate.release()

	ctx, cancel := context.WithTimeout(r.Context(), 450*time.Second)
	defer cancel()

	body, err := readJSONBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	title := jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "campaign", "title"), "")), 180)
	scene := jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "campaign", "scene"), "")), 240)
	narration := jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "narration"), "")), 5000)
	// The DM agent's ready-made English SD prompt for this scene; when present
	// the image backend uses it directly instead of translating again.
	imagePrompt := jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "imagePrompt"), "")), 600)

	// Party visuals come from the server-authoritative sheets when the body
	// names a campaign; the legacy body players list stays as a fallback.
	var players []images.ScenePlayer
	if campaignID := sanitizeStoryID(jsutil.StrOr(jsutil.Get(body, "campaignId"), "")); campaignID != "" {
		if view, err := s.Game.View(campaignID); err == nil {
			for i, p := range view.Players {
				if i >= 4 {
					break
				}
				players = append(players, images.ScenePlayer{
					Name:       jsutil.JSSlice(p.Name, 100),
					ClassName:  jsutil.JSSlice(p.ClassName, 100),
					Species:    jsutil.JSSlice(p.Species, 100),
					Appearance: jsutil.JSSlice(p.Appearance, 500),
				})
			}
		}
	}
	if len(players) == 0 {
		if arr, ok := jsutil.AsSlice(jsutil.Get(body, "players")); ok {
			if len(arr) > 4 {
				arr = arr[:4]
			}
			for _, p := range arr {
				players = append(players, images.ScenePlayer{
					Name:       jsutil.JSSlice(jsutil.StrOr(jsutil.Get(p, "name"), "冒險者"), 100),
					ClassName:  jsutil.JSSlice(jsutil.StrOr(jsutil.Get(p, "className"), "旅人"), 100),
					Species:    jsutil.JSSlice(jsutil.StrOr(jsutil.Get(p, "species"), ""), 100),
					Appearance: jsutil.JSSlice(jsutil.StrOr(jsutil.Get(p, "appearance"), ""), 500),
				})
			}
		}
	}

	if title == "" || scene == "" || narration == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "需要戰役、場景與最近敘事才能生成插圖。"})
		return
	}

	renderer, err := s.imageRenderer(jsutil.StrOr(jsutil.Get(body, "imageBackend"), ""))
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	forgeOptions, err := parseForgeOptions(body, renderer)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	result, err := images.GenerateScene(ctx, renderer, s.Store, images.SceneInput{
		Title:       title,
		Scene:       scene,
		Narration:   narration,
		ImagePrompt: imagePrompt,
		Players:     players,
		Forge:       forgeOptions,
	})
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCharacterImage(w http.ResponseWriter, r *http.Request) {
	if err := s.imgGate.acquire(imageGateMinGap); err != nil {
		writeJSON(w, http.StatusTooManyRequests, errorBody{Error: err.Error()})
		return
	}
	defer s.imgGate.release()

	ctx, cancel := context.WithTimeout(r.Context(), 450*time.Second)
	defer cancel()

	body, err := readJSONBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	input := images.CharacterInput{
		Name:       jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "name"), "")), 100),
		Species:    jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "species"), "")), 80),
		ClassName:  jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "className"), "")), 100),
		Background: jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "background"), "")), 100),
		Appearance: jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "appearance"), "")), 1200),
	}
	if input.Name == "" || input.Appearance == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "需要角色名稱與外觀描述才能生成角色圖。"})
		return
	}

	renderer, err := s.imageRenderer(jsutil.StrOr(jsutil.Get(body, "imageBackend"), ""))
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	result, err := images.GenerateCharacter(ctx, renderer, s.Store, input)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleTTS synthesizes narration audio through the local GPT-SoVITS server
// and streams the clip back (audio/wav).
func (s *Server) handleTTS(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	body, err := readJSONBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	text := tts.PrepareText(jsutil.StrOr(jsutil.Get(body, "text"), ""))
	text = jsutil.JSSlice(text, 2000)
	if text == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "需要要朗讀的文字。"})
		return
	}
	if s.TTS == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "此伺服器未啟用語音朗讀。"})
		return
	}

	audio, mime, err := s.TTS.Synthesize(ctx, text)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("content-type", mime)
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write(audio)
}
