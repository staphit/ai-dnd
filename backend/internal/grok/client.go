// Package grok implements the DM provider and image generation against the
// xAI Grok HTTP API (OpenAI-compatible base URL https://api.x.ai/v1).
package grok

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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dndduet/internal/provider"
)

const (
	defaultBaseURL     = "https://api.x.ai/v1"
	defaultChatModel   = "grok-4.5"
	defaultImageModel  = "grok-imagine-image-quality"
	defaultHTTPTimeout = 180 * time.Second
)

// ModelOptions — this app only uses Grok 4.5 for DM turns.
var ModelOptions = []provider.ModelOption{
	{ID: "", Label: "Grok 4.5"},
	{ID: "grok-4.5", Label: "Grok 4.5"},
}

// EffortOptions — Grok API does not expose Codex-style effort knobs; keep a
// single default so the frontend selector still works.
var EffortOptions = []provider.ModelOption{
	{ID: "", Label: "Grok 預設"},
}

// chatMsg is one multi-turn message retained for Grok HTTP sessions.
type chatMsg struct {
	Role    string
	Content string
}

// storyChat keeps recent user/assistant turns so later requests can use a
// compact rules system prompt without re-attaching the full preamble.
type storyChat struct {
	turns []chatMsg // user/assistant pairs only (no system)
}

// maxChatTurns is the number of recent user+assistant messages kept (3 exchanges).
const maxChatTurns = 6

// Client talks to api.x.ai.
type Client struct {
	APIKey               string
	BaseURL              string
	ChatModel            string
	ConfiguredImageModel string
	HTTPClient           *http.Client
	mu                   sync.Mutex
	boundStory           string
	boundAlive           bool
	// chats is per-story multi-turn history for DM turns (cleared on Connect).
	chats map[string]*storyChat
}

// NewClientFromEnv builds a Client when XAI_API_KEY is set; returns nil otherwise.
func NewClientFromEnv() *Client {
	key := strings.TrimSpace(os.Getenv("XAI_API_KEY"))
	if key == "" {
		// Also accept GROK_API_KEY as alias.
		key = strings.TrimSpace(os.Getenv("GROK_API_KEY"))
	}
	if key == "" {
		return nil
	}
	base := strings.TrimSpace(os.Getenv("XAI_BASE_URL"))
	if base == "" {
		base = defaultBaseURL
	}
	base = strings.TrimRight(base, "/")
	chat := resolveChatModel(os.Getenv("GROK_MODEL"))
	img := strings.TrimSpace(os.Getenv("GROK_IMAGE_MODEL"))
	if img == "" {
		img = defaultImageModel
	}
	return &Client{
		APIKey:               key,
		BaseURL:              base,
		ChatModel:            chat,
		ConfiguredImageModel: img,
		HTTPClient:           &http.Client{Timeout: defaultHTTPTimeout},
		chats:                map[string]*storyChat{},
	}
}

var _ provider.API = (*Client)(nil)

func (c *Client) Status(ctx context.Context) provider.Status {
	if c == nil || c.APIKey == "" {
		return provider.Status{
			Configured: false,
			Provider:   "Grok (xAI)",
			Model:      c.safeModel(),
			Message:    "未設定 XAI_API_KEY（或 GROK_API_KEY）",
		}
	}
	return provider.Status{
		Configured: true,
		Provider:   "Grok (xAI API)",
		Model:      c.Model(),
	}
}

func (c *Client) safeModel() string {
	if c == nil {
		return defaultChatModel
	}
	return c.Model()
}

func (c *Client) Model() string {
	if c.ChatModel != "" {
		return c.ChatModel
	}
	return defaultChatModel
}

func (c *Client) ModelOptions() []provider.ModelOption { return ModelOptions }

func (c *Client) ImageModel() string {
	if c.ConfiguredImageModel != "" {
		return c.ConfiguredImageModel
	}
	return defaultImageModel
}

func (c *Client) EffortOptions() []provider.ModelOption { return EffortOptions }

