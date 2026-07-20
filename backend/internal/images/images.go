// Package images generates scene and character illustrations and stores the
// results as session files on disk (not SQLite). Rendering goes through the
// Codex CLI's built-in image_gen tool (GPT / ChatGPT image generation).
package images

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dndduet/internal/provider"
	"dndduet/internal/store"
)

// ScenePlayer is the character description used in scene prompts. Species and
// Appearance keep the party's look consistent across scene illustrations.
type ScenePlayer struct {
	Name       string
	ClassName  string
	Species    string
	Appearance string
}

// defaultAppearance anchors a party member whose player left appearance blank,
// so scene images don't render a wildly different figure each time.
const defaultAppearance = "weathered traveler wearing a cloak and leather armor"

// SceneInput describes a scene to illustrate.
type SceneInput struct {
	Title     string
	Scene     string
	Narration string
	// ImagePrompt is optional visual guidance from the DM agent.
	ImagePrompt string
	Players     []ScenePlayer
	// Optional SQLite image_meta linkage.
	CampaignID   string
	SourceSlotID string
}

// CharacterInput describes a character portrait to generate.
type CharacterInput struct {
	Name       string
	Species    string
	ClassName  string
	Background string
	Appearance string
	CampaignID string
}

// Result is the JSON payload returned to the client.
type Result struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
	Model  string `json:"model"`
}

// Rendered is one generated image before persistence.
type Rendered struct {
	Data   []byte
	Ext    string // lower-case extension including the dot, e.g. ".png"
	Prompt string
	Model  string
}

// Renderer produces illustrations for one image backend.
type Renderer interface {
	// Model is the human-readable label for this backend (shown in /api/status).
	Model() string
	RenderScene(ctx context.Context, input SceneInput) (Rendered, error)
	RenderCharacter(ctx context.Context, input CharacterInput) (Rendered, error)
}

// clock and idGen are overridable in tests.
var (
	nowMillis = func() int64 { return time.Now().UnixMilli() }
	newID     = randomID
)

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should not fail; fall back to a timestamp-derived id.
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]), hex.EncodeToString(b[4:6]), hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]), hex.EncodeToString(b[10:16]))
}

func mimeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// persist stores one rendered image and returns the client payload.
func persist(st *store.Store, rd Rendered, meta store.ImageMeta) (Result, error) {
	created := nowMillis()
	filename := fmt.Sprintf("%d-%s%s", created, newID(), rd.Ext)
	if err := st.SaveImage(store.Image{
		Filename:  filename,
		Mime:      mimeForExt(rd.Ext),
		Bytes:     rd.Data,
		Prompt:    rd.Prompt,
		Model:     rd.Model,
		CreatedAt: created,
	}); err != nil {
		return Result{}, err
	}
	meta.Filename = filename
	meta.Prompt = rd.Prompt
	meta.Model = rd.Model
	meta.CreatedAt = created
	_ = st.UpsertImageMeta(meta)
	return Result{URL: "/generated/" + filename, Prompt: rd.Prompt, Model: rd.Model}, nil
}

// GenerateScene creates one scene illustration and stores it.
func GenerateScene(ctx context.Context, r Renderer, st *store.Store, input SceneInput) (Result, error) {
	rd, err := r.RenderScene(ctx, input)
	if err != nil {
		return Result{}, err
	}
	return persist(st, rd, store.ImageMeta{
		CampaignID:   input.CampaignID,
		Scene:        input.Scene,
		SourceSlotID: input.SourceSlotID,
	})
}

// GenerateCharacter creates one character portrait and stores it.
func GenerateCharacter(ctx context.Context, r Renderer, st *store.Store, input CharacterInput) (Result, error) {
	rd, err := r.RenderCharacter(ctx, input)
	if err != nil {
		return Result{}, err
	}
	return persist(st, rd, store.ImageMeta{
		CampaignID: input.CampaignID,
		Scene:      "portrait:" + input.Name,
	})
}

// ---------------------------------------------------------------------------
// Codex (GPT) renderer

// CodexRenderer generates images through the Codex CLI's built-in image_gen
// tool via provider.API.
type CodexRenderer struct {
	API provider.API
	CWD string
}

// NewCodexRenderer wraps a provider for image generation rooted at cwd.
func NewCodexRenderer(api provider.API, cwd string) *CodexRenderer {
	return &CodexRenderer{API: api, CWD: cwd}
}

// Model returns the Codex image-model label.
func (r *CodexRenderer) Model() string { return r.API.ImageModel() }

func (r *CodexRenderer) requireConfigured(ctx context.Context) error {
	status := r.API.Status(ctx)
	if status.Configured {
		return nil
	}
	if status.Message != "" {
		return errors.New(status.Message)
	}
	return errors.New("Codex CLI 尚未登入")
}

