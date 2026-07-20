package images_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"dndduet/internal/forge"
	"dndduet/internal/images"
	"dndduet/internal/provider"
	"dndduet/internal/store"
)

type fakeProvider struct {
	status    provider.Status
	imagePath string
}

func (f *fakeProvider) Status(context.Context) provider.Status   { return f.status }
func (f *fakeProvider) Connect(context.Context, string) error    { return nil }
func (f *fakeProvider) ConnectionState() provider.ConnState      { return provider.ConnState{Alive: true} }
func (f *fakeProvider) NormalizeModel(v string) (string, error)  { return v, nil }
func (f *fakeProvider) NormalizeEffort(v string) (string, error) { return v, nil }
func (f *fakeProvider) EffortOptions() []provider.ModelOption    { return nil }
func (f *fakeProvider) Model() string                            { return "model" }
func (f *fakeProvider) ModelOptions() []provider.ModelOption     { return nil }
func (f *fakeProvider) ImageModel() string                       { return "IMG-MODEL" }
func (f *fakeProvider) RunStructured(context.Context, string, provider.StructuredOpts) (json.RawMessage, error) {
	return nil, nil
}
func (f *fakeProvider) RunImageGeneration(context.Context, string, provider.ImageOpts) (string, error) {
	return f.imagePath, nil
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func newStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func configured() provider.Status {
	return provider.Status{Configured: true, Provider: "Codex CLI", Model: "m"}
}

func TestGenerateSceneOrdersVisualDataAndStores(t *testing.T) {
	dir := t.TempDir()
	png := []byte{0x89, 0x50, 0x4e, 0x47}
	src := writeFile(t, dir, "src.png", png)
	st := newStore(t, dir)
	api := &fakeProvider{status: configured(), imagePath: src}

	res, err := images.GenerateScene(context.Background(), images.NewCodexRenderer(api, dir), st, images.SceneInput{
		Title:     "T",
		Scene:     "S",
		Narration: "N",
		Players:   []images.ScenePlayer{{Name: "甲", ClassName: "法師"}, {Name: "乙", ClassName: "戰士"}},
	})
	if err != nil {
		t.Fatalf("GenerateScene: %v", err)
	}
	// Keys must follow the Node object-literal order, not Go's alphabetical map order.
	want := `{"visualData":{"campaign":"T","location":"S","characters":"甲，法師；乙，戰士","latestScene":"N"}}`
	if !strings.Contains(res.Prompt, want) {
		t.Errorf("prompt missing ordered visualData\nwant substring: %s\ngot: %s", want, res.Prompt)
	}
	if res.Model != "IMG-MODEL" {
		t.Errorf("model = %q", res.Model)
	}
	if !strings.HasPrefix(res.URL, "/generated/") || !strings.HasSuffix(res.URL, ".png") {
		t.Errorf("url = %q", res.URL)
	}
	img, ok, _ := st.GetImage(path.Base(res.URL))
	if !ok || len(img.Bytes) != len(png) || img.Mime != "image/png" {
		t.Errorf("stored image wrong: ok=%v mime=%q len=%d", ok, img.Mime, len(img.Bytes))
	}
}

func TestGenerateCharacterOrdersVisualData(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "src.webp", []byte{1, 2, 3})
	st := newStore(t, dir)
	api := &fakeProvider{status: configured(), imagePath: src}

	res, err := images.GenerateCharacter(context.Background(), images.NewCodexRenderer(api, dir), st, images.CharacterInput{
		Name: "N", Species: "S", ClassName: "C", Background: "B", Appearance: "A",
	})
	if err != nil {
		t.Fatalf("GenerateCharacter: %v", err)
	}
	want := `{"visualData":{"name":"N","species":"S","className":"C","background":"B","appearance":"A"}}`
	if !strings.Contains(res.Prompt, want) {
		t.Errorf("prompt missing ordered visualData\nwant substring: %s\ngot: %s", want, res.Prompt)
	}
	if !strings.HasSuffix(res.URL, ".webp") {
		t.Errorf("url = %q (webp ext expected)", res.URL)
	}
	img, _, _ := st.GetImage(path.Base(res.URL))
	if img.Mime != "image/webp" {
		t.Errorf("mime = %q, want image/webp", img.Mime)
	}
}

