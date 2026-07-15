package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dndduet/internal/dm"
	"dndduet/internal/images"
	"dndduet/internal/jsutil"
	"dndduet/internal/provider"
	"dndduet/internal/tts"
)

type statusResponse struct {
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
	Text             string               `json:"text"`
	Scene            string               `json:"scene"`
	Objective        string               `json:"objective"`
	ObjectiveContext string               `json:"objectiveContext"`
	Stakes           string               `json:"stakes"`
	Choices          []string             `json:"choices"`
	RequiresCheck    bool                 `json:"requiresCheck"`
	Check            *dm.Check            `json:"check"`
	PrivateMessages  []dm.PrivateMessage  `json:"privateMessages"`
	Effects          []dm.Effect          `json:"effects"`
	Combat           dm.Combat            `json:"combat"`
	ActionIssues     []dm.ActionIssue     `json:"actionIssues"`
	ExperienceAwards []dm.ExperienceAward `json:"experienceAwards"`
	Model            string               `json:"model"`
}

func (s *Server) handleDm(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 210*time.Second)
	defer cancel()

	body, err := readJSONBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	prompt, _, err := dm.BuildDMRequest(body)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	// Match `typeof value === 'string' ? value.trim() : ''`: only a string
	// model value is honoured, anything else falls back to the default.
	modelInput, _ := body["model"].(string)
	selectedModel, err := s.Provider.NormalizeModel(modelInput)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	effortInput, _ := body["effort"].(string)
	selectedEffort, err := s.Provider.NormalizeEffort(effortInput)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	output, err := dm.RunDungeonMaster(ctx, s.Provider, prompt, selectedModel, selectedEffort, s.SchemaPath, s.ProviderCWD)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}

	checkText := ""
	if output.RequiresCheck && output.Check != nil {
		checkText = "\n\n檢定：" + output.Check.Character + " 進行 DC " + strconv.Itoa(output.Check.DC) + " 的" + output.Check.Ability + "（" + output.Check.Skill + "）檢定。" + output.Check.Reason
	}
	choiceText := ""
	if len(output.Choices) > 0 {
		choiceText = "\n\n可考慮：" + strings.Join(output.Choices, "／")
	}

	status := s.Provider.Status(ctx)
	model := selectedModel
	if model == "" {
		model = status.Model
	}

	writeJSON(w, http.StatusOK, dmResponse{
		Text:             output.Narration + checkText + choiceText,
		Scene:            output.Scene,
		Objective:        output.Objective,
		ObjectiveContext: output.ObjectiveContext,
		Stakes:           output.Stakes,
		Choices:          output.Choices,
		RequiresCheck:    output.RequiresCheck,
		Check:            output.Check,
		PrivateMessages:  output.PrivateMessages,
		Effects:          output.Effects,
		Combat:           output.Combat,
		ActionIssues:     output.ActionIssues,
		ExperienceAwards: output.ExperienceAwards,
		Model:            model,
	})
}

func (s *Server) handleSceneImage(w http.ResponseWriter, r *http.Request) {
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

	var players []images.ScenePlayer
	if arr, ok := jsutil.AsSlice(jsutil.Get(body, "players")); ok {
		if len(arr) > 4 {
			arr = arr[:4]
		}
		for _, p := range arr {
			players = append(players, images.ScenePlayer{
				Name:      jsutil.JSSlice(jsutil.StrOr(jsutil.Get(p, "name"), "冒險者"), 100),
				ClassName: jsutil.JSSlice(jsutil.StrOr(jsutil.Get(p, "className"), "旅人"), 100),
			})
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
	result, err := images.GenerateScene(ctx, renderer, s.Store, images.SceneInput{
		Title:     title,
		Scene:     scene,
		Narration: narration,
		Players:   players,
	})
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCharacterImage(w http.ResponseWriter, r *http.Request) {
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
