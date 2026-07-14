package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dndduet/internal/codex"
	"dndduet/internal/httpapi"
	"dndduet/internal/provider"
	"dndduet/internal/store"
)

const validTurnJSON = `{"narration":"隊伍推進，燭火搖曳。","scene":"禮拜堂","objective":"找到伊薩克","objectiveContext":"線索指向地下","stakes":"午夜漲潮","requiresCheck":false,"check":null,"choices":["搜索祭壇","檢查泥痕"],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`

// fakeCodex embeds a real client (for the pure NormalizeModel/Model helpers) and
// overrides the CLI-touching methods.
type fakeCodex struct {
	*codex.Client
	status    provider.Status
	turn      string
	imagePath string
	imgCalls  int
}

func (f *fakeCodex) Status(context.Context) provider.Status { return f.status }
func (f *fakeCodex) RunStructured(context.Context, string, provider.StructuredOpts) (json.RawMessage, error) {
	return json.RawMessage(f.turn), nil
}
func (f *fakeCodex) RunImageGeneration(context.Context, string, provider.ImageOpts) (string, error) {
	f.imgCalls++
	return f.imagePath, nil
}

func newServer(t *testing.T, fake *fakeCodex) *httpapi.Server {
	t.Helper()
	dir := t.TempDir()
	webDist := filepath.Join(dir, "web-dist")
	if err := os.MkdirAll(webDist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDist, "index.html"), []byte("<!doctype html><title>table</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return &httpapi.Server{
		Provider:    fake,
		Store:       st,
		WebDist:     webDist,
		SchemaPath:  filepath.Join(dir, "schema.json"),
		ProviderCWD: dir,
	}
}

func configured() provider.Status {
	return provider.Status{Configured: true, Provider: "Codex CLI（ChatGPT 登入）", Model: "test-model"}
}

func do(t *testing.T, srv *httpapi.Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("content-type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, r)
	return w
}

func TestStatusEndpoint(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured(), turn: validTurnJSON})
	w := do(t, srv, http.MethodGet, "/api/status", "")
	if w.Code != 200 {
		t.Fatalf("status code %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["connected"] != true {
		t.Errorf("connected = %v", resp["connected"])
	}
	if resp["imageModel"] != codex.ImageModel {
		t.Errorf("imageModel = %v", resp["imageModel"])
	}
	models, _ := resp["models"].([]any)
	if len(models) != len(codex.ModelOptions) {
		t.Errorf("models length = %d", len(models))
	}
	if _, hasMessage := resp["message"]; hasMessage {
		t.Error("message must be omitted when connected")
	}
}

func TestStatusIncludesMessageWhenDisconnected(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: provider.Status{Configured: false, Provider: "Codex CLI", Model: "Codex 預設模型", Message: "尚未登入"}})
	w := do(t, srv, http.MethodGet, "/api/status", "")
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] != "尚未登入" {
		t.Errorf("message = %v", resp["message"])
	}
}

func TestDmEndpointSuccess(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured(), turn: validTurnJSON})
	body := `{"players":[{"name":"甲","className":"法師"}],"actions":[{"playerId":"player1","text":"檢查符文"}],"campaign":{"title":"測試","scene":"石門","round":1}}`
	w := do(t, srv, http.MethodPost, "/api/dm", body)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	text, _ := resp["text"].(string)
	if !strings.Contains(text, "隊伍推進") || !strings.Contains(text, "可考慮：搜索祭壇／檢查泥痕") {
		t.Errorf("text = %q", text)
	}
	if resp["scene"] != "禮拜堂" {
		t.Errorf("scene = %v", resp["scene"])
	}
	if resp["model"] != "test-model" {
		t.Errorf("model = %v", resp["model"])
	}
}

