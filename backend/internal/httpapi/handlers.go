package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

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
	Alive      bool   `json:"alive"`
	StoryId    string `json:"storyId"`
	ImageAlive bool   `json:"imageAlive"` // dedicated GPT image connection (max 2 per story)
	DmProvider string `json:"dmProvider,omitempty"`
}

type needsConsentBody struct {
	Error        string `json:"error"`
	NeedsConsent bool   `json:"needsConsent"`
}

// imageConnectionReporter is implemented by providers that own a second
// app-server process for GPT image generation (story + image ≤ 2).
type imageConnectionReporter interface {
	ImageConnectionState() provider.ConnState
}

func connectionSnapshot(p provider.API) connectionBody {
	cs := p.ConnectionState()
	body := connectionBody{Alive: cs.Alive, StoryId: cs.StoryID}
	if reporter, ok := p.(imageConnectionReporter); ok {
		ics := reporter.ImageConnectionState()
		body.ImageAlive = ics.Alive && ics.StoryID != "" && ics.StoryID == cs.StoryID
	}
	return body
}

// handleCodexConnection reports the current persistent-connection binding.
func (s *Server) handleCodexConnection(w http.ResponseWriter, r *http.Request) {
	id, api := s.pickDM(r.URL.Query().Get("dmProvider"))
	if api == nil {
		writeJSON(w, http.StatusOK, connectionBody{DmProvider: id})
		return
	}
	snap := connectionSnapshot(api)
	snap.DmProvider = id
	writeJSON(w, http.StatusOK, snap)
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
	dmID, api := s.pickDM(jsutil.StrOr(jsutil.Get(body, "dmProvider"), ""))
	if api == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "沒有可用的 DM 資料源。"})
		return
	}
	if err := api.Connect(ctx, storyID); err != nil {
		logHandlerErr("codex/connect", err, "story="+storyID+" dm="+dmID+" | tip: Codex→codex login；Grok→grok login 或 XAI_API_KEY")
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	// Fresh multi-turn session: next DM turn must re-send full rules.
	if s.Prompt != nil {
		s.Prompt.Reset(storyID)
	}
	snap := connectionSnapshot(api)
	snap.DmProvider = dmID
	log.Printf("[codex/connect] ok story=%s dm=%s imageAlive=%v", snap.StoryId, dmID, snap.ImageAlive)
	writeJSON(w, http.StatusOK, snap)
}

// dmProviderStatus is one selectable storyteller backend for the settings UI.
type dmProviderStatus struct {
	ID        string                 `json:"id"`
	Label     string                 `json:"label"`
	Connected bool                   `json:"connected"`
	Model     string                 `json:"model"`
	Models    []provider.ModelOption `json:"models"`
	Efforts   []provider.ModelOption `json:"efforts"`
	Message   string                 `json:"message,omitempty"`
}

