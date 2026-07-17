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
	"dndduet/internal/dm"
	"dndduet/internal/game"
	"dndduet/internal/images"
	"dndduet/internal/memory"
	"dndduet/internal/provider"
	"dndduet/internal/store"
	"dndduet/internal/tts"
)

const maxRequestBody = 1_000_000

// Server holds the backend dependencies.
type Server struct {
	// Provider is the default DM backend (also in Providers[DefaultDMProvider]).
	Provider provider.API
	// Providers maps dmProvider ids ("codex", "grok") for runtime UI switching.
	Providers map[string]provider.API
	// DefaultDMProvider is used when a request omits dmProvider (DM_PROVIDER env).
	DefaultDMProvider string

	Store       *store.Store
	WebDist     string // absolute path to the built frontend
	SchemaPath  string // absolute path to the DM output schema
	ProviderCWD string // absolute working directory for the CLI's --cd flag

	// TacticsSchemaPath is the combat-tactics output schema; empty disables
	// AI enemy turns (mechanical fallback targeting only).
	TacticsSchemaPath string

	// ImageRenderers maps a backend id ("codex", "local", "grok") to its renderer.
	ImageRenderers map[string]images.Renderer
	// DefaultImageBackend is used when a request omits imageBackend (IMAGE_BACKEND).
	DefaultImageBackend string

	// TTS reads DM narration aloud through a local GPT-SoVITS server; nil
	// disables the /api/tts endpoint.
	TTS *tts.Client

	// imgGate serialises image generation: at most one at a time, with a
	// minimum gap between runs, so a busy local GPU isn't flooded.
	imgGate imageGate

	// imgJobs tracks async scene-image jobs so the story flow never waits on
	// a long-lived generation request.
	imgJobs imageJobs

	// Memory persists per-story narrative memory and materialises the file the
	// DM turn's Codex reads; nil disables the memory pipeline (full-context mode).
	Memory *memory.Manager

	// Game orchestrates server-authoritative campaign state (characters,
	// combat, story journal) on top of Store.
	Game *game.Service

	// Prompt tracks what was already sent on the live Codex thread so subsequent
	// turns can omit the full system preamble. Reset on Connect. Nil disables
	// compaction (always full prompt). Only used for multi-turn thread providers.
	Prompt *dm.PromptSession
}

// pickDM resolves the DM provider for a request. Unknown ids fall back to default.
func (s *Server) pickDM(requested string) (id string, api provider.API) {
	id = strings.ToLower(strings.TrimSpace(requested))
	if id == "xai" {
		id = "grok"
	}
	if id == "" {
		id = strings.ToLower(strings.TrimSpace(s.DefaultDMProvider))
	}
	if id == "" {
		id = "codex"
	}
	if s.Providers != nil {
		if p, ok := s.Providers[id]; ok && p != nil {
			return id, p
		}
	}
	if s.Provider != nil {
		return id, s.Provider
	}
	for k, p := range s.Providers {
		if p != nil {
			return k, p
		}
	}
	return id, nil
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

// imageJob is one async scene-image generation.
type imageJob struct {
	Status  string // pending | done | error
	URL     string
	Prompt  string
	Model   string
	SlotID  string
	Err     string
	DoneAt  time.Time
	Created time.Time
}

// imageJobs is an in-memory job registry (single local process; jobs are
// throwaway — the finished image itself lands in image_meta / the campaign).
type imageJobs struct {
	mu   sync.Mutex
	jobs map[string]*imageJob
	seq  int64
}

func (j *imageJobs) create() (string, *imageJob) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.jobs == nil {
		j.jobs = map[string]*imageJob{}
	}
	j.seq++
	id := fmt.Sprintf("img-%d-%d", time.Now().UnixMilli(), j.seq)
	job := &imageJob{Status: "pending", Created: time.Now()}
	j.jobs[id] = job
	// Lazy purge: drop finished jobs older than 30 minutes.
	for k, v := range j.jobs {
		if v.Status != "pending" && time.Since(v.DoneAt) > 30*time.Minute {
			delete(j.jobs, k)
		}
	}
	return id, job
}

