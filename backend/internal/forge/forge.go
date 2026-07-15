// Package forge talks to a local Stable Diffusion WebUI Forge (or AUTOMATIC1111
// compatible) server over its /sdapi/v1 REST API, so illustrations can be
// generated on the local GPU without any cloud image model. Start Forge with
// the --api flag to expose the endpoint.
package forge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// logPrompts reports whether LOG_PROMPTS is truthy. Read at call time (not
// package-init) because .env is loaded inside main(), after package vars
// would otherwise have already captured an empty value.
func logPrompts() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_PROMPTS"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// Client holds the connection and generation parameters for one Forge server.
type Client struct {
	// BaseURL is the Forge server root, e.g. http://127.0.0.1:7860.
	BaseURL string
	// Checkpoint optionally forces a model via override_settings; empty uses
	// whichever checkpoint the server currently has loaded.
	Checkpoint string
	Steps      int
	CFGScale   float64
	Sampler    string
	Scheduler  string

	// SceneWidth/SceneHeight and PortraitWidth/PortraitHeight are the
	// generation resolutions, sized to the checkpoint family's native
	// resolution (SDXL vs SD-Turbo's 512x512).
	SceneWidth     int
	SceneHeight    int
	PortraitWidth  int
	PortraitHeight int

	// Lora is an optional prompt fragment prepended to every positive prompt,
	// e.g. "<lora:Hyper-SD15-8steps-lora:1>" to turn a normal SD1.5 checkpoint
	// (epiCRealism) into a few-step model. Chain more LoRAs by concatenating
	// tags, e.g. "...:1>, <lora:add_detail:1>". Empty means no LoRA.
	Lora string

	// ADetailerModel, when set (e.g. "face_yolov8n.pt"), runs the ADetailer
	// extension as an always-on script to detect and re-inpaint faces at
	// higher fidelity. Requires the adetailer extension installed in Forge.
	ADetailerModel string

	HTTP *http.Client
}

// Presets bundle sampler settings and native resolution per checkpoint family:
//   - quality: standard SDXL checkpoints (e.g. JuggernautXL)
//   - turbo:   SD-Turbo (single-step distillation of SD 2.1), low VRAM —
//     for running alongside other GPU workloads (e.g. TTS)
//   - hyper:   an SD1.5 photoreal checkpoint (e.g. epiCRealism) driven by a
//     Hyper-SD LoRA (set FORGE_LORA); 6 steps, low CFG, 512x512 — the
//     lowest-VRAM realistic option, comfortable alongside GPU TTS on 8 GB
const (
	PresetQuality = "quality"
	PresetTurbo   = "turbo"
	PresetHyper   = "hyper"
)

// NewClientFromEnv builds a Client from FORGE_* environment variables, applying
// the FORGE_PRESET defaults first and individual overrides on top.
func NewClientFromEnv() *Client {
	return newClientFromEnv("")
}

// NewClientFromEnvVariant builds a second local backend from
// FORGE_CHECKPOINT_<suffix>/FORGE_PRESET_<suffix>/etc (sharing FORGE_URL), so
// two checkpoints — e.g. a quality one and a fast turbo one — can be offered
// side by side. Returns nil if FORGE_CHECKPOINT_<suffix> is unset.
func NewClientFromEnvVariant(suffix string) *Client {
	if strings.TrimSpace(os.Getenv("FORGE_CHECKPOINT_"+suffix)) == "" {
		return nil
	}
	return newClientFromEnv("_" + suffix)
}