func TestDmEndpointRejectsIncompleteParty(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured(), turn: validTurnJSON})
	body := `{"players":[{"name":"甲","className":"法師"},{"name":"乙","className":"戰士"}],"actions":[{"playerId":"player1","text":"檢查符文"}]}`
	w := do(t, srv, http.MethodPost, "/api/dm", body)
	if w.Code != 400 {
		t.Fatalf("status %d, want 400", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if msg, _ := resp["error"].(string); !strings.Contains(msg, "每位玩家") {
		t.Errorf("error = %v", resp["error"])
	}
}

func TestSceneImageRejectsMissingFields(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured()})
	w := do(t, srv, http.MethodPost, "/api/scene-image", `{"campaign":{"title":"t"},"narration":""}`)
	if w.Code != 400 {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestSceneImageSuccessAndServe(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "source.png")
	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a}
	if err := os.WriteFile(imgPath, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCodex{Client: &codex.Client{}, status: configured(), imagePath: imgPath}
	srv := newServer(t, fake)

	body := `{"campaign":{"title":"灰燼王冠","scene":"禮拜堂"},"narration":"燭火搖曳的禮拜堂","players":[{"name":"甲","className":"法師"}]}`
	w := do(t, srv, http.MethodPost, "/api/scene-image", body)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	url, _ := resp["url"].(string)
	if !strings.HasPrefix(url, "/generated/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("url = %q", url)
	}
	if fake.imgCalls != 1 {
		t.Errorf("image generation calls = %d", fake.imgCalls)
	}

	// The generated image must now be served from SQLite.
	served := do(t, srv, http.MethodGet, url, "")
	if served.Code != 200 {
		t.Fatalf("serve status %d", served.Code)
	}
	if served.Header().Get("content-type") != "image/png" {
		t.Errorf("content-type = %q", served.Header().Get("content-type"))
	}
	if !strings.Contains(served.Header().Get("cache-control"), "immutable") {
		t.Errorf("cache-control = %q", served.Header().Get("cache-control"))
	}
	if string(served.Body.Bytes()) != string(pngBytes) {
		t.Error("served bytes differ from generated image")
	}
}

func TestCharacterImageSuccess(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "portrait.jpg")
	jpgBytes := []byte{0xff, 0xd8, 0xff, 0xe0}
	if err := os.WriteFile(imgPath, jpgBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCodex{Client: &codex.Client{}, status: configured(), imagePath: imgPath}
	srv := newServer(t, fake)

	body := `{"name":"賽勒恩","species":"人類","className":"遊俠","background":"獵人","appearance":"披著灰斗篷的高瘦男子"}`
	w := do(t, srv, http.MethodPost, "/api/character-image", body)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	url, _ := resp["url"].(string)
	if !strings.HasPrefix(url, "/generated/") || !strings.HasSuffix(url, ".jpg") {
		t.Fatalf("url = %q", url)
	}
	served := do(t, srv, http.MethodGet, url, "")
	if served.Code != 200 || served.Header().Get("content-type") != "image/jpeg" {
		t.Errorf("serve: status %d type %q", served.Code, served.Header().Get("content-type"))
	}
}

func TestCharacterImageRejectsMissingFields(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured()})
	// Missing appearance.
	w := do(t, srv, http.MethodPost, "/api/character-image", `{"name":"賽勒恩","appearance":""}`)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestStaticRejectsPathTraversal(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured()})
	// A traversal attempt must not escape web-dist (403), and must never 200
	// with outside content.
	w := do(t, srv, http.MethodGet, "/..%2f..%2f..%2fpackage.json", "")
	if w.Code == 200 && strings.Contains(w.Body.String(), "dndDuet") {
		t.Fatalf("path traversal leaked a file outside web-dist: %q", w.Body.String())
	}
	if w.Code != 403 {
		t.Logf("traversal returned %d (acceptable as long as no leak)", w.Code)
	}
}

func TestNonObjectDmBodyReturns400(t *testing.T) {
	// A JSON array body has no players -> Node returned 400 (empty party), not a
	// 503 parse error. The Go backend must match.
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured(), turn: validTurnJSON})
	w := do(t, srv, http.MethodPost, "/api/dm", `[]`)
	if w.Code != 400 {
		t.Errorf("array body status = %d, want 400", w.Code)
	}
}

func TestGeneratedRejectsBadName(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured()})
	if w := do(t, srv, http.MethodGet, "/generated/..%2fpasswd", ""); w.Code != 400 {
		t.Errorf("bad name status = %d, want 400", w.Code)
	}
	if w := do(t, srv, http.MethodGet, "/generated/missing.png", ""); w.Code != 404 {
		t.Errorf("missing image status = %d, want 404", w.Code)
	}
}

func TestStaticServesSPA(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured()})
	w := do(t, srv, http.MethodGet, "/", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "table") {
		t.Fatalf("root status %d body %q", w.Code, w.Body.String())
	}
	// Unknown client route falls back to index.html (SPA behaviour).
	deep := do(t, srv, http.MethodGet, "/journal", "")
	if deep.Code != 200 || !strings.Contains(deep.Body.String(), "table") {
		t.Errorf("SPA fallback status %d", deep.Code)
	}
}
