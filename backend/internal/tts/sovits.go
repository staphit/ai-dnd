// Package tts talks to a local GPT-SoVITS api_v2 server so the table can read
// DM narration aloud without any cloud service. Start the server with
// scripts/sovits.sh; it needs one short reference audio clip (3–10 s) plus its
// transcript to define the narrator voice.
package tts

import (
	"bytes"
	"context"
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

// logPrompts mirrors the LOG_PROMPTS toggle used by the dm/forge packages;
// read at call time since .env loads in main() after package init.
func logPrompts() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_PROMPTS"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// Client holds the connection and voice settings for one GPT-SoVITS server.
type Client struct {
	// BaseURL is the api_v2 root, e.g. http://127.0.0.1:9880.
	BaseURL string
	// RefAudio is the reference clip path *on the TTS server's machine*.
	RefAudio string
	// PromptText is the transcript of the reference clip.
	PromptText string
	PromptLang string
	TextLang   string
	// Speed is api_v2's speed_factor: 1.0 is normal, >1 faster, <1 slower.
	Speed float64

	HTTP *http.Client
}

// NewClientFromEnv builds a Client from SOVITS_* environment variables.
func NewClientFromEnv() *Client {
	speed := 1.0
	if v, err := strconv.ParseFloat(envOr("SOVITS_SPEED", ""), 64); err == nil && v > 0 {
		speed = v
	}
	return &Client{
		BaseURL:    strings.TrimRight(envOr("SOVITS_URL", "http://127.0.0.1:9880"), "/"),
		RefAudio:   strings.TrimSpace(os.Getenv("SOVITS_REF_AUDIO")),
		PromptText: strings.TrimSpace(os.Getenv("SOVITS_PROMPT_TEXT")),
		PromptLang: envOr("SOVITS_PROMPT_LANG", "zh"),
		TextLang:   envOr("SOVITS_TEXT_LANG", "zh"),
		Speed:      speed,
		HTTP:       &http.Client{},
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// Configured reports whether the voice is set up; the api_v2 endpoint cannot
// synthesize without a reference clip.
func (c *Client) Configured() error {
	if c.RefAudio == "" {
		return errors.New("尚未設定朗讀聲線：請在 backend/.env 設定 SOVITS_REF_AUDIO（TTS 主機上的參考音檔路徑）與 SOVITS_PROMPT_TEXT（該音檔的逐字稿）")
	}
	return nil
}

// Synthesize renders text to speech and returns the audio bytes and MIME type.
func (c *Client) Synthesize(ctx context.Context, text string) ([]byte, string, error) {
	if err := c.Configured(); err != nil {
		return nil, "", err
	}
	if logPrompts() {
		log.Printf("[tts] lang=%s speed=%.2f text: %s", c.TextLang, c.Speed, text)
	}
	payload := map[string]any{
		"text":              text,
		"text_lang":         c.TextLang,
		"ref_audio_path":    c.RefAudio,
		"prompt_text":       c.PromptText,
		"prompt_lang":       c.PromptLang,
		"text_split_method": "cut5",
		"batch_size":        1,
		"media_type":        "wav",
		"streaming_mode":    false,
		"speed_factor":      c.Speed,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/tts", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", fmt.Errorf("無法連線本地 GPT-SoVITS（%s）；請先執行 scripts/sovits.sh", c.BaseURL)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		detail := strings.TrimSpace(string(data))
		var parsed struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(data, &parsed) == nil && parsed.Message != "" {
			detail = parsed.Message
		}
		if len(detail) > 300 {
			detail = detail[:300]
		}
		return nil, "", fmt.Errorf("GPT-SoVITS 回應錯誤（HTTP %d）：%s", resp.StatusCode, detail)
	}
	if len(data) == 0 {
		return nil, "", errors.New("GPT-SoVITS 回傳了空的音訊")
	}
	mime := resp.Header.Get("content-type")
	if mime == "" {
		mime = "audio/wav"
	}
	return data, mime, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
