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
	"net/http"
	"os"
	"strconv"
	"strings"
)

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

	HTTP *http.Client
}

// Presets bundle sampler settings per checkpoint family:
//   - quality:   standard SDXL checkpoints (e.g. JuggernautXL)
//   - lightning: SDXL Lightning distilled checkpoints (4–8 steps, low CFG)
const (
	PresetQuality   = "quality"
	PresetLightning = "lightning"
)

// NewClientFromEnv builds a Client from FORGE_* environment variables, applying
// the FORGE_PRESET defaults first and individual overrides on top.
func NewClientFromEnv() *Client {
	c := &Client{
		BaseURL:   strings.TrimRight(envOr("FORGE_URL", "http://127.0.0.1:7860"), "/"),
		Steps:     30,
		CFGScale:  5,
		Sampler:   "DPM++ 2M",
		Scheduler: "Karras",
		HTTP:      &http.Client{},
	}
	if strings.EqualFold(envOr("FORGE_PRESET", PresetQuality), PresetLightning) {
		c.Steps = 6
		c.CFGScale = 1.5
		c.Sampler = "Euler"
		c.Scheduler = "SGM Uniform"
	}
	c.Checkpoint = strings.TrimSpace(os.Getenv("FORGE_CHECKPOINT"))
	if v, err := strconv.Atoi(envOr("FORGE_STEPS", "")); err == nil && v > 0 {
		c.Steps = v
	}
	if v, err := strconv.ParseFloat(envOr("FORGE_CFG", ""), 64); err == nil && v > 0 {
		c.CFGScale = v
	}
	if v := envOr("FORGE_SAMPLER", ""); v != "" {
		c.Sampler = v
	}
	if v := envOr("FORGE_SCHEDULER", ""); v != "" {
		c.Scheduler = v
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
}

type txt2imgResponse struct {
	Images []string `json:"images"`
}

// GenerateImage runs txt2img and returns the decoded PNG bytes of the single
// generated image.
func (c *Client) GenerateImage(ctx context.Context, req Txt2Img) ([]byte, error) {
	payload := map[string]any{
		"prompt":          req.Prompt,
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