func TestGenerateSceneEmptyFileRejected(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "empty.png", []byte{})
	st := newStore(t, dir)
	api := &fakeProvider{status: configured(), imagePath: src}
	_, err := images.GenerateScene(context.Background(), images.NewCodexRenderer(api, dir), st, images.SceneInput{Title: "T", Scene: "S", Narration: "N"})
	if err == nil || !strings.Contains(err.Error(), "空檔案") {
		t.Errorf("expected empty-file error, got %v", err)
	}
}

func TestGenerateRequiresConfigured(t *testing.T) {
	dir := t.TempDir()
	st := newStore(t, dir)
	api := &fakeProvider{status: provider.Status{Configured: false, Message: "尚未登入"}}
	_, err := images.GenerateScene(context.Background(), images.NewCodexRenderer(api, dir), st, images.SceneInput{Title: "T", Scene: "S", Narration: "N"})
	if err == nil || !strings.Contains(err.Error(), "尚未登入") {
		t.Errorf("expected not-configured error, got %v", err)
	}
}

// newForgeServer fakes a Forge /sdapi/v1/txt2img endpoint and captures the
// request payload.
func newForgeServer(t *testing.T, respond func(w http.ResponseWriter)) (*httptest.Server, *map[string]any) {
	t.Helper()
	captured := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sdapi/v1/txt2img" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		respond(w)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func forgeClientFor(url string) *forge.Client {
	return &forge.Client{
		BaseURL: url, Checkpoint: "juggernautXL", Steps: 6, CFGScale: 1.5, Sampler: "Euler", Scheduler: "SGM Uniform",
		SceneWidth: 1216, SceneHeight: 832, PortraitWidth: 1024, PortraitHeight: 1024,
	}
}

func TestForgeSceneGeneratesAndStores(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 1, 2}
	srv, captured := newForgeServer(t, func(w http.ResponseWriter) {
		json.NewEncoder(w).Encode(map[string]any{"images": []string{base64.StdEncoding.EncodeToString(png)}})
	})
	dir := t.TempDir()
	st := newStore(t, dir)
	renderer := images.NewForgeRenderer(forgeClientFor(srv.URL), nil)

	res, err := images.GenerateScene(context.Background(), renderer, st, images.SceneInput{
		Title: "T", Scene: "廢棄禮拜堂", Narration: "燭火搖曳",
		Players: []images.ScenePlayer{{Name: "甲", ClassName: "法師"}},
	})
	if err != nil {
		t.Fatalf("GenerateScene(forge): %v", err)
	}
	if !strings.HasSuffix(res.URL, ".png") {
		t.Errorf("url = %q", res.URL)
	}
	if res.Model != "SD Forge（juggernautXL）" {
		t.Errorf("model = %q", res.Model)
	}
	prompt, _ := (*captured)["prompt"].(string)
	if !strings.Contains(prompt, "廢棄禮拜堂") || !strings.Contains(prompt, "photorealistic") {
		t.Errorf("prompt = %q", prompt)
	}
	if neg, _ := (*captured)["negative_prompt"].(string); !strings.Contains(neg, "watermark") {
		t.Errorf("negative_prompt = %q", neg)
	}
	if w, h := (*captured)["width"].(float64), (*captured)["height"].(float64); w != 1216 || h != 832 {
		t.Errorf("size = %vx%v, want 1216x832", w, h)
	}
	if s, _ := (*captured)["sampler_name"].(string); s != "Euler" {
		t.Errorf("sampler_name = %q", s)
	}
	override, _ := (*captured)["override_settings"].(map[string]any)
	if override["sd_model_checkpoint"] != "juggernautXL" {
		t.Errorf("override_settings = %v", override)
	}
	img, ok, _ := st.GetImage(path.Base(res.URL))
	if !ok || len(img.Bytes) != len(png) {
		t.Errorf("stored image wrong: ok=%v len=%d", ok, len(img.Bytes))
	}
}

