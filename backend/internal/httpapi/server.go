// Package httpapi serves the D&D Duet backend: the /api endpoints, generated
// images from SQLite, and the built frontend as a single-page app. It mirrors
// server.mjs.
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"dndduet/internal/apperr"
	"dndduet/internal/images"
	"dndduet/internal/memory"
	"dndduet/internal/provider"
	"dndduet/internal/store"
	"dndduet/internal/tts"
)

const maxRequestBody = 1_000_000

// Server holds the backend dependencies.
type Server struct {
	Provider    provider.API
	Store       *store.Store
	WebDist     string // absolute path to the built frontend
	SchemaPath  string // absolute path to the DM output schema
	ProviderCWD string // absolute working directory for the CLI's --cd flag

	// ImageRenderers maps a backend id ("codex", "local") to its renderer; the
	// request body's imageBackend field picks one per generation.
	ImageRenderers map[string]images.Renderer
	// DefaultImageBackend is used when a request omits imageBackend (IMAGE_BACKEND).
	DefaultImageBackend string

	// TTS reads DM narration aloud through a local GPT-SoVITS server; nil
	// disables the /api/tts endpoint.
	TTS *tts.Client

	// imgGate serialises image generation: at most one at a time, with a
	// minimum gap between runs, so a busy local GPU isn't flooded.
	imgGate imageGate

	// Memory persists per-story narrative memory and materialises the file the
	// DM turn's Codex reads; nil disables the memory pipeline (full-context mode).
	Memory *memory.Manager
}

// imageGateMinGap is the minimum spacing between image generations.
const imageGateMinGap = 5 * time.Second

// imageGate serialises image generation across requests.
type imageGate struct {
	mu       sync.Mutex
	busy     bool
	lastDone time.Time
}

// acquire reserves the single image slot or returns a user-facing error when
// one is already running or the last one finished less than minGap ago.
func (g *imageGate) acquire(minGap time.Duration) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.busy {
		return errors.New("目前已有一張圖片正在生成，請等它完成再試。")
	}
	if wait := minGap - time.Since(g.lastDone); wait > 0 {
		return fmt.Errorf("圖片生成間隔太短，請於 %.0f 秒後再試。", wait.Seconds()+0.5)
	}
	g.busy = true
	return nil
}

// release frees the slot and records the completion time.
func (g *imageGate) release() {
	g.mu.Lock()
	g.busy = false
	g.lastDone = time.Now()
	g.mu.Unlock()
}

// imageBackendOrder fixes the /api/status listing order.
var imageBackendOrder = []string{"codex", "local", "local2"}

// imageBackendOptions lists the configured image backends for /api/status.
func (s *Server) imageBackendOptions() []provider.ModelOption {
	var opts []provider.ModelOption
	for _, id := range imageBackendOrder {
		if r, ok := s.ImageRenderers[id]; ok {
			opts = append(opts, provider.ModelOption{ID: id, Label: r.Model()})
		}
	}
	return opts
}

// forgeDefaults exposes presets only for local renderers. External image
// providers never receive or advertise these parameters.
func (s *Server) forgeDefaults() map[string]images.ForgeOptions {
	defaults := make(map[string]images.ForgeOptions)
	for id, renderer := range s.ImageRenderers {
		if configurable, ok := renderer.(interface{ SceneDefaults() images.ForgeOptions }); ok {
			defaults[id] = configurable.SceneDefaults()
		}
	}
	return defaults
}

// imageRenderer resolves the requested backend id, falling back to the default.
func (s *Server) imageRenderer(requested string) (images.Renderer, error) {
	id := strings.TrimSpace(requested)
	if id == "" {
		id = s.DefaultImageBackend
	}
	if id == "" {
		id = "codex"
	}
	if r, ok := s.ImageRenderers[id]; ok {
		return r, nil
	}
	return nil, errors.New("不支援的圖片後端選項")
}

// Router wires the routes, falling through to the SPA for anything unmatched so
// the behaviour matches the original method+URL dispatch.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(requestLogger)
	r.Get("/api/status", s.handleStatus)
	r.Get("/api/codex/connection", s.handleCodexConnection)
	r.Post("/api/codex/connect", s.handleCodexConnect)
	r.Post("/api/dm", s.handleDm)
	r.Post("/api/scene-image", s.handleSceneImage)
	r.Post("/api/character-image", s.handleCharacterImage)
	r.Post("/api/tts", s.handleTTS)
	r.Get("/generated/*", s.serveGenerated)
	r.NotFound(s.serveStatic)
	r.MethodNotAllowed(s.serveStatic)
	return r
}

// statusRecorder captures the response status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// requestLogger logs one line per HTTP request (method, path, status,
// duration) via the standard logger, which the server tees to the log file.
// Static asset noise is skipped; API and generated-image routes are kept.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/generated/") {
			log.Printf("[http] %s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
		}
	})
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
