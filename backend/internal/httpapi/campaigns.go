package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"dndduet/internal/apperr"
	"dndduet/internal/game"
)

// readRawBody reads the request body up to the standard limit without parsing.
func readRawBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	limited := http.MaxBytesReader(w, r.Body, maxRequestBody)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, errors.New("Request body is too large")
	}
	return data, nil
}

// campaignID extracts and validates the {id} route param using the same
// pattern as the memory storyID.
func campaignID(r *http.Request) (string, error) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if sanitizeStoryID(id) == "" {
		return "", apperr.New(400, "缺少有效的戰役 ID。")
	}
	return id, nil
}

// writeView writes a game.View (or a service error).
func writeView(w http.ResponseWriter, view game.View, err error) {
	if err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleCampaignList(w http.ResponseWriter, r *http.Request) {
	list, err := s.Game.List()
	if err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []game.Summary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"campaigns": list})
}

func (s *Server) handleCampaignCreate(w http.ResponseWriter, r *http.Request) {
	data, err := readRawBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	var params game.CreateParams
	if err := json.Unmarshal(data, &params); err != nil {
		writeErr(w, apperr.New(400, "建立戰役的資料格式錯誤。"), http.StatusBadRequest)
		return
	}
	view, err := s.Game.Create(params)
	writeView(w, view, err)
}

func (s *Server) handleCampaignGet(w http.ResponseWriter, r *http.Request) {
	id, err := campaignID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	view, err := s.Game.View(id)
	writeView(w, view, err)
}

func (s *Server) handleCampaignDelete(w http.ResponseWriter, r *http.Request) {
	id, err := campaignID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	if err := s.Game.Delete(id); err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCampaignImport(w http.ResponseWriter, r *http.Request) {
	data, err := readRawBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	overwrite := r.URL.Query().Get("overwrite") == "true"
	view, err := s.Game.Import(data, overwrite)
	writeView(w, view, err)
}

func (s *Server) handleCampaignExport(w http.ResponseWriter, r *http.Request) {
	id, err := campaignID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	data, err := s.Game.Export(id)
	if err != nil {
		writeErr(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("content-disposition", `attachment; filename="`+id+`.json"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handleCampaignSettings(w http.ResponseWriter, r *http.Request) {
	id, err := campaignID(r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	data, err := readRawBody(w, r)
	if err != nil {
		writeErr(w, err, http.StatusBadRequest)
		return
	}
	view, err := s.Game.UpdateSettings(id, data)
	writeView(w, view, err)
}
