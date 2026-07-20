// Command server runs the D&D Duet local backend: the /api endpoints, generated
// images from SQLite, and the built frontend. It replaces server.mjs.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dndduet/internal/applog"
	"dndduet/internal/codex"
	"dndduet/internal/dm"
	"dndduet/internal/game"
	"dndduet/internal/grok"
	"dndduet/internal/httpapi"
	"dndduet/internal/images"
	"dndduet/internal/memory"
	"dndduet/internal/provider"
	"dndduet/internal/store"
	schema "dndduet/schemas"
)

func main() {
	repoRoot := resolveRepoRoot()
	loadEnvFile(filepath.Join(repoRoot, ".env"))
	loadEnvFile(filepath.Join(repoRoot, "backend", ".env"))

	port := envOr("PORT", "4318")

	// Tee all log output to a dated file as well as the console, so prompt logs
	// (LOG_PROMPTS) and errors survive after the window closes. Files are split
	// by local calendar day: logs/server-2006-01-02.log. LOG_FILE overrides the
	// base path (date is still inserted before the extension); empty disables
	// file logging.
	logBase := envOr("LOG_FILE", filepath.Join(repoRoot, "logs", "server.log"))
	if logBase != "" {
		daily := applog.NewDailyWriter(logBase)
		log.SetOutput(io.MultiWriter(os.Stderr, daily))
		log.Printf("logging to %s (daily rotate)", applog.PathFor(logBase, time.Now()))
	}

	webDist := absOr(envOr("WEB_DIST", filepath.Join(repoRoot, "web-dist")))
	dataDir := absOr(envOr("DND_DATA_DIR", filepath.Join(repoRoot, "campaign-data")))
	codexCWD := absOr(envOr("CODEX_CWD", repoRoot))

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("cannot create data directory %s: %v", dataDir, err)
	}
	schemaPath, err := schema.WriteTempFile()
	if err != nil {
		log.Fatalf("cannot materialise DM schema: %v", err)
	}
	imagePromptSchemaPath, err := schema.WriteImagePromptTempFile()
	if err != nil {
		log.Fatalf("cannot materialise image-prompt schema: %v", err)
	}
	tacticsSchemaPath, err := schema.WriteCombatTacticsTempFile()
	if err != nil {
		log.Fatalf("cannot materialise combat-tactics schema: %v", err)
	}
	novelSchemaPath, err := schema.WriteNovelExportTempFile()
	if err != nil {
		log.Fatalf("cannot materialise novel-export schema: %v", err)
	}

	// Persisted scene/portrait art lives under generated-images/ (DND_IMAGE_DIR overrides).
	imageDir := absOr(envOr("DND_IMAGE_DIR", filepath.Join(repoRoot, "generated-images")))
	dbPath := filepath.Join(dataDir, "dnd-duet.db")
	db, err := store.Open(dbPath, imageDir)
	if err != nil {
		log.Fatalf("cannot open database: %v", err)
	}
	defer db.Close()
	log.Printf("database: %s", dbPath)
	log.Printf("image storage: %s", imageDir)

	// Default DM backend (UI can switch at runtime among registered providers).
	//   codex — local Codex CLI (ChatGPT login)
	//   grok  — Grok Build CLI (`grok login`) or XAI_API_KEY HTTP
	defaultDM := strings.ToLower(envOr("DM_PROVIDER", "codex"))
	if defaultDM == "xai" {
		defaultDM = "grok"
	}
	if defaultDM == "" {
		defaultDM = "codex"
	}

	providers := map[string]provider.API{}

	// PromptSession tracks whether full DM rules already entered a multi-turn
	// session (Codex thread / Grok HTTP history) so later turns can send a short
	// compact preamble instead of the full system block.
	promptSession := dm.NewPromptSession()

	// --- Codex ---
	var codexClient provider.API
	switch strings.ToLower(envOr("CODEX_MODE", "app-server")) {
	case "app-server", "appserver", "":
		ac := codex.NewAppServerClient(codexCWD)
		defer func() { _ = ac.Close() }()
		codexClient = ac
		log.Printf("DM 資料源已註冊：codex（app-server）")
	case "exec":
		codexClient = codex.NewClient()
		log.Printf("DM 資料源已註冊：codex（exec）")
	default:
		log.Fatalf("unknown CODEX_MODE %q (use \"app-server\" or \"exec\")", envOr("CODEX_MODE", "app-server"))
	}
	providers["codex"] = codexClient

	// --- Grok (CLI login preferred; HTTP if key present) — DM only, not image ---
	if gp := grok.NewProviderFromEnv(); gp != nil {
		providers["grok"] = gp
		st := gp.Status(context.Background())
		if st.Configured {
			log.Printf("DM 資料源已註冊：grok（%s / %s）", st.Provider, st.Model)
		} else {
			log.Printf("DM 資料源已註冊：grok（尚未就緒：%s）", st.Message)
		}
	}

	if _, ok := providers[defaultDM]; !ok {
		if _, ok := providers["codex"]; ok {
			defaultDM = "codex"
		} else if _, ok := providers["grok"]; ok {
			defaultDM = "grok"
		} else {
			log.Fatalf("沒有可用的 DM 資料源")
		}
		log.Printf("DM_PROVIDER 不可用，改用 %s", defaultDM)
	}
	client := providers[defaultDM]
	log.Printf("DM 預設資料源：%s（UI 可切換）", defaultDM)

	// Memory: use Codex text summarizer when available, else Grok structured.
	var mem *memory.Manager
	memDir := filepath.Join(codexCWD, "campaign-data", "memory")
	relDir, rerr := filepath.Rel(codexCWD, memDir)
	if rerr != nil {
		relDir = filepath.Join("campaign-data", "memory")
	}
	threshold := 20
	if v, e := strconv.Atoi(envOr("MEMORY_COMPACT_THRESHOLD", "")); e == nil && v > 0 {
		threshold = v
	}
	tailK := 40
	if v, e := strconv.Atoi(envOr("MEMORY_TAIL", "")); e == nil && v > 0 {
		tailK = v
	}
	var runner func(ctx context.Context, prompt string) (string, error)
	if _, ok := providers["codex"]; ok {
		summarizer := codex.NewClient()
		runner = func(ctx context.Context, prompt string) (string, error) {
			return summarizer.RunText(ctx, prompt, provider.StructuredOpts{CWD: codexCWD, Timeout: 150 * time.Second})
		}
	} else if g, ok := providers["grok"]; ok {
		runner = func(ctx context.Context, prompt string) (string, error) {
			raw, err := g.RunStructured(ctx, prompt+"\n\n回傳 JSON：{\"tags\":\"摘要文字\"}", provider.StructuredOpts{
				SchemaPath: imagePromptSchemaPath, Timeout: 150 * time.Second, CWD: codexCWD,
			})
			if err != nil {
				return "", err
			}
			var parsed struct {
				Tags string `json:"tags"`
			}
			if json.Unmarshal(raw, &parsed) == nil && strings.TrimSpace(parsed.Tags) != "" {
				return parsed.Tags, nil
			}
			return string(raw), nil
		}
	}
	if runner != nil {
		if m, merr := memory.New(db, memDir, relDir, runner, threshold, tailK); merr != nil {
			log.Printf("記憶系統停用（無法建立記憶目錄 %s）：%v", memDir, merr)
		} else {
			mem = m
			log.Printf("記憶系統：SQLite + 匯出檔 %s（compact 門檻 %d 事件）", relDir, threshold)
		}
	}

	// Image generation is GPT-only via Codex CLI $imagegen / image_gen.
	imageRenderers := map[string]images.Renderer{}
	if c, ok := providers["codex"]; ok && c != nil {
		imageRenderers["codex"] = images.NewCodexRenderer(c, codexCWD)
		log.Printf("圖片後端：codex（GPT / Codex $imagegen）")
	} else {
		log.Printf("警告：未註冊 codex，場景／角色圖生成不可用")
	}

	srv := &httpapi.Server{
		Provider:            client,
		Providers:           providers,
		DefaultDMProvider:   defaultDM,
		Store:               db,
		WebDist:             webDist,
		SchemaPath:          schemaPath,
		ProviderCWD:         codexCWD,
		ImageRenderers:      imageRenderers,
		DefaultImageBackend: "codex",
		Memory:              mem,
		Game:                game.New(db, nil),
		TacticsSchemaPath:   tacticsSchemaPath,
		NovelSchemaPath:     novelSchemaPath,
		Prompt:              promptSession,
	}

	addr := net.JoinHostPort("127.0.0.1", port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("D&D local table: http://127.0.0.1:%s", port)
	status := client.Status(context.Background())
	if status.Configured {
		log.Printf("預設 DM %s: %s", defaultDM, status.Model)
	} else {
		log.Printf("預設 DM %s: %s", defaultDM, status.Message)
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}

func resolveRepoRoot() string {
	if root := strings.TrimSpace(os.Getenv("DND_ROOT")); root != "" {
		return absOr(root)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	cwd, _ := os.Getwd()
	return cwd
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func absOr(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