func newClientFromEnv(suffix string) *Client {
	c := &Client{
		BaseURL:        strings.TrimRight(envOr("FORGE_URL", "http://127.0.0.1:7860"), "/"),
		Steps:          30,
		CFGScale:       5,
		Sampler:        "DPM++ 2M",
		Scheduler:      "Karras",
		SceneWidth:     1216,
		SceneHeight:    832,
		PortraitWidth:  1024,
		PortraitHeight: 1024,
		HTTP:           &http.Client{},
	}
	switch strings.ToLower(envOr("FORGE_PRESET"+suffix, PresetQuality)) {
	case PresetTurbo:
		c.Steps = 1
		// Keep classifier-free guidance active so negative prompts are not
		// silently ignored. Forge skips the unconditional pass at CFG 1.
		c.CFGScale = 2
		c.Sampler = "Euler a"
		c.Scheduler = "Automatic"
		c.SceneWidth = 768
		c.SceneHeight = 512
		c.PortraitWidth = 512
		c.PortraitHeight = 512
	case PresetHyper:
		// SD1.5 + Hyper-SD 8-step LoRA. ByteDance's official settings for the
		// step-specific LoRAs are 8 steps, guidance_scale=0 and DDIM with
		// timestep_spacing='trailing' (Forge maps trailing onto "SGM Uniform").
		// We deviate on CFG: the official CFG 1 makes Forge skip the
		// unconditional pass, which silently ignores the negative prompt — and
		// we rely on negatives for photorealism and people-count control. CFG 2
		// keeps negatives active; Hyper-SD 8-step tolerates it well.
		// 512 is SD1.5's native size; scene/portrait use aspect-correct 512.
		c.Steps = 8
		c.CFGScale = 2
		c.Sampler = "DDIM"
		c.Scheduler = "SGM Uniform"
		// Scene 3:2 (768x512); portrait 2:3 (512x768) — SD1.5's classic
		// portrait size, better for faces/waist-up framing. The UI portrait
		// box is square with object-fit:contain, so the taller image shows in
		// full (letterboxed) rather than cropped.
		c.SceneWidth = 768
		c.SceneHeight = 512
		c.PortraitWidth = 512
		c.PortraitHeight = 768
	}
	c.Checkpoint = strings.TrimSpace(os.Getenv("FORGE_CHECKPOINT" + suffix))
	c.Lora = strings.TrimSpace(os.Getenv("FORGE_LORA" + suffix))
	c.ADetailerModel = strings.TrimSpace(os.Getenv("FORGE_ADETAILER" + suffix))
	if v, err := strconv.Atoi(envOr("FORGE_STEPS"+suffix, "")); err == nil && v > 0 {
		c.Steps = v
	}
	if v, err := strconv.ParseFloat(envOr("FORGE_CFG"+suffix, ""), 64); err == nil && v > 0 {
		c.CFGScale = v
	}
	if v := envOr("FORGE_SAMPLER"+suffix, ""); v != "" {
		c.Sampler = v
	}
	if v := envOr("FORGE_SCHEDULER"+suffix, ""); v != "" {
		c.Scheduler = v
	}
	if v, err := strconv.Atoi(envOr("FORGE_SCENE_WIDTH"+suffix, "")); err == nil && v > 0 {
		c.SceneWidth = v
	}
	if v, err := strconv.Atoi(envOr("FORGE_SCENE_HEIGHT"+suffix, "")); err == nil && v > 0 {
		c.SceneHeight = v
	}
	return c
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// ModelLabel is the human-readable label shown for this backend.
func (c *Client) ModelLabel() string {
	if c.Checkpoint != "" {
		return "SD Forge（" + c.Checkpoint + "）"
	}
	return "SD Forge（本地 Stable Diffusion）"
}

// Txt2Img describes one text-to-image request; sampler settings come from the
// Client so every image uses the operator-configured quality profile.
type Txt2Img struct {
	Prompt         string
	NegativePrompt string
	Width          int
	Height         int
	// Per-request overrides come from the optional player-facing Forge panel.
	Steps     int
	CFGScale  float64
	Sampler   string
	Scheduler string
	Seed      *int64
}

type txt2imgResponse struct {
	Images []string `json:"images"`
}

// GenerateImage runs txt2img and returns the decoded PNG bytes of the single
// generated image.
func (c *Client) GenerateImage(ctx context.Context, req Txt2Img) ([]byte, error) {
	prompt := req.Prompt
	if c.Lora != "" {
		// LoRA tags must live in the prompt; Forge parses and strips them
		// before text conditioning, so their position doesn't affect the image.
		prompt = c.Lora + ", " + prompt
	}
	steps := c.Steps
	if req.Steps > 0 {
		steps = req.Steps
	}
	cfgScale := c.CFGScale
	if req.CFGScale > 0 {
		cfgScale = req.CFGScale
	}
	sampler := c.Sampler
	if len(strings.TrimSpace(req.Sampler)) > 0 {
		sampler = strings.TrimSpace(req.Sampler)
	}
	scheduler := c.Scheduler
	if len(strings.TrimSpace(req.Scheduler)) > 0 {
		scheduler = strings.TrimSpace(req.Scheduler)
	}
	seed := int64(-1)
	if req.Seed != nil {
		seed = *req.Seed
	}
	// Forge skips classifier-free guidance at CFG 1. Never emit that value:
	// it makes negative prompts inert and is too easy to reintroduce via a
	// preset, environment override, or player request.
	if cfgScale <= 1 {
		cfgScale = 2
	}
	if logPrompts() {
		log.Printf("[forge] %dx%d steps=%d cfg=%.1f %s/%s\n  positive: %s\n  negative: %s",
			req.Width, req.Height, steps, cfgScale, sampler, scheduler, prompt, req.NegativePrompt)
	}
	payload := map[string]any{
		"prompt":          prompt,
		"negative_prompt": req.NegativePrompt,
		"width":           req.Width,
		"height":          req.Height,
		"steps":           c.Steps,
		"cfg_scale":       c.CFGScale,
		"sampler_name":    c.Sampler,
		"scheduler":       c.Scheduler,
		"seed":            -1,
		"batch_size":      1,
		"n_iter":          1,
	}
	if c.Checkpoint != "" {
		payload["override_settings"] = map[string]any{"sd_model_checkpoint": c.Checkpoint}
		// Keep the checkpoint loaded between requests instead of swapping back.
		payload["override_settings_restore_afterwards"] = false
	}
	if c.ADetailerModel != "" {
		// ADetailer always-on script: [ad_enable, skip_img2img, unit...]. It
		// re-inpaints detected faces at higher fidelity after the main pass.
		payload["alwayson_scripts"] = map[string]any{
			"ADetailer": map[string]any{
				"args": []any{
					true,
					false,
					map[string]any{
						"ad_model":              c.ADetailerModel,
						"ad_denoising_strength": 0.4,
						"ad_confidence":         0.3,
					},
				},
			},
		}
	}
	// Apply player overrides last so the emitted payload and prompt log agree.
	payload[`steps`] = steps
	payload[`cfg_scale`] = cfgScale
	payload[`sampler_name`] = sampler
	payload[`scheduler`] = scheduler
	payload[`seed`] = seed
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/sdapi/v1/txt2img", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("content-type", "application/json")

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("無法連線本地 SD Forge（%s）；請先啟動 Forge 並加上 --api 參數", c.BaseURL)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		detail := strings.TrimSpace(string(data))
		if len(detail) > 300 {
			detail = detail[:300]
		}
		return nil, fmt.Errorf("SD Forge 回應錯誤（HTTP %d）：%s", resp.StatusCode, detail)
	}

	var parsed txt2imgResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, errors.New("SD Forge 沒有回傳有效的 JSON 結果")
	}
	if len(parsed.Images) == 0 || strings.TrimSpace(parsed.Images[0]) == "" {
		return nil, errors.New("SD Forge 沒有產生圖片")
	}

	encoded := parsed.Images[0]
	// Some servers prefix a data URI; strip it before decoding.
	if i := strings.IndexByte(encoded, ','); i >= 0 && strings.HasPrefix(encoded, "data:") {
		encoded = encoded[i+1:]
	}
	img, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("SD Forge 回傳的圖片無法解碼")
	}
	if len(img) == 0 {
		return nil, errors.New("SD Forge 圖片輸出是空檔案")
	}
	return img, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
