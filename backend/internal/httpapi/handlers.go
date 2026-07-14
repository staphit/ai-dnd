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
)

type statusResponse struct {
	Connected  bool                   `json:"connected"`
	Provider   string                 `json:"provider"`
	Model      string                 `json:"model"`
	Models     []provider.ModelOption `json:"models"`
	ImageModel string                 `json:"imageModel"`
	Message    string                 `json:"message,omitempty"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.Provider.Status(r.Context())
	writeJSON(w, http.StatusOK, statusResponse{
		Connected:  status.Configured,
		Provider:   status.Provider,
		Model:      status.Model,
		Models:     s.Provider.ModelOptions(),
		ImageModel: s.Provider.ImageModel(),
		Message:    status.Message,
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
	output, err := dm.RunDungeonMaster(ctx, s.Provider, prompt, selectedModel, s.SchemaPath, s.ProviderCWD)
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

	result, err := images.GenerateScene(ctx, s.Provider, s.Store, images.SceneInput{
		Title:     title,
		Scene:     scene,
		Narration: narration,
		Players:   players,
	}, s.ProviderCWD)
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

	result, err := images.GenerateCharacter(ctx, s.Provider, s.Store, input, s.ProviderCWD)
	if err != nil {
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
