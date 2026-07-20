package httpapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"dndduet/internal/apperr"
	"dndduet/internal/game"
	"dndduet/internal/rules"
)

var playerIDPattern = regexp.MustCompile(`^player[1-4]$`)

// playerID extracts and validates the {pid} route param.
func playerID(r *http.Request) (string, error) {
	pid := strings.TrimSpace(chi.URLParam(r, "pid"))
	if !playerIDPattern.MatchString(pid) {
		return "", apperr.New(400, "缺少有效的角色 ID。")
	}
	return pid, nil
}

// decodeBody parses the request body into dst.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) error {
	data, err := readRawBody(w, r)
	if err != nil {
		return apperr.New(400, err.Error())
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return apperr.New(400, "資料格式錯誤。")
	}
	return nil
}

// characterAction wraps the common id/pid/body plumbing for player endpoints.
func (s *Server) characterAction(w http.ResponseWriter, r *http.Request, body any, run func(id, pid string) (game.View, error)) {
	id, err := campaignID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	pid, err := playerID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	if body != nil {
		if err := decodeBody(w, r, body); err != nil {
			writeErr(w, err, http.StatusBadRequest)
			return
		}
	}
	view, err := run(id, pid)
	writeView(w, view, err)
}

func (s *Server) handleCast(w http.ResponseWriter, r *http.Request) {
	id, err := campaignID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	pid, err := playerID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	var params game.CastParams
	if err := decodeBody(w, r, &params); err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	result, err := s.Game.CastSpell(id, pid, params)
	if err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type string `json:"type"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.Rest(id, pid, body.Type)
	})
}

// handleRevive: {pid} is the downed target; body names the rescuer.
func (s *Server) handleRevive(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RescuerID string `json:"rescuerId"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.Revive(id, body.RescuerID, pid)
	})
}

func (s *Server) handleShopCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": game.ShopCatalog})
}

func (s *Server) handleBuyItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemID string `json:"itemId"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.BuyItem(id, pid, body.ItemID)
	})
}

func (s *Server) handleSellItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemName string `json:"itemName"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.SellItem(id, pid, body.ItemName)
	})
}

func (s *Server) handleLevelUp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClassName string `json:"className"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.LevelUp(id, pid, body.ClassName)
	})
}

func (s *Server) handleAbilityPoint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ability string `json:"ability"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.SpendAbilityPointAction(id, pid, body.Ability)
	})
}

func (s *Server) handlePreparedSpells(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SpellIDs []string `json:"spellIds"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.SetPrepared(id, pid, body.SpellIDs)
	})
}

func (s *Server) handleResource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceID string `json:"resourceId"`
		Delta      int    `json:"delta"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.ChangeResourceAction(id, pid, body.ResourceID, body.Delta)
	})
}

func (s *Server) handleCharacterPatch(w http.ResponseWriter, r *http.Request) {
	var body game.CharacterPatch
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.UpdateCharacter(id, pid, body)
	})
}

func (s *Server) handleActionSubmit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	s.characterAction(w, r, &body, func(id, pid string) (game.View, error) {
		return s.Game.SubmitAction(id, pid, body.Text)
	})
}

func (s *Server) handleActionUnlock(w http.ResponseWriter, r *http.Request) {
	s.characterAction(w, r, nil, func(id, pid string) (game.View, error) {
		return s.Game.UnlockAction(id, pid)
	})
}

// rulesCatalog is built once: full spell objects plus class/ability labels for
// the character-building UI, replacing the frontend rules/ static data.
var rulesCatalog = func() []byte {
	spells := make([]rules.Spell, 0, len(rules.SpellIDs))
	for _, id := range rules.SpellIDs {
		if spell, err := rules.MakeSpell(id, rules.SpellOverrides{}); err == nil {
			spells = append(spells, spell)
		}
	}
	data, err := json.Marshal(map[string]any{
		"classNames":    rules.ClassNames,
		"abilityLabels": rules.AbilityLabels,
		"spells":        spells,
	})
	if err != nil {
		return []byte(`{}`)
	}
	return data
}()

func (s *Server) handleRulesCatalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("cache-control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(rulesCatalog)
}