func (j *imageJobs) get(id string) (imageJob, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, ok := j.jobs[id]
	if !ok {
		return imageJob{}, false
	}
	return *job, true
}

func (j *imageJobs) finish(id string, apply func(*imageJob)) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if job, ok := j.jobs[id]; ok {
		apply(job)
		job.DoneAt = time.Now()
	}
}

// imageBackendOrder fixes the /api/status listing order.
var imageBackendOrder = []string{"codex", "grok", "local", "local2"}

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
	r.Get("/api/campaigns", s.handleCampaignList)
	r.Post("/api/campaigns", s.handleCampaignCreate)
	r.Post("/api/campaigns/import", s.handleCampaignImport)
	r.Get("/api/campaign/{id}", s.handleCampaignGet)
	r.Delete("/api/campaign/{id}", s.handleCampaignDelete)
	r.Get("/api/campaign/{id}/export", s.handleCampaignExport)
	r.Patch("/api/campaign/{id}/settings", s.handleCampaignSettings)
	r.Get("/api/rules/catalog", s.handleRulesCatalog)
	r.Post("/api/campaign/{id}/players/{pid}/cast", s.handleCast)
	r.Post("/api/campaign/{id}/players/{pid}/rest", s.handleRest)
	r.Post("/api/campaign/{id}/players/{pid}/revive", s.handleRevive)
	r.Get("/api/shop/catalog", s.handleShopCatalog)
	r.Post("/api/campaign/{id}/players/{pid}/buy", s.handleBuyItem)
	r.Post("/api/campaign/{id}/players/{pid}/sell", s.handleSellItem)
	r.Post("/api/campaign/{id}/players/{pid}/level-up", s.handleLevelUp)
	r.Post("/api/campaign/{id}/players/{pid}/ability-point", s.handleAbilityPoint)
	r.Post("/api/campaign/{id}/players/{pid}/prepared-spells", s.handlePreparedSpells)
	r.Post("/api/campaign/{id}/players/{pid}/resource", s.handleResource)
	r.Patch("/api/campaign/{id}/players/{pid}", s.handleCharacterPatch)
	r.Post("/api/campaign/{id}/players/{pid}/action", s.handleActionSubmit)
	r.Delete("/api/campaign/{id}/players/{pid}/action", s.handleActionUnlock)
	r.Post("/api/campaign/{id}/combat/start", s.handleCombatStart)
	r.Post("/api/campaign/{id}/combat/attack", s.handleCombatAttack)
	r.Post("/api/campaign/{id}/combat/end-turn", s.handleCombatEndTurn)
	r.Post("/api/campaign/{id}/combat/enemy-turn", s.handleCombatEnemyTurn)
	r.Post("/api/campaign/{id}/combat/conclude", s.handleCombatConclude)
	r.Post("/api/campaign/{id}/combat/retry", s.handleCombatRetry)
	r.Post("/api/dm", s.handleDm)
	r.Post("/api/scene-image", s.handleSceneImage)
	r.Get("/api/scene-image/job/{jobId}", s.handleSceneImageJob)
	r.Post("/api/character-image", s.handleCharacterImage)
	r.Delete("/api/generated/{filename}", s.handleDeleteGenerated)
	r.Get("/api/images/meta", s.handleListImageMeta)
	r.Post("/api/campaign/{id}/revise-story", s.handleReviseStory)
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
// Always logs the error so operators can fix issues from server.log without
// relying on the browser toast alone.
func writeErr(w http.ResponseWriter, err error, def int) {
	status := apperr.StatusOf(err, def)
	if err != nil {
		log.Printf("[api] error status=%d: %v", status, err)
	}
	writeJSON(w, status, errorBody{Error: err.Error()})
}

// logHandlerErr records a handler failure with route context and an optional
// fix hint (what to check / restart). Prefer this before writeErr when the
// surrounding request state matters for diagnosis.
func logHandlerErr(route string, err error, detail string) {
	if err == nil {
		return
	}
	if detail != "" {
		log.Printf("[%s] failed: %v | %s", route, err, detail)
		return
	}
	log.Printf("[%s] failed: %v", route, err)
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