type statusResponse struct {
	ForgeDefaults map[string]images.ForgeOptions `json:"ForgeDefaults,omitempty"`
	Connected     bool                           `json:"connected"`
	Provider      string                         `json:"provider"`
	Model         string                         `json:"model"`
	Models        []provider.ModelOption         `json:"models"`
	Efforts       []provider.ModelOption         `json:"efforts"`
	ImageModel    string                         `json:"imageModel"`
	ImageBackends []provider.ModelOption         `json:"imageBackends"`
	ImageBackend  string                         `json:"imageBackend"`
	Message       string                         `json:"message,omitempty"`
	DmProvider    string                         `json:"dmProvider"`
	DmProviders   []dmProviderStatus             `json:"dmProviders"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	reqProvider := r.URL.Query().Get("dmProvider")
	dmID, api := s.pickDM(reqProvider)
	if api == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "沒有可用的 DM 資料源。"})
		return
	}
	status := api.Status(r.Context())
	imageModel := api.ImageModel()
	defaultBackend := s.DefaultImageBackend
	if defaultBackend == "" {
		defaultBackend = "codex"
	}
	if renderer, err := s.imageRenderer(""); err == nil {
		imageModel = renderer.Model()
	}

	var dmProviders []dmProviderStatus
	order := []string{"codex", "grok"}
	seen := map[string]bool{}
	for _, id := range order {
		p, ok := s.Providers[id]
		if !ok || p == nil {
			continue
		}
		seen[id] = true
		st := p.Status(r.Context())
		label := st.Provider
		if label == "" {
			label = id
		}
		dmProviders = append(dmProviders, dmProviderStatus{
			ID: id, Label: label, Connected: st.Configured, Model: st.Model,
			Models: p.ModelOptions(), Efforts: p.EffortOptions(), Message: st.Message,
		})
	}
	for id, p := range s.Providers {
		if seen[id] || p == nil {
			continue
		}
		st := p.Status(r.Context())
		dmProviders = append(dmProviders, dmProviderStatus{
			ID: id, Label: st.Provider, Connected: st.Configured, Model: st.Model,
			Models: p.ModelOptions(), Efforts: p.EffortOptions(), Message: st.Message,
		})
	}

	writeJSON(w, http.StatusOK, statusResponse{
		ForgeDefaults: s.forgeDefaults(),
		Connected:     status.Configured,
		Provider:      status.Provider,
		Model:         status.Model,
		Models:        api.ModelOptions(),
		Efforts:       api.EffortOptions(),
		ImageModel:    imageModel,
		ImageBackends: s.imageBackendOptions(),
		ImageBackend:  defaultBackend,
		Message:       status.Message,
		DmProvider:    dmID,
		DmProviders:   dmProviders,
	})
}

// sceneSlotPayload is the client-facing scene-image placeholder returned with each DM turn.
type sceneSlotPayload struct {
	ID          string `json:"id"`
	Scene       string `json:"scene"`
	ImagePrompt string `json:"imagePrompt"`
	CreatedAt   int64  `json:"createdAt"`
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
	SceneSlot       *sceneSlotPayload   `json:"sceneSlot,omitempty"`
}

// dmRequest is the slim /api/dm body: everything else (characters, combat,
// history, campaign meta) now comes from the server's own store.
type dmRequest struct {
	CampaignID  string `json:"campaignId"`
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	DmProvider  string `json:"dmProvider"`
	Actions     []struct {
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

	dmID, api := s.pickDM(req.DmProvider)
	if api == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "沒有可用的 DM 資料源。"})
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

	// Codex app-server with a live thread: materialise memory + rules files for
	// delta mode. Grok / unbound / exec: inject rendered memory into the prompt
	// body (they cannot read sandbox files).
	cs := api.ConnectionState()
	threadAlive := cs.Alive && cs.StoryID == storyID
	useFileDelta := dmID == "codex" && threadAlive && s.Memory != nil
	memoryInline := ""
	if s.Memory != nil {
		if useFileDelta {
			if err := s.Memory.Materialise(storyID); err == nil {
				prepared.Input.DeltaMode = true
				prepared.Input.MemRef = s.Memory.Ref(storyID)
				if dossier, err := s.Game.BuildRulesDossier(storyID); err == nil {
					if err := s.Memory.MaterialiseRules(storyID, dossier); err == nil {
						prepared.Input.RulesRef = s.Memory.RulesRef(storyID)
					}
				}
			} else {
				log.Printf("[dm] memory materialise failed story=%s: %v", storyID, err)
			}
		} else {
			if text, rerr := s.Memory.Render(storyID); rerr != nil {
				log.Printf("[dm] memory render failed story=%s dm=%s: %v", storyID, dmID, rerr)
			} else if strings.TrimSpace(text) != "" {
				memoryInline = text
			}
		}
	}

	// Prompt-session: after Connect, first turn sends full rules; later turns
	// use compact rules (Grok multi-turn / Codex thread) or mini rules-file
	// (Codex delta). Avoids re-paying the full system preamble every request.
	plan := dm.Plan{FullRules: true}
	if s.Prompt != nil && threadAlive {
		plan = s.Prompt.Plan(storyID, threadAlive)
	}

	turnBody := dm.BuildDMRequestV2(prepared.Input)
	if memoryInline != "" && !prepared.Input.DeltaMode {
		turnBody = "前情提要（由伺服器注入，只讀）：\n" + memoryInline + "\n\n" + turnBody
	}

	selectedModel, err := api.NormalizeModel(req.Model)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	selectedEffort, err := api.NormalizeEffort(req.Effort)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}

	// Choose rules block:
	//  - Codex delta + rules file → mini (points at dossier)
	//  - Session already has full rules → compact (short reminders)
	//  - Otherwise → full preamble
	rulesMode := dm.RulesFull
	switch {
	case prepared.Input.DeltaMode && prepared.Input.RulesRef != "":
		rulesMode = dm.RulesMini
	case threadAlive && s.Prompt != nil && !plan.FullRules:
		rulesMode = dm.RulesCompact
	}

	output, err := dm.RunDungeonMaster(ctx, api, turnBody, selectedModel, selectedEffort, s.SchemaPath, s.ProviderCWD, storyID, rulesMode)
	if err != nil {
		detail := fmt.Sprintf("story=%s dm=%s model=%q effort=%q rules=%v delta=%v",
			storyID, dmID, selectedModel, selectedEffort, rulesMode, prepared.Input.DeltaMode)
		if errors.Is(err, provider.ErrNeedsConsent) {
			logHandlerErr("dm", err, detail+" | tip: POST /api/codex/connect 綁定本故事")
			writeJSON(w, http.StatusConflict, needsConsentBody{Error: err.Error(), NeedsConsent: true})
			return
		}
		logHandlerErr("dm", err, detail+" | tip: Codex/Grok 是否逾時/斷線；LOG_PROMPTS=1 可看完整 prompt")
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}

	if s.Prompt != nil && threadAlive {
		fullRules := rulesMode == dm.RulesFull
		s.Prompt.Commit(storyID, &dm.TurnSnapshot{
			Title: prepared.Input.Title, Scene: prepared.Input.Scene,
			Objective: prepared.Input.Objective, Stakes: prepared.Input.Stakes,
		}, fullRules, true)
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

	status := api.Status(ctx)
	model := selectedModel
	if model == "" {
		model = status.Model
	}

	publicText := FormatDialogueBreaks(strings.TrimSpace(output.Narration)) + checkText
	if len(applied.Rejected) > 0 {
		publicText = ""
	}
	privateMsgs := make([]dm.PrivateMessage, 0, len(output.PrivateMessages))
	for _, m := range output.PrivateMessages {
		privateMsgs = append(privateMsgs, dm.PrivateMessage{
			PlayerID: m.PlayerID,
			Text:     FormatDialogueBreaks(strings.TrimSpace(m.Text)),
		})
	}

	// Persist a scene-image placeholder so the player can generate this beat later.
	var slotPayload *sceneSlotPayload
	if s.Store != nil && strings.TrimSpace(output.Narration) != "" && len(applied.Rejected) == 0 {
		slotID := "slot-" + strconv.FormatInt(time.Now().UnixMilli(), 10) + "-" + randomHex(4)
		playersJSON := "[]"
		if rawPlayers, merr := json.Marshal(prepared.Players); merr == nil {
			playersJSON = string(rawPlayers)
		}
		sceneName := strings.TrimSpace(output.Scene)
		if sceneName == "" {
			sceneName = prepared.Input.Scene
		}
		slot := store.SceneSlot{
			ID: slotID, StoryID: storyID, Scene: sceneName,
			Title: prepared.Input.Title, Narration: publicText,
			ImagePrompt: strings.TrimSpace(output.ImagePrompt),
			PlayersJSON: playersJSON, CreatedAt: time.Now().UnixMilli(),
		}
		if err := s.Store.SaveSceneSlot(slot); err != nil {
			log.Printf("[dm] save scene slot failed story=%s: %v", storyID, err)
		} else {
			slotPayload = &sceneSlotPayload{
				ID: slot.ID, Scene: slot.Scene, ImagePrompt: slot.ImagePrompt, CreatedAt: slot.CreatedAt,
			}
		}
	}

	writeJSON(w, http.StatusOK, dmResponse{
		View:            &applied.View,
		Text:            publicText,
		Choices:         output.Choices,
		RequiresCheck:   output.RequiresCheck,
		Check:           output.Check,
		PrivateMessages: privateMsgs,
		ActionIssues:    applied.Rejected,
		Model:           model,
		SceneSlot:       slotPayload,
	})
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// FormatDialogueBreaks inserts newlines around 「」 dialogue so speech is readable.
var (
	dialogueOpenRe  = regexp.MustCompile(`([^\n「])(「)`)
	dialogueCloseRe = regexp.MustCompile(`(」[。！？…—]*)([^\n」])`)
	speechTagRe     = regexp.MustCompile(`([。！？])([\p{Han}]{1,12}(?:低聲|輕聲|喃喃|碎念|嘀咕|低語|咕噥|說道|問道|喝道|怒道|笑道|喊道|接著說|繼續說)[：「])`)
)

func FormatDialogueBreaks(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	s = dialogueOpenRe.ReplaceAllString(s, "$1\n$2")
	s = dialogueCloseRe.ReplaceAllString(s, "$1\n$2")
	s = speechTagRe.ReplaceAllString(s, "$1\n$2")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
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

	// Prefer a server-side scene slot captured at DM-turn time when provided.
	slotID := strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "sceneSlotId"), ""))
	title := jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "campaign", "title"), "")), 180)
	scene := jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "campaign", "scene"), "")), 240)
	narration := jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "narration"), "")), 5000)
	imagePrompt := jsutil.JSSlice(strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "imagePrompt"), "")), 600)
	campaignID := sanitizeStoryID(jsutil.StrOr(jsutil.Get(body, "campaignId"), ""))

	if slotID != "" && s.Store != nil {
		if slot, ok, gerr := s.Store.GetSceneSlot(slotID); gerr != nil {
			logHandlerErr("scene-image", gerr, "sceneSlotId="+slotID)
			writeErr(w, gerr, http.StatusServiceUnavailable)
			return
		} else if ok {
			if title == "" {
				title = jsutil.JSSlice(slot.Title, 180)
			}
			if scene == "" {
				scene = jsutil.JSSlice(slot.Scene, 240)
			}
			if narration == "" {
				narration = jsutil.JSSlice(slot.Narration, 5000)
			}
			if imagePrompt == "" {
				imagePrompt = jsutil.JSSlice(slot.ImagePrompt, 600)
			}
			if campaignID == "" {
				campaignID = sanitizeStoryID(slot.StoryID)
			}
		}
	}

	// Party visuals come from the server-authoritative sheets when the body
	// names a campaign; the legacy body players list stays as a fallback.
	var players []images.ScenePlayer
	if campaignID != "" {
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

	backendReq := jsutil.StrOr(jsutil.Get(body, "imageBackend"), "")
	renderer, err := s.imageRenderer(backendReq)
	if err != nil {
		logHandlerErr("scene-image", err, "backend="+backendReq+" default="+s.DefaultImageBackend)
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	forgeOptions, err := parseForgeOptions(body, renderer)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	result, err := images.GenerateScene(ctx, renderer, s.Store, images.SceneInput{
		Title:        title,
		Scene:        scene,
		Narration:    narration,
		ImagePrompt:  imagePrompt,
		Players:      players,
		Forge:        forgeOptions,
		CampaignID:   campaignID,
		SourceSlotID: slotID,
	})
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	if slotID != "" && s.Store != nil {
		if err := s.Store.BindSceneSlotImage(slotID, result.URL, result.Model); err != nil {
			log.Printf("[scene-image] bind slot image failed id=%s: %v", slotID, err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url": result.URL, "prompt": result.Prompt, "model": result.Model, "sceneSlotId": slotID,
	})
}

func (s *Server) handleListImageMeta(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"images": []any{}})
		return
	}
	campaignID := strings.TrimSpace(r.URL.Query().Get("campaignId"))
	limit := 100
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	list, err := s.Store.ListImageMeta(campaignID, limit)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		out = append(out, map[string]any{
			"filename": m.Filename, "campaignId": m.CampaignID, "scene": m.Scene,
			"prompt": m.Prompt, "model": m.Model, "sourceSlotId": m.SourceSlotID,
			"createdAt": m.CreatedAt, "url": "/generated/" + m.Filename,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": out})
}

// handleReviseStory rewrites the last public DM narration from a player note
// without advancing the round or re-validating actions.
func (s *Server) handleReviseStory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 210*time.Second)
	defer cancel()

	storyID := sanitizeStoryID(chi.URLParam(r, "id"))
	if storyID == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "缺少有效的 campaignId。"})
		return
	}
	body, err := readJSONBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	note := strings.TrimSpace(jsutil.StrOr(jsutil.Get(body, "note"), ""))
	if note == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "需要修正說明（note）。"})
		return
	}
	dmID, api := s.pickDM(jsutil.StrOr(jsutil.Get(body, "dmProvider"), ""))
	if api == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "沒有可用的 DM 資料源。"})
		return
	}

	view, err := s.Game.View(storyID)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	lastDM := ""
	for i := len(view.Story) - 1; i >= 0; i-- {
		e := view.Story[i]
		if e.Speaker == "dm" && (e.Audience == "" || e.Audience == "public") {
			lastDM = e.Text
			break
		}
	}
	if strings.TrimSpace(lastDM) == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "尚無可重寫的 DM 敘事。"})
		return
	}

	selectedModel, err := api.NormalizeModel(jsutil.StrOr(jsutil.Get(body, "model"), ""))
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	selectedEffort, err := api.NormalizeEffort(jsutil.StrOr(jsutil.Get(body, "effort"), ""))
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}

	prompt := strings.Join([]string{
		"規則版本：2024 第五版／SRD 5.2.1。這是敘事修正回合，不是新的玩家行動。",
		"戰役：" + view.Title,
		"場景：" + view.Scene,
		"目前目標：" + view.Objective,
		"",
		"上一則公開 DM 敘事草稿：",
		lastDM,
		"",
		"玩家修正要求：",
		note,
		"",
		"請只重寫 narration（繁體中文公開敘事），修正事實錯誤、語氣或遺漏；不可推進新場景、不可開始戰鬥、不可要求新檢定、不可發放 XP 或套用 effects。actionIssues、choices 可為空。combat.starts 必須為 false。",
	}, "\n")

	// Revision keeps mechanical state; prefer compact rules when session is warm.
	revRules := dm.RulesFull
	if s.Prompt != nil {
		if p := s.Prompt.Plan(storyID, true); !p.FullRules {
			revRules = dm.RulesCompact
		}
	}
	output, err := dm.RunDungeonMaster(ctx, api, prompt, selectedModel, selectedEffort, s.SchemaPath, s.ProviderCWD, storyID, revRules)
	if err != nil {
		if errors.Is(err, provider.ErrNeedsConsent) {
			writeJSON(w, http.StatusConflict, needsConsentBody{Error: err.Error(), NeedsConsent: true})
			return
		}
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	newText := FormatDialogueBreaks(strings.TrimSpace(output.Narration))
	if newText == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "DM 未回傳可用敘事。"})
		return
	}
	if err := s.Game.ReplaceLastPublicDM(storyID, newText); err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	updated, err := s.Game.View(storyID)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	_ = dmID
	writeJSON(w, http.StatusOK, map[string]any{"view": updated, "text": newText, "model": selectedModel})
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
