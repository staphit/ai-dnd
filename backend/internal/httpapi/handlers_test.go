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
	"dndduet/internal/game"
	"dndduet/internal/httpapi"
	"dndduet/internal/images"
	"dndduet/internal/provider"
	"dndduet/internal/store"
)

const validTurnJSON = `{"narration":"隊伍推進，燭火搖曳。","scene":"禮拜堂","objective":"找到伊薩克","objectiveContext":"線索指向地下","stakes":"午夜漲潮","requiresCheck":false,"check":null,"choices":["搜索祭壇","檢查泥痕"],"effects":[],"privateMessages":[],"combat":{"starts":false,"firstTurn":"initiative","enemies":[]},"actionIssues":[],"experienceAwards":[]}`

// fakeCodex embeds a real client (for the pure NormalizeModel/Model helpers) and
// overrides the CLI-touching methods.
type fakeCodex struct {
	*codex.Client
	status          provider.Status
	turn            string
	imagePath       string
	structuredCalls int
	imgCalls        int
}

func (f *fakeCodex) Status(context.Context) provider.Status { return f.status }
func (f *fakeCodex) RunStructured(context.Context, string, provider.StructuredOpts) (json.RawMessage, error) {
	f.structuredCalls++
	return json.RawMessage(f.turn), nil
}
func (f *fakeCodex) RunImageGeneration(context.Context, string, provider.ImageOpts) (string, error) {
	f.imgCalls++
	return f.imagePath, nil
}

func newTestServer(t *testing.T, fake *fakeCodex) *httpapi.Server {
	t.Helper()
	dir := t.TempDir()
	webDist := filepath.Join(dir, "web-dist")
	if err := os.MkdirAll(webDist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDist, "index.html"), []byte("<!doctype html><title>table</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "images"))
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
		ImageRenderers: map[string]images.Renderer{
			"codex": images.NewCodexRenderer(fake, dir),
		},
		DefaultImageBackend: "codex",
		Game:                game.New(st, nil),
	}
}

// seedCampaign creates a server-side campaign and returns its id.
func seedCampaign(t *testing.T, srv *httpapi.Server, players ...game.PlayerSeed) string {
	t.Helper()
	if len(players) == 0 {
		players = []game.PlayerSeed{{Name: "甲", ClassName: "法師"}}
	}
	view, err := srv.Game.Create(game.CreateParams{
		ID: "campaign-1", Title: "測試", Scene: "石門", Objective: "找到伊薩克",
		Players: players,
	})
	if err != nil {
		t.Fatalf("seed campaign: %v", err)
	}
	return view.ID
}

func newServer(t *testing.T, fake *fakeCodex) *httpapi.Server {
	t.Helper()
	return newTestServer(t, fake)
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
	r.Host = "127.0.0.1:4318"
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
	efforts, _ := resp["efforts"].([]any)
	if len(efforts) != len(codex.EffortOptions) {
		t.Errorf("efforts length = %d", len(efforts))
	}
	backends, _ := resp["imageBackends"].([]any)
	if len(backends) != 1 {
		t.Errorf("imageBackends length = %d, want 1", len(backends))
	}
	if resp["imageBackend"] != "codex" {
		t.Errorf("imageBackend = %v", resp["imageBackend"])
	}
	if _, hasMessage := resp["message"]; hasMessage {
		t.Error("message must be omitted when connected")
	}
}