func (c *Client) NormalizeModel(value string) (string, error) {
	model := strings.TrimSpace(value)
	if model == "" {
		if c.ChatModel != "" {
			return resolveChatModel(c.ChatModel), nil
		}
		return defaultChatModel, nil
	}
	if model == "grok-4.5" || model == "4.5" {
		return defaultChatModel, nil
	}
	// Older saved campaign settings may still request other Grok ids — pin to 4.5.
	if strings.HasPrefix(model, "grok-") {
		return defaultChatModel, nil
	}
	return "", errors.New("Grok 僅支援 grok-4.5")
}

// resolveChatModel always returns grok-4.5 (the only supported DM model).
func resolveChatModel(raw string) string {
	_ = raw
	return defaultChatModel
}

func (c *Client) NormalizeEffort(value string) (string, error) {
	// Grok has no effort channel; accept empty / ignore unknown quietly.
	return "", nil
}

// Connect binds the story id and clears multi-turn history for a fresh session.
func (c *Client) Connect(ctx context.Context, storyID string) error {
	if c.APIKey == "" {
		return errors.New("未設定 XAI_API_KEY")
	}
	storyID = strings.TrimSpace(storyID)
	if storyID == "" {
		return errors.New("缺少 campaignId")
	}
	c.mu.Lock()
	c.boundStory = storyID
	c.boundAlive = true
	if c.chats == nil {
		c.chats = map[string]*storyChat{}
	}
	delete(c.chats, storyID)
	c.mu.Unlock()
	return nil
}

func (c *Client) ConnectionState() provider.ConnState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return provider.ConnState{Alive: c.boundAlive && c.boundStory != "", StoryID: c.boundStory}
}

