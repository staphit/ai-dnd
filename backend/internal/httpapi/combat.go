package httpapi

import (
	"context"
	"net/http"
	"time"

	"dndduet/internal/dm"
	"dndduet/internal/game"
)

// combatAction wraps the id/body plumbing for combat endpoints that return a
// custom payload.
func combatCampaignID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := campaignID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return "", false
	}
	return id, true
}

func (s *Server) handleCombatStart(w http.ResponseWriter, r *http.Request) {
	id, ok := combatCampaignID(w, r)
	if !ok {
		return
	}
	var body struct {
		Enemies []game.EnemySpec `json:"enemies"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	view, err := s.Game.StartCombatManual(id, body.Enemies)
	writeView(w, view, err)
}

func (s *Server) handleCombatAttack(w http.ResponseWriter, r *http.Request) {
	id, ok := combatCampaignID(w, r)
	if !ok {
		return
	}
	var body game.AttackParams
	if err := decodeBody(w, r, &body); err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	result, err := s.Game.Attack(id, body)
	if err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCombatEndTurn(w http.ResponseWriter, r *http.Request) {
	id, ok := combatCampaignID(w, r)
	if !ok {
		return
	}
	result, err := s.Game.EndTurn(id)
	if err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCombatEnemyTurn(w http.ResponseWriter, r *http.Request) {
	id, ok := combatCampaignID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	var runner game.TacticsRunner
	if s.TacticsSchemaPath != "" {
		runner = func(ctx context.Context, input dm.TacticsInput) (dm.Tactic, error) {
			return dm.RunCombatTactics(ctx, s.Provider, input, s.TacticsSchemaPath, s.ProviderCWD, id)
		}
	}
	result, err := s.Game.EnemyTurn(ctx, id, runner)
	if err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCombatRetry(w http.ResponseWriter, r *http.Request) {
	id, ok := combatCampaignID(w, r)
	if !ok {
		return
	}
	view, err := s.Game.RetryCombat(id)
	writeView(w, view, err)
}

func (s *Server) handleCombatConclude(w http.ResponseWriter, r *http.Request) {
	id, ok := combatCampaignID(w, r)
	if !ok {
		return
	}
	result, err := s.Game.Conclude(id)
	if err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
