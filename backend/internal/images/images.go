// Package images generates scene and character illustrations and stores the
// results as session files on disk (not SQLite). Two interchangeable renderers
// are provided: one through the Codex CLI's built-in image_gen tool (mirrors
// scene-image.mjs) and one through a local Stable Diffusion WebUI Forge server.
package images

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"dndduet/internal/forge"
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

// ForgeOptions are optional, validated per-request overrides. A nil pointer
// means the player left custom Forge controls disabled.
type ForgeOptions struct {
	PositivePrompt string
	NegativePrompt string
	Steps          int
	CFGScale       float64
	Sampler        string
	Scheduler      string
	Seed           *int64
	Width          int
	Height         int
}

// defaultAppearance anchors a party member whose player left appearance blank,
// so scene images don't render a wildly different figure each time.
// Must stay English: local SD CLIP is English-only.
const defaultAppearance = "weathered traveler wearing a cloak and leather armor"

// SceneInput describes a scene to illustrate.
type SceneInput struct {
	Title     string
	Scene     string
	Narration string
	// ImagePrompt is the DM agent's ready-made English SD prompt for this
	// scene. When set, the Forge renderer uses it directly and skips the
	// separate translation call.
	ImagePrompt string
	Players     []ScenePlayer
	Forge       *ForgeOptions
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
// Codex renderer

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

// ---------------------------------------------------------------------------
// Grok (xAI) renderer

// GrokImageClient is the subset of the Grok client used for image generation.
type GrokImageClient interface {
	ImageModel() string
	GenerateImageBytes(ctx context.Context, prompt, model string) ([]byte, string, error)
}

// GrokRenderer generates scene / portrait images via the xAI Imagine API.
type GrokRenderer struct {
	Client GrokImageClient
}

// NewGrokRenderer wraps a Grok client for image generation.
func NewGrokRenderer(c GrokImageClient) *GrokRenderer {
	return &GrokRenderer{Client: c}
}

func (r *GrokRenderer) Model() string {
	if r == nil || r.Client == nil {
		return "Grok Imagine"
	}
	return "Grok Imagine（" + r.Client.ImageModel() + "）"
}

func (r *GrokRenderer) RenderScene(ctx context.Context, input SceneInput) (Rendered, error) {
	if r == nil || r.Client == nil {
		return Rendered{}, errors.New("Grok 圖片後端未設定")
	}
	var b strings.Builder
	b.WriteString("Photorealistic D&D fantasy establishing shot, cinematic wide angle, no text, no UI, no watermark, no dice, no character sheets.\n")
	if t := strings.TrimSpace(input.Title); t != "" {
		fmt.Fprintf(&b, "Campaign: %s\n", t)
	}
	if s := strings.TrimSpace(input.Scene); s != "" {
		fmt.Fprintf(&b, "Location: %s\n", s)
	}
	if n := strings.TrimSpace(input.Narration); n != "" {
		fmt.Fprintf(&b, "Narration: %s\n", truncateRunes(n, 600))
	}
	if p := strings.TrimSpace(input.ImagePrompt); p != "" {
		fmt.Fprintf(&b, "Visual: %s\n", truncateRunes(p, 500))
	}
	if len(input.Players) > 0 {
		b.WriteString("Party present:\n")
		for _, p := range input.Players {
			look := strings.TrimSpace(p.Appearance)
			if look == "" {
				look = defaultAppearance
			}
			fmt.Fprintf(&b, "- %s (%s %s): %s\n",
				strings.TrimSpace(p.Name), strings.TrimSpace(p.Species), strings.TrimSpace(p.ClassName), look)
		}
	}
	prompt := strings.TrimSpace(b.String())
	log.Printf("[grok-image] scene start promptLen=%d", len(prompt))
	data, ext, err := r.Client.GenerateImageBytes(ctx, prompt, "")
	if err != nil {
		log.Printf("[grok-image] scene failed: %v | tip: 檢查 XAI_API_KEY、GROK_IMAGE_MODEL、帳號是否有 Imagine 權限", err)
		return Rendered{}, err
	}
	log.Printf("[grok-image] scene ok bytes=%d ext=%s", len(data), ext)
	return Rendered{Data: data, Ext: ext, Prompt: prompt, Model: r.Model()}, nil
}

func (r *GrokRenderer) RenderCharacter(ctx context.Context, input CharacterInput) (Rendered, error) {
	if r == nil || r.Client == nil {
		return Rendered{}, errors.New("Grok 圖片後端未設定")
	}
	var b strings.Builder
	b.WriteString("Photorealistic fantasy character portrait, single character, waist-up, face clearly visible, square crop, no text, no UI, no watermark.\n")
	fmt.Fprintf(&b, "Name: %s\n", strings.TrimSpace(input.Name))
	fmt.Fprintf(&b, "Species: %s\n", strings.TrimSpace(input.Species))
	fmt.Fprintf(&b, "Class: %s\n", strings.TrimSpace(input.ClassName))
	if bg := strings.TrimSpace(input.Background); bg != "" {
		fmt.Fprintf(&b, "Background: %s\n", bg)
	}
	fmt.Fprintf(&b, "Appearance: %s\n", strings.TrimSpace(input.Appearance))
	prompt := strings.TrimSpace(b.String())
	log.Printf("[grok-image] portrait start name=%q", input.Name)
	data, ext, err := r.Client.GenerateImageBytes(ctx, prompt, "")
	if err != nil {
		log.Printf("[grok-image] portrait failed: %v", err)
		return Rendered{}, err
	}
	return Rendered{Data: data, Ext: ext, Prompt: prompt, Model: r.Model()}, nil
}

// ---------------------------------------------------------------------------
// Forge renderer

// ForgeRenderer generates images on a local Stable Diffusion WebUI Forge
// server. SDXL checkpoints are trained on English tags, so the prompt is an
// English descriptor list; well-known Chinese class/species names are mapped
// via englishTerms, and free-text fields (scene, narration, appearance) go
// through Translator when configured — CLIP mostly ignores untranslated
// Chinese sentences, which otherwise makes generated images drift from the
// actual story.
type ForgeRenderer struct {
	Client     *forge.Client
	Translator *PromptTranslator
}

// NewForgeRenderer wraps a Forge client. translator may be nil, in which case
// free-text fields are passed through untranslated (the pre-translation
// behavior).
func NewForgeRenderer(c *forge.Client, translator *PromptTranslator) *ForgeRenderer {
	return &ForgeRenderer{Client: c, Translator: translator}
}

// PromptTranslator condenses Chinese story text into English SD visual tags
// via Codex, so local SDXL checkpoints (English-only CLIP) actually respond
// to it.
type PromptTranslator struct {
	API        provider.API
	CWD        string
	SchemaPath string
	// Effort is the reasoning-effort id for the translation call; this is a
	// small task, so a light effort keeps image generation latency down.
	Effort string
}

type imagePromptTags struct {
	Tags string `json:"tags"`
}

// logPrompts mirrors the LOG_PROMPTS toggle used by the dm/forge packages;
// read at call time since .env loads in main() after package init.
func logPrompts() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_PROMPTS"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// translate turns Chinese (or mixed) scene text into English SD visual tags via
// the AI agent. On any failure it returns "" — callers must use englishFallback
// rather than sending Chinese to the local model.
func (t *PromptTranslator) translate(ctx context.Context, zh string) string {
	zh = strings.TrimSpace(zh)
	if t == nil || zh == "" {
		return ""
	}
	prompt := "You are a Stable Diffusion prompt engineer. Convert the following scene text into ONE English SD prompt for a photorealistic illustration. Requirements: cover location, architecture, environment, lighting/time, mood, key props, characters' race/class/appearance, and actions; comma-separated concrete English visual tags (~20–40 words), subject first then details; omit personal names (do not transliterate them); no explanations, no Chinese, no other non-English scripts — tags field only.\n\nScene text:\n" + truncateRunes(zh, 800)
	raw, err := t.API.RunStructured(ctx, prompt, provider.StructuredOpts{
		CWD:        t.CWD,
		SchemaPath: t.SchemaPath,
		Effort:     t.Effort,
		Timeout:    120 * time.Second,
	})
	if err != nil {
		log.Printf("[translate] Codex structured call failed: %v | input=%q | tip: 確認 codex 可用且 schema 路徑有效；將使用詞典/剝中文 fallback（不送中文給 SD）",
			err, truncateRunes(zh, 80))
		return ""
	}
	var parsed imagePromptTags
	if err := json.Unmarshal(raw, &parsed); err != nil || strings.TrimSpace(parsed.Tags) == "" {
		log.Printf("[translate] empty/invalid tags raw=%q err=%v | input=%q | tip: 檢查 image-prompt schema 與模型輸出",
			truncateRunes(string(raw), 120), err, truncateRunes(zh, 80))
		return ""
	}
	tags := strings.TrimSpace(parsed.Tags)
	if hasCJK(tags) {
		log.Printf("[translate] model returned CJK in tags=%q | input=%q | tip: 模型未遵守全英文；改用 fallback",
			truncateRunes(tags, 80), truncateRunes(zh, 80))
		return ""
	}
	log.Printf("[translate] ok %q -> %q", truncateRunes(zh, 80), truncateRunes(tags, 120))
	return tags
}

// englishVisual guarantees an English SD fragment for local Forge. Already-
// English text passes through; Chinese is translated when a Translator is
// available, otherwise mapped/stripped via englishFallback. Never returns CJK.
func (r *ForgeRenderer) englishVisual(ctx context.Context, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if en, ok := englishTerms[text]; ok {
		return en
	}
	if !hasCJK(text) {
		return text
	}
	if r != nil && r.Translator != nil {
		if out := r.Translator.translate(ctx, text); out != "" {
			return out
		}
		log.Printf("[translate] translator produced no English; dictionary/strip fallback input=%q", truncateRunes(text, 80))
	} else {
		log.Printf("[translate] no PromptTranslator configured; dictionary/strip fallback input=%q | tip: app-server 模式需可用的 Codex 才能做中→英", truncateRunes(text, 80))
	}
	out := englishFallback(text)
	log.Printf("[translate] fallback result=%q", truncateRunes(out, 120))
	return out
}

func hasCJK(s string) bool {
	for _, r := range s {
		if isCJKRune(r) {
			return true
		}
	}
	return false
}

func isCJKRune(r rune) bool {
	if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
		return true
	}
	// CJK punctuation / fullwidth forms that often leak from zh narration.
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	if r >= 0xFF00 && r <= 0xFFEF {
		return true
	}
	return false
}

