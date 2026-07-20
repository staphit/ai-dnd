package images_test

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

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
	st, err := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "images"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func configured() provider.Status {
	return provider.Status{Configured: true, Provider: "Codex CLI", Model: "m"}
}

func TestGenerateSceneSendsContentDirectlyAndStores(t *testing.T) {
	dir := t.TempDir()
	png := []byte{0x89, 0x50, 0x4e, 0x47}
	src := writeFile(t, dir, "src.png", png)
	st := newStore(t, dir)
	api := &fakeProvider{status: configured(), imagePath: src}

	res, err := images.GenerateScene(context.Background(), images.NewCodexRenderer(api, dir), st, images.SceneInput{
		Title:     "灰燼王冠",
		Scene:     "廢墟禮拜堂",
		Narration: "燭火搖曳，門後有聲。",
		Players:   []images.ScenePlayer{{Name: "甲", ClassName: "法師", Species: "人類"}, {Name: "乙", ClassName: "戰士", Species: "矮人"}},
	})
	if err != nil {
		t.Fatalf("GenerateScene: %v", err)
	}
	// Direct dump: no intermediate visualData JSON, no multi-step SD engineering brief.
	if strings.Contains(res.Prompt, "visualData") {
		t.Errorf("prompt should not wrap visualData JSON, got: %s", res.Prompt)
	}
	for _, want := range []string{"灰燼王冠", "廢墟禮拜堂", "燭火搖曳", "甲", "法師", "image_gen"} {
		if !strings.Contains(res.Prompt, want) {
			t.Errorf("prompt missing %q\ngot: %s", want, res.Prompt)
		}
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

func TestGenerateCharacterSendsContentDirectly(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "src.webp", []byte{1, 2, 3})
	st := newStore(t, dir)
	api := &fakeProvider{status: configured(), imagePath: src}

	res, err := images.GenerateCharacter(context.Background(), images.NewCodexRenderer(api, dir), st, images.CharacterInput{
		Name: "賽勒恩", Species: "精靈", ClassName: "遊俠", Background: "流浪者", Appearance: "銀髮綠眼",
	})
	if err != nil {
		t.Fatalf("GenerateCharacter: %v", err)
	}
	if strings.Contains(res.Prompt, "visualData") {
		t.Errorf("prompt should not wrap visualData JSON, got: %s", res.Prompt)
	}
	for _, want := range []string{"賽勒恩", "精靈", "遊俠", "銀髮綠眼", "image_gen"} {
		if !strings.Contains(res.Prompt, want) {
			t.Errorf("prompt missing %q\ngot: %s", want, res.Prompt)
		}
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