// run executes one image generation and reads the produced file.
func (r *CodexRenderer) run(ctx context.Context, prompt, emptyErr string) (Rendered, error) {
	log.Printf("[codex-image] start promptLen=%d cwd=%s", len(prompt), r.CWD)
	sourcePath, err := r.API.RunImageGeneration(ctx, prompt, provider.ImageOpts{CWD: r.CWD, Timeout: 420 * time.Second})
	if err != nil {
		log.Printf("[codex-image] RunImageGeneration failed: %v | tip: codex login；查 app-server image 連線或 exec 日誌；確認帳號有 image_gen", err)
		return Rendered{}, err
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		log.Printf("[codex-image] read output failed path=%s: %v", sourcePath, err)
		return Rendered{}, err
	}
	if len(data) == 0 {
		log.Printf("[codex-image] empty file path=%s | %s", sourcePath, emptyErr)
		return Rendered{}, errors.New(emptyErr)
	}
	log.Printf("[codex-image] ok path=%s bytes=%d", sourcePath, len(data))
	return Rendered{
		Data:   data,
		Ext:    strings.ToLower(filepath.Ext(sourcePath)),
		Prompt: prompt,
		Model:  r.API.ImageModel(),
	}, nil
}

// RenderScene sends the scene material straight to Codex image_gen — no separate
// prompt-engineering call. GPT handles Chinese narration and composition itself.
func (r *CodexRenderer) RenderScene(ctx context.Context, input SceneInput) (Rendered, error) {
	if err := r.requireConfigured(ctx); err != nil {
		return Rendered{}, err
	}

	var b strings.Builder
	b.WriteString("用 $imagegen / image_gen 直接畫一張 D&D 場景插圖（橫向建立鏡頭，約 3:2）。\n")
	b.WriteString("根據下面內容一次生成即可，不要先另外寫 SD prompt、不要多輪問答。\n")
	b.WriteString("畫面不要文字、UI、水印、骰子、角色卡。不要修改專案檔案。\n\n")
	if t := strings.TrimSpace(input.Title); t != "" {
		fmt.Fprintf(&b, "戰役：%s\n", t)
	}
	if s := strings.TrimSpace(input.Scene); s != "" {
		fmt.Fprintf(&b, "地點：%s\n", s)
	}
	if n := strings.TrimSpace(input.Narration); n != "" {
		fmt.Fprintf(&b, "敘事：%s\n", n)
	}
	// Prefer the ready English prompt when the DM already wrote one, but still
	// attach Chinese context so the model can fill gaps.
	if p := strings.TrimSpace(input.ImagePrompt); p != "" {
		fmt.Fprintf(&b, "補充視覺要點：%s\n", p)
	}
	if len(input.Players) > 0 {
		b.WriteString("在場人物：\n")
		for _, p := range input.Players {
			look := strings.TrimSpace(p.Appearance)
			if look == "" {
				look = defaultAppearance
			}
			fmt.Fprintf(&b, "- %s（%s %s）：%s\n",
				strings.TrimSpace(p.Name),
				strings.TrimSpace(p.Species),
				strings.TrimSpace(p.ClassName),
				look,
			)
		}
	}
	return r.run(ctx, strings.TrimSpace(b.String()), "Codex 圖片輸出是空檔案")
}

// RenderCharacter sends the character sheet fields straight to Codex image_gen.
func (r *CodexRenderer) RenderCharacter(ctx context.Context, input CharacterInput) (Rendered, error) {
	if err := r.requireConfigured(ctx); err != nil {
		return Rendered{}, err
	}

	var b strings.Builder
	b.WriteString("用 $imagegen / image_gen 直接畫一張 D&D 角色半身肖像（方形 1:1，單人、臉清楚）。\n")
	b.WriteString("根據下面描述一次生成即可，不要先另外寫 SD prompt、不要多輪問答。\n")
	b.WriteString("畫面不要文字、UI、水印、多人或骰子。不要修改專案檔案。\n\n")
	fmt.Fprintf(&b, "名字：%s\n", strings.TrimSpace(input.Name))
	fmt.Fprintf(&b, "種族：%s\n", strings.TrimSpace(input.Species))
	fmt.Fprintf(&b, "職業：%s\n", strings.TrimSpace(input.ClassName))
	if bg := strings.TrimSpace(input.Background); bg != "" {
		fmt.Fprintf(&b, "背景：%s\n", bg)
	}
	fmt.Fprintf(&b, "外觀：%s\n", strings.TrimSpace(input.Appearance))
	return r.run(ctx, strings.TrimSpace(b.String()), "Codex 角色圖片輸出是空檔案")
}
