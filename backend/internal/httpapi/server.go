// Package httpapi serves the D&D Duet backend: the /api endpoints, generated
// images from SQLite, and the built frontend as a single-page app. It mirrors
// server.mjs.
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"dndduet/internal/apperr"
	"dndduet/internal/provider"
	"dndduet/internal/store"
)

const maxRequestBody = 1_000_000

// Server holds the backend dependencies.
type Server struct {
	Provider    provider.API
	Store       *store.Store
	WebDist     string // absolute path to the built frontend
	SchemaPath  string // absolute path to the DM output schema
	ProviderCWD string // absolute working directory for the CLI's --cd flag
}

// Router wires the routes, falling through to the SPA for anything unmatched so
// the behaviour matches the original method+URL dispatch.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/status", s.handleStatus)
	r.Post("/api/dm", s.handleDm)
	r.Post("/api/scene-image", s.handleSceneImage)
	r.Post("/api/character-image", s.handleCharacterImage)
	r.Get("/generated/*", s.serveGenerated)
	r.NotFound(s.serveStatic)
	r.MethodNotAllowed(s.serveStatic)
	return r
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("cache-control", "no-store")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	// json.Encoder appends a trailing newline; the original JSON.stringify did
	// not, so trim it to keep responses byte-identical.
	w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
}

type errorBody struct {
	Error string `json:"error"`
}

// writeErr writes {"error": ...} with the status carried by err (default def).
func writeErr(w http.ResponseWriter, err error, def int) {
	writeJSON(w, apperr.StatusOf(err, def), errorBody{Error: err.Error()})
}

// readJSONBody reads and parses the request body as a JSON object, enforcing the
// 1 MB limit from the original readJson helper.
func readJSONBody(w http.ResponseWriter, r *http.Request) (map[string]any, error) {
	limited := http.MaxBytesReader(w, r.Body, maxRequestBody)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, errors.New("Request body is too large")
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return map[string]any{}, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, err
	}
	// Node did JSON.parse(raw || '{}') and then read body?.field. A non-object
	// body (array/number/string) has no fields, so downstream reads all see
	// undefined — reproduce that by returning an empty object instead of
	// erroring, so e.g. /api/dm still returns 400 (empty party) rather than 503.
	if m, ok := parsed.(map[string]any); ok {
		return m, nil
	}
	return map[string]any{}, nil
}
