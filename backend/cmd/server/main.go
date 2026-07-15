// Command server runs the D&D Duet local backend: the /api endpoints, generated
// images from SQLite, and the built frontend. It replaces server.mjs.
package main

import (
	"bufio"
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dndduet/internal/codex"
	"dndduet/internal/forge"
	"dndduet/internal/httpapi"
	"dndduet/internal/images"
	"dndduet/internal/provider"
	"dndduet/internal/store"
	"dndduet/internal/tts"
	schema "dndduet/schemas"
)

func main() {
	repoRoot := resolveRepoRoot()
	loadEnvFile(filepath.Join(repoRoot, ".env"))
	loadEnvFile(filepath.Join(repoRoot, "backend", ".env"))

	port := envOr("PORT", "4318")

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

	db, err := store.Open(filepath.Join(dataDir, "dnd-duet.db"))
	if err != nil {
		log.Fatalf("cannot open database: %v", err)
	}
	defer db.Close()

	// CODEX_MODE selects how the DM turn reaches Codex:
	//   app-server (default) — keep one `codex app-server` process alive and run
	//                          turns over its persistent connection (long-lived)
	//   exec                 — spawn `codex exec` per request (fallback)
	var client provider.API
	switch strings.ToLower(envOr("CODEX_MODE", "app-server")) {
	case "app-server", "appserver", "":
		ac := codex.NewAppServerClient(codexCWD)
		defer ac.Close()
		client = ac
		log.Printf("Codex 連線模式：app-server（長連線，常駐單一行程）")
	case "exec":
		client = codex.NewClient()
		log.Printf("Codex 連線模式：exec（每次請求 spawn）")
	default:
		log.Fatalf("unknown CODEX_MODE %q (use \"app-server\" or \"exec\")", envOr("CODEX_MODE", "app-server"))
	}

	// IMAGE_BACKEND selects the default illustration backend; the frontend can
	// override it per request:
	//   codex (default) — Codex CLI's built-in image_gen tool (cloud)
	//   local           — Stable Diffusion WebUI Forge on this machine (FORGE_*)
	forgeClient := forge.NewClientFromEnv()
	defaultImageBackend := strings.ToLower(envOr("IMAGE_BACKEND", "codex"))
	switch defaultImageBackend {
	case "codex":
	case "local", "forge", "sd":
		defaultImageBackend = "local"
	default:
		log.Fatalf("unknown IMAGE_BACKEND %q (use \"codex\" or \"local\")", envOr("IMAGE_BACKEND", "codex"))
	}
	log.Printf("圖片後端：預設 %s（本地 SD Forge：%s）", defaultImageBackend, forgeClient.BaseURL)

	srv := &httpapi.Server{
		Provider:    client,
		Store:       db,
		WebDist:     webDist,
		SchemaPath:  schemaPath,
		ProviderCWD: codexCWD,
		ImageRenderers: map[string]images.Renderer{
			"codex": images.NewCodexRenderer(client, codexCWD),
			"local": images.NewForgeRenderer(forgeClient),
		},
		DefaultImageBackend: defaultImageBackend,
		TTS:                 tts.NewClientFromEnv(),
	}
	log.Printf("語音朗讀：GPT-SoVITS %s（未啟動時 /api/tts 會回報連線錯誤）", srv.TTS.BaseURL)

	addr := net.JoinHostPort("127.0.0.1", port)
	httpServer := &http.Server{Addr: addr, Handler: srv.Router()}

	log.Printf("D&D local table: http://127.0.0.1:%s", port)
	status := client.Status(context.Background())
	if status.Configured {
		log.Printf("%s: %s", status.Provider, status.Model)
	} else {
		log.Printf("%s: %s", status.Provider, status.Message)
	}

	// Shut down gracefully on Ctrl-C / SIGTERM so the deferred Close() calls run
	// (important in app-server mode, which owns a long-lived child process).
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

// resolveRepoRoot finds the repository root by walking up for a .git directory,
// so the server locates web-dist and campaign-data whether it is launched from
// the repo root or from backend/.
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

// loadEnvFile applies KEY=VALUE lines from a .env file without overriding
// variables already present in the environment.
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