func TestSceneImageUsesCodexBackend(t *testing.T) {
	dir := t.TempDir()
	png := []byte{0x89, 0x50, 0x4e, 0x47}
	imgPath := filepath.Join(dir, "out.png")
	if err := os.WriteFile(imgPath, png, 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCodex{Client: &codex.Client{}, status: configured(), imagePath: imgPath}
	srv := newServer(t, fake)

	body := `{"imageBackend":"codex","campaign":{"title":"灰燼王冠","scene":"禮拜堂"},"narration":"燭火搖曳","players":[]}`
	w := do(t, srv, http.MethodPost, "/api/scene-image", body)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if fake.imgCalls != 1 {
		t.Errorf("codex image calls = %d, want 1", fake.imgCalls)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["model"] != codex.ImageModel {
		t.Errorf("model = %v, want %s", resp["model"], codex.ImageModel)
	}
}

func TestSceneImageLegacyBackendFallsBackToCodex(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "out.png")
	if err := os.WriteFile(imgPath, []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCodex{Client: &codex.Client{}, status: configured(), imagePath: imgPath}
	srv := newServer(t, fake)
	// Old settings may still send local/grok; server maps them to codex.
	body := `{"imageBackend":"local","campaign":{"title":"t","scene":"s"},"narration":"n"}`
	w := do(t, srv, http.MethodPost, "/api/scene-image", body)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if fake.imgCalls != 1 {
		t.Errorf("codex image calls = %d, want 1", fake.imgCalls)
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
	id := seedCampaign(t, srv)
	body := `{"campaignId":"` + id + `","actions":[{"playerId":"player1","text":"檢查符文"}]}`
	w := do(t, srv, http.MethodPost, "/api/dm", body)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	text, _ := resp["text"].(string)
	if !strings.Contains(text, "隊伍推進") {
		t.Errorf("text = %q", text)
	}
	// Choices are returned as structured chips, not appended to public text.
	choices, _ := resp["choices"].([]any)
	if len(choices) == 0 {
		t.Errorf("expected structured choices, got %v", resp["choices"])
	}
	view, _ := resp["view"].(map[string]any)
	if view == nil {
		t.Fatalf("missing view in response: %v", resp)
	}
	if view["scene"] != "禮拜堂" {
		t.Errorf("view scene = %v", view["scene"])
	}
	if view["round"] != float64(2) {
		t.Errorf("round should advance to 2, got %v", view["round"])
	}
	story, _ := view["story"].([]any)
	if len(story) < 3 { // setup system + action + narration
		t.Fatalf("story should carry action + narration entries, got %d", len(story))
	}
	last, _ := story[len(story)-1].(map[string]any)
	if last["speaker"] != "dm" || !strings.Contains(last["text"].(string), "隊伍推進") {
		t.Errorf("last story entry should be the DM narration: %v", last)
	}
	if resp["model"] != "test-model" {
		t.Errorf("model = %v", resp["model"])
	}
}

func TestDemoDmUsesBackendAndSkipsProvider(t *testing.T) {
	fake := &fakeCodex{Client: &codex.Client{}, status: configured(), turn: validTurnJSON}
	srv := newServer(t, fake)
	id := seedCampaign(t, srv)
	body := `{"campaignId":"` + id + `","demo":true,"actions":[{"playerId":"player1","text":"檢查符文"}]}`
	w := do(t, srv, http.MethodPost, "/api/dm", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if fake.structuredCalls != 0 {
		t.Fatalf("demo called provider %d times", fake.structuredCalls)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["model"] != "示範 DM" {
		t.Fatalf("model = %v", resp["model"])
	}
	view := resp["view"].(map[string]any)
	if view["round"] != float64(2) {
		t.Fatalf("demo round = %v", view["round"])
	}
	players := view["players"].([]any)
	if players[0].(map[string]any)["experience"] == float64(0) {
		t.Fatal("demo XP was not persisted")
	}
}

func TestLocalRequestGuardRejectsRebindingAndForeignOrigin(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured()})

	rebinding := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rebinding.Host = "attacker.example"
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, rebinding)
	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign Host status = %d", w.Code)
	}

	foreign := httptest.NewRequest(http.MethodPost, "/api/campaigns", strings.NewReader(`{}`))
	foreign.Host = "127.0.0.1:4318"
	foreign.Header.Set("Origin", "https://attacker.example")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, foreign)
	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign Origin status = %d", w.Code)
	}
}

func TestDmEndpointRejectsIncompleteParty(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured(), turn: validTurnJSON})
	id := seedCampaign(t, srv, game.PlayerSeed{Name: "甲", ClassName: "法師"}, game.PlayerSeed{Name: "乙", ClassName: "戰士"})
	body := `{"campaignId":"` + id + `","actions":[{"playerId":"player1","text":"檢查符文"}]}`
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

func TestDmEndpointPreValidatesSpells(t *testing.T) {
	srv := newServer(t, &fakeCodex{Client: &codex.Client{}, status: configured(), turn: validTurnJSON})
	id := seedCampaign(t, srv, game.PlayerSeed{Name: "米芮", ClassName: "牧師"})
	// Drain every level-1+ slot so a named cast must fail mechanically.
	view, err := srv.Game.View(id)
	if err != nil {
		t.Fatal(err)
	}
	spellName, spellID := "", ""
	for _, sp := range view.Players[0].Spellcasting.Spells {
		if sp.Level > 0 && (sp.Prepared || sp.AlwaysPrepared) && (sp.Effect == nil || sp.Effect.Kind != "damage") {
			spellName, spellID = sp.Name, sp.ID
			break
		}
	}
	if spellID == "" {
		t.Fatal("cleric should have a prepared non-damage level-1 spell")
	}
	for i := 0; i < 10; i++ {
		if _, err := srv.Game.CastSpell(id, "player1", game.CastParams{SpellID: spellID, TargetID: "player1"}); err != nil {
			break
		}
		if _, err := srv.Game.UnlockAction(id, "player1"); err != nil {
			t.Fatal(err)
		}
	}

	body := `{"campaignId":"` + id + `","actions":[{"playerId":"player1","text":"我對自己施放「` + spellName + `」"}]}`
	w := do(t, srv, http.MethodPost, "/api/dm", body)
	if w.Code != 422 {
		t.Fatalf("status %d body %s, want 422", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	issues, _ := resp["actionIssues"].([]any)
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %v", resp)
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