// RunStructured posts chat/completions with json_schema response_format.
//
// When StoryID is bound, recent user/assistant turns are retained so later
// requests can attach a compact system prompt (RulesCompact) while still
// providing prior exchange context. Full rules go out on first / refresh turns
// via opts.SystemPrompt (PromptSession).
func (c *Client) RunStructured(ctx context.Context, prompt string, opts provider.StructuredOpts) (json.RawMessage, error) {
	if c.APIKey == "" {
		return nil, errors.New("未設定 XAI_API_KEY")
	}
	storyID := strings.TrimSpace(opts.StoryID)
	if storyID != "" {
		cs := c.ConnectionState()
		if !cs.Alive || cs.StoryID != storyID {
			return nil, provider.ErrNeedsConsent
		}
	}

	schemaObj, err := loadSchema(opts.SchemaPath)
	if err != nil {
		return nil, err
	}

	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = c.ChatModel
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	system := strings.TrimSpace(opts.SystemPrompt)
	if system == "" {
		system = "You are a structured-output engine for a Traditional Chinese D&D 2024 app. Reply with JSON only that matches the provided schema. Never wrap the JSON in markdown fences."
	}

	messages := make([]map[string]string, 0, 8)
	messages = append(messages, map[string]string{"role": "system", "content": system})

	if storyID != "" {
		c.mu.Lock()
		if opts.ResetSession {
			delete(c.chats, storyID)
		}
		if sess := c.chats[storyID]; sess != nil {
			for _, m := range sess.turns {
				messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
			}
		}
		c.mu.Unlock()
	}
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	body := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.7,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "dm_turn",
				"schema": schemaObj,
				"strict": true,
			},
		},
	}
	raw, err := c.postJSON(reqCtx, "/chat/completions", body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("Grok 回應格式錯誤: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return nil, errors.New("Grok API: " + resp.Error.Message)
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return nil, errors.New("Grok 未回傳內容")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = stripJSONFences(content)
	if !json.Valid([]byte(content)) {
		return nil, fmt.Errorf("Grok 回傳的不是合法 JSON")
	}

	if storyID != "" {
		c.mu.Lock()
		if c.chats == nil {
			c.chats = map[string]*storyChat{}
		}
		sess := c.chats[storyID]
		if sess == nil {
			sess = &storyChat{}
			c.chats[storyID] = sess
		}
		sess.turns = append(sess.turns,
			chatMsg{Role: "user", Content: prompt},
			chatMsg{Role: "assistant", Content: content},
		)
		if len(sess.turns) > maxChatTurns {
			sess.turns = sess.turns[len(sess.turns)-maxChatTurns:]
		}
		c.mu.Unlock()
	}

	return json.RawMessage(content), nil
}

// RunImageGeneration calls /images/generations and writes a temp PNG/JPG, returning its path
// so CodexRenderer-style callers can read it. Prefer GrokRenderer for in-process bytes.
func (c *Client) RunImageGeneration(ctx context.Context, prompt string, opts provider.ImageOpts) (string, error) {
	data, ext, err := c.GenerateImageBytes(ctx, prompt, opts.Model)
	if err != nil {
		return "", err
	}
	dir := opts.CWD
	if dir == "" {
		dir = os.TempDir()
	}
	tmpDir := filepath.Join(dir, ".grok-images")
	_ = os.MkdirAll(tmpDir, 0o755)
	name := fmt.Sprintf("grok-%d%s", time.Now().UnixNano(), ext)
	path := filepath.Join(tmpDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// GenerateImageBytes returns image bytes and file extension (".png" / ".jpg").
func (c *Client) GenerateImageBytes(ctx context.Context, prompt, model string) ([]byte, string, error) {
	if c.APIKey == "" {
		return nil, "", errors.New("未設定 XAI_API_KEY")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = c.ImageModel()
	}
	timeout := 120 * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body := map[string]any{
		"model":           model,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
	}
	raw, err := c.postJSON(reqCtx, "/images/generations", body)
	if err != nil {
		// Some deployments only return URL; retry without b64.
		body["response_format"] = "url"
		raw, err = c.postJSON(reqCtx, "/images/generations", body)
		if err != nil {
			return nil, "", err
		}
	}

	var resp struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", fmt.Errorf("Grok 圖片回應格式錯誤: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return nil, "", errors.New("Grok 圖片 API: " + resp.Error.Message)
	}
	if len(resp.Data) == 0 {
		return nil, "", errors.New("Grok 圖片 API 未回傳圖像")
	}
	item := resp.Data[0]
	if item.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return nil, "", fmt.Errorf("解碼 b64 圖片失敗: %w", err)
		}
		ext := sniffExt(data)
		return data, ext, nil
	}
	if item.URL != "" {
		data, ext, err := downloadImage(reqCtx, c.HTTPClient, item.URL)
		if err != nil {
			return nil, "", err
		}
		return data, ext, nil
	}
	return nil, "", errors.New("Grok 圖片回應缺少 b64_json 與 url")
}

func (c *Client) postJSON(ctx context.Context, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("Grok API 逾時")
		}
		return nil, fmt.Errorf("Grok API 連線失敗: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		log.Printf("[grok] HTTP %d path=%s body=%s", resp.StatusCode, path, msg)
		return nil, fmt.Errorf("Grok API HTTP %d：%s", resp.StatusCode, msg)
	}
	return raw, nil
}

func loadSchema(path string) (any, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("缺少 structured-output schema 路徑")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("讀取 schema 失敗: %w", err)
	}
	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("schema JSON 無效: %w", err)
	}
	// xAI strict mode is happier without draft meta keys sometimes; keep as-is.
	return obj, nil
}

func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

func sniffExt(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
		return ".png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return ".jpg"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}
	return ".png"
}

func downloadImage(ctx context.Context, hc *http.Client, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("下載 Grok 圖片失敗: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("下載 Grok 圖片 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}
	ext := sniffExt(data)
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "jpeg") {
		ext = ".jpg"
	} else if strings.Contains(ct, "webp") {
		ext = ".webp"
	} else if strings.Contains(ct, "png") {
		ext = ".png"
	}
	return data, ext, nil
}