var multiSpace = regexp.MustCompile(`\s+`)

func stripCJK(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isCJKRune(r) {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(multiSpace.ReplaceAllString(b.String(), " "))
}

// englishFallback maps known game terms then strips residual CJK so Forge
// never receives Chinese when the translator is offline.
func englishFallback(s string) string {
	s = applyEnglishTerms(s)
	s = stripCJK(s)
	s = strings.Trim(s, ",;: ")
	if s == "" {
		return "fantasy scene, detailed environment"
	}
	return s
}

func applyEnglishTerms(s string) string {
	if s == "" || len(englishTerms) == 0 {
		return s
	}
	keys := make([]string, 0, len(englishTerms))
	for k := range englishTerms {
		keys = append(keys, k)
	}
	// Longer keys first so e.g. 山地矮人 beats 矮人.
	sort.Slice(keys, func(i, j int) bool { return len([]rune(keys[i])) > len([]rune(keys[j])) })
	for _, k := range keys {
		if strings.Contains(s, k) {
			s = strings.ReplaceAll(s, k, englishTerms[k])
		}
	}
	return s
}

// Model returns the Forge backend label.
func (r *ForgeRenderer) Model() string { return r.Client.ModelLabel() }

// SceneDefaults exposes the active server preset to the local-only UI.
func (r *ForgeRenderer) SceneDefaults() ForgeOptions {
	seed := int64(-1)
	cfg := r.Client.CFGScale
	if cfg <= 1 {
		// The built-in scene negative must remain active even for Turbo presets.
		cfg = 2
	}
	return ForgeOptions{
		NegativePrompt: sceneNegative,
		Steps:          r.Client.Steps, CFGScale: cfg,
		Sampler: r.Client.Sampler, Scheduler: r.Client.Scheduler, Seed: &seed,
		Width: r.Client.SceneWidth, Height: r.Client.SceneHeight,
	}
}

// englishTerms maps class/species/background labels the UI ships with (and
// common scene nouns) to English tags CLIP understands.
var englishTerms = map[string]string{
	"法師": "wizard", "戰士": "fighter", "盜賊": "rogue", "遊俠": "ranger",
	"牧師": "cleric", "聖騎士": "paladin", "野蠻人": "barbarian", "吟遊詩人": "bard",
	"德魯伊": "druid", "武僧": "monk", "術士": "sorcerer", "邪術師": "warlock",
	"魔契師": "warlock", "旅人": "traveler", "獵人": "hunter", "嚮導": "guide",
	"人類": "human", "精靈": "elf", "半精靈": "half-elf", "木精靈": "wood elf",
	"高等精靈": "high elf", "卓爾": "drow", "矮人": "dwarf", "山地矮人": "mountain dwarf",
	"半身人": "halfling", "龍裔": "dragonborn", "提夫林": "tiefling",
	"侏儒": "gnome", "半獸人": "half-orc", "獸人": "orc",
	"灰斗篷": "gray cloak", "披風": "cloak", "皮甲": "leather armor",
	"鐘樓": "bell tower", "禮拜堂": "chapel", "廢墟": "ruins", "廢棄禮拜堂": "ruined chapel",
	"森林": "forest", "地城": "dungeon", "村莊": "village", "洞穴": "cave",
	"燭火": "candlelight", "火把": "torchlight", "月光": "moonlight",
}

func englishTerm(s string) string {
	trimmed := strings.TrimSpace(s)
	if en, ok := englishTerms[trimmed]; ok {
		return en
	}
	// Species/class may still contain CJK when custom; never pass that to Forge.
	if hasCJK(trimmed) {
		return englishFallback(trimmed)
	}
	return trimmed
}

func truncateRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

func joinNonEmpty(parts []string, sep string) string {
	kept := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

// Negatives steer toward photorealism (banning illustration/anime/3d looks)
// and away from artefacts. "crowd, extra people, duplicate" curb SDXL's
// tendency to spawn stray figures the story never mentioned. The realistic*
// prefix is shared; scene/portrait add their own framing bans.
const (
	realisticNegative = "cartoon, anime, illustration, painting, drawing, sketch, cel shading, " +
		"concept art, 3d render, cgi, video game screenshot, oversaturated, " +
		"text, watermark, logo, signature, border, frame, UI, dice, character sheet, " +
		"low quality, blurry, jpeg artifacts, deformed, extra limbs"
	sceneNegative = realisticNegative + ", closeup portrait, crowd, extra people, " +
		"duplicate people, cloned figures"
	portraitNegative = realisticNegative + ", multiple people, two people, group, crowd, " +
		"full body, bad anatomy, bad hands, extra fingers"
)

// realisticSceneStyle / realisticPortraitStyle intentionally omit any fixed
// colour/lighting palette: the scene's own lighting words (from the
// translated text) must win, otherwise a hardcoded "amber palette" paints
// sunlight into caves the story says are pitch dark.
const (
	realisticSceneStyle    = "photorealistic, cinematic film still, realistic materials and textures, natural lighting consistent with the scene, atmospheric depth, sharp focus, highly detailed"
	realisticPortraitStyle = "photorealistic, cinematic portrait photography, realistic skin and fabric texture, lighting consistent with the setting, sharp focus, highly detailed"
)

// forgeScenePrompt builds the SD positive/negative prompt pair for a scene.
// Every free-text fragment is forced through englishVisual so the local model
// only ever sees English tags (CLIP is English-only).
func (r *ForgeRenderer) sceneCharacterAppearance(ctx context.Context, players []ScenePlayer) string {
	if len(players) == 0 {
		return ""
	}
	ordinals := []string{"first adventurer", "second adventurer", "third adventurer", "fourth adventurer"}
	members := make([]string, 0, len(players))
	for i, p := range players {
		look := strings.TrimSpace(p.Appearance)
		if look == "" {
			look = defaultAppearance
		}
		look = r.englishVisual(ctx, look)
		role := strings.TrimSpace(strings.Join([]string{englishTerm(p.Species), englishTerm(p.ClassName)}, " "))
		label := "adventurer"
		if i < len(ordinals) {
			label = ordinals[i]
		}
		members = append(members, label+": "+joinNonEmpty([]string{role, truncateRunes(look, 240)}, ", "))
	}
	plural := ""
	if len(players) != 1 {
		plural = "s"
	}
	return fmt.Sprintf("exactly %d adventurer%s visible; character appearance continuity: %s",
		len(players), plural, strings.Join(members, "; "))
}

func (r *ForgeRenderer) forgeScenePrompt(ctx context.Context, input SceneInput) (string, string) {
	appearance := r.sceneCharacterAppearance(ctx, input.Players)
	if input.Forge != nil && len(strings.TrimSpace(input.Forge.PositivePrompt)) > 0 {
		custom := r.englishVisual(ctx, strings.TrimSpace(input.Forge.PositivePrompt))
		positive := forceEnglishPrompt(joinNonEmpty([]string{custom, truncateRunes(appearance, 500)}, ", "))
		neg := input.Forge.NegativePrompt
		if hasCJK(neg) {
			neg = sceneNegative
		}
		return positive, neg
	}
	visual := strings.TrimSpace(input.ImagePrompt)
	if visual == "" {
		var members []string
		for _, p := range input.Players {
			look := strings.TrimSpace(p.Appearance)
			if look == "" {
				look = defaultAppearance
			}
			desc := joinNonEmpty([]string{englishTerm(p.Species), englishTerm(p.ClassName)}, " ")
			members = append(members, joinNonEmpty([]string{desc, r.englishVisual(ctx, look)}, ", "))
		}
		party := ""
		if len(members) > 0 {
			party = fmt.Sprintf("%d adventurers visible: %s", len(members), strings.Join(members, "; "))
		}
		// Prefer one translation of the full Chinese scene when needed.
		sceneText := joinNonEmpty([]string{input.Scene, input.Narration, party}, ". ")
		visual = r.englishVisual(ctx, sceneText)
	} else {
		visual = r.englishVisual(ctx, visual)
	}
	positive := forceEnglishPrompt(joinNonEmpty([]string{
		"photorealistic fantasy environment, wide establishing shot",
		truncateRunes(appearance, 500),
		truncateRunes(visual, 500),
		realisticSceneStyle,
	}, ", "))
	negative := sceneNegative
	if input.Forge != nil {
		negative = input.Forge.NegativePrompt
		if hasCJK(negative) {
			negative = sceneNegative
		}
	}
	return positive, negative
}

// forgeCharacterPrompt builds the SD positive/negative prompt pair for a
// portrait. All free text is forced to English before reaching Forge.
func (r *ForgeRenderer) forgeCharacterPrompt(ctx context.Context, input CharacterInput) (string, string) {
	visualText := joinNonEmpty([]string{input.Background, input.Appearance}, ". ")
	visual := r.englishVisual(ctx, visualText)
	positive := forceEnglishPrompt(joinNonEmpty([]string{
		"photorealistic fantasy character portrait, single character, waist-up, centered, face clearly visible",
		strings.TrimSpace(englishTerm(input.Species) + " " + englishTerm(input.ClassName)),
		truncateRunes(visual, 500),
		realisticPortraitStyle,
	}, ", "))
	return positive, portraitNegative
}

// forceEnglishPrompt is the last gate before Forge: strip any residual CJK and
// guarantee a non-empty English prompt.
func forceEnglishPrompt(positive string) string {
	if !hasCJK(positive) {
		if strings.TrimSpace(positive) == "" {
			log.Printf("[forge] empty positive prompt; using generic English fallback")
			return "photorealistic fantasy scene, highly detailed"
		}
		return positive
	}
	log.Printf("[forge] stripping residual CJK from SD prompt before send: %q", truncateRunes(positive, 160))
	cleaned := stripCJK(positive)
	cleaned = strings.Trim(cleaned, ",;: ")
	if cleaned == "" {
		log.Printf("[forge] prompt empty after CJK strip; using generic English fallback")
		return "photorealistic fantasy scene, highly detailed"
	}
	return cleaned
}

func (r *ForgeRenderer) render(ctx context.Context, positive, negative string, width, height int, options *ForgeOptions) (Rendered, error) {
	positive = forceEnglishPrompt(positive)
	if hasCJK(negative) {
		log.Printf("[forge] negative prompt contained CJK; replaced with default English negative")
		negative = sceneNegative
	}
	// Always log the final English prompt on the error-diagnosis path (truncated).
	// Full dump still requires LOG_PROMPTS=1 in forge.Client.
	log.Printf("[forge] txt2img %dx%d model=%q positive=%q", width, height, r.Client.ModelLabel(), truncateRunes(positive, 220))
	req := forge.Txt2Img{
		Prompt:         positive,
		NegativePrompt: negative,
		Width:          width,
		Height:         height,
	}
	if options != nil {
		req.Steps = options.Steps
		req.CFGScale = options.CFGScale
		req.Sampler = options.Sampler
		req.Scheduler = options.Scheduler
		req.Seed = options.Seed
	}
	data, err := r.Client.GenerateImage(ctx, req)
	if err != nil {
		log.Printf("[forge] GenerateImage failed model=%q url=%s size=%dx%d: %v | tip: 啟動 WebUI Forge 並加 --api；檢查 FORGE_BASE_URL=%s；CUDA OOM 時降寬高/steps",
			r.Client.ModelLabel(), r.Client.BaseURL, width, height, err, r.Client.BaseURL)
		return Rendered{}, err
	}
	log.Printf("[forge] ok model=%q bytes=%d", r.Client.ModelLabel(), len(data))
	return Rendered{
		Data:   data,
		Ext:    ".png",
		Prompt: positive + "\nNegative prompt: " + negative,
		Model:  r.Client.ModelLabel(),
	}, nil
}

// RenderScene generates a 3:2 environment shot on the local GPU.
func (r *ForgeRenderer) RenderScene(ctx context.Context, input SceneInput) (Rendered, error) {
	positive, negative := r.forgeScenePrompt(ctx, input)
	width, height := r.Client.SceneWidth, r.Client.SceneHeight
	if input.Forge != nil {
		width, height = input.Forge.Width, input.Forge.Height
	}
	return r.render(ctx, positive, negative, width, height, input.Forge)
}

// RenderCharacter generates a square waist-up portrait on the local GPU.
func (r *ForgeRenderer) RenderCharacter(ctx context.Context, input CharacterInput) (Rendered, error) {
	positive, negative := r.forgeCharacterPrompt(ctx, input)
	return r.render(ctx, positive, negative, r.Client.PortraitWidth, r.Client.PortraitHeight, nil)
}