func TestForgeSceneAppliesPlayerOptionsAndKeepsNegativeActive(t *testing.T) {
	srv, captured := newForgeServer(t, func(w http.ResponseWriter) {
		json.NewEncoder(w).Encode(map[string]any{`images`: []string{base64.StdEncoding.EncodeToString([]byte{1})}})
	})
	renderer := images.NewForgeRenderer(forgeClientFor(srv.URL), nil)
	seed := int64(42)
	_, err := renderer.RenderScene(context.Background(), images.SceneInput{
		Title: `T`, Scene: `S`, Narration: `N`,
		Forge: &images.ForgeOptions{
			PositivePrompt: `exact positive`, NegativePrompt: `no dragons`,
			Steps: 3, CFGScale: 1, Sampler: `DDIM`, Scheduler: `Karras`,
			Seed: &seed, Width: 640, Height: 512,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]any{
		`prompt`: `exact positive`, `negative_prompt`: `no dragons`,
		`steps`: float64(3), `cfg_scale`: float64(2),
		`sampler_name`: `DDIM`, `scheduler`: `Karras`,
		`seed`: float64(42), `width`: float64(640), `height`: float64(512),
	}
	for key, want := range checks {
		if got := (*captured)[key]; got != want {
			t.Errorf(`%s = %v, want %v`, key, got, want)
		}
	}
}

func TestForgeSceneAlwaysIncludesCustomPlayerAppearance(t *testing.T) {
	srv, captured := newForgeServer(t, func(w http.ResponseWriter) {
		json.NewEncoder(w).Encode(map[string]any{`images`: []string{base64.StdEncoding.EncodeToString([]byte{1})}})
	})
	renderer := images.NewForgeRenderer(forgeClientFor(srv.URL), nil)

	_, err := renderer.RenderScene(context.Background(), images.SceneInput{
		Title: `T`, Scene: `S`, Narration: `N`,
		ImagePrompt: `moonlit ruined chapel`,
		Players: []images.ScenePlayer{{
			Name: `Private Player Name`, Species: `human`, ClassName: `druid`,
			Appearance: `silver braided hair, emerald eyes, scar across left cheek`,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	prompt, _ := (*captured)[`prompt`].(string)
	for _, want := range []string{`human druid`, `silver braided hair`, `emerald eyes`, `scar across left cheek`, `moonlit ruined chapel`} {
		if !strings.Contains(prompt, want) {
			t.Errorf(`prompt missing %q: %q`, want, prompt)
		}
	}
	if strings.Contains(prompt, `Private Player Name`) {
		t.Errorf(`prompt leaked player name: %q`, prompt)
	}
}

func TestForgeCharacterUsesSquarePortrait(t *testing.T) {
	srv, captured := newForgeServer(t, func(w http.ResponseWriter) {
		json.NewEncoder(w).Encode(map[string]any{"images": []string{base64.StdEncoding.EncodeToString([]byte{1})}})
	})
	dir := t.TempDir()
	st := newStore(t, dir)
	renderer := images.NewForgeRenderer(forgeClientFor(srv.URL), nil)

	_, err := images.GenerateCharacter(context.Background(), renderer, st, images.CharacterInput{
		Name: "賽勒恩", Species: "精靈", ClassName: "遊俠", Background: "獵人", Appearance: "灰斗篷",
	})
	if err != nil {
		t.Fatalf("GenerateCharacter(forge): %v", err)
	}
	if w, h := (*captured)["width"].(float64), (*captured)["height"].(float64); w != 1024 || h != 1024 {
		t.Errorf("size = %vx%v, want 1024x1024", w, h)
	}
	prompt, _ := (*captured)["prompt"].(string)
	if !strings.Contains(prompt, "elf ranger") || !strings.Contains(prompt, "灰斗篷") {
		t.Errorf("prompt = %q", prompt)
	}
}

func TestForgeErrorsSurfaceMessage(t *testing.T) {
	srv, _ := newForgeServer(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("CUDA out of memory"))
	})
	dir := t.TempDir()
	st := newStore(t, dir)
	renderer := images.NewForgeRenderer(forgeClientFor(srv.URL), nil)

	_, err := images.GenerateScene(context.Background(), renderer, st, images.SceneInput{Title: "T", Scene: "S", Narration: "N"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "CUDA out of memory") {
		t.Errorf("expected HTTP 500 error with detail, got %v", err)
	}
}

func TestForgeUnreachableGivesFriendlyError(t *testing.T) {
	dir := t.TempDir()
	st := newStore(t, dir)
	renderer := images.NewForgeRenderer(&forge.Client{BaseURL: "http://127.0.0.1:1"}, nil)

	_, err := images.GenerateScene(context.Background(), renderer, st, images.SceneInput{Title: "T", Scene: "S", Narration: "N"})
	if err == nil || !strings.Contains(err.Error(), "無法連線本地 SD Forge") {
		t.Errorf("expected connection error, got %v", err)
	}
}
