package grok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"dndduet/internal/cliexec"
	"dndduet/internal/provider"
)

// CLIClient runs the official Grok Build CLI (`grok login` subscription auth).
// No XAI_API_KEY required when the user is signed in via the CLI.
type CLIClient struct {
	Command         string
	ConfiguredModel string
	mu              sync.Mutex
	boundStory      string
	boundAlive      bool
	statusMu        sync.Mutex
	cachedStatus    *provider.Status
	cachedAt        time.Time
}

const cliStatusTTL = 30 * time.Second

// ResolveCLICommand finds the grok binary (GROK_CLI_PATH, PATH, or ~/.grok/bin).
func ResolveCLICommand() string {
	if p := strings.TrimSpace(os.Getenv("GROK_CLI_PATH")); p != "" {
		return p
	}
	if p, err := exec.LookPath("grok"); err == nil && p != "" {
		return p
	}
	// Official installer puts the binary under ~/.grok/bin
	home, err := os.UserHomeDir()
	if err != nil {
		return "grok"
	}
	candidates := []string{
		filepath.Join(home, ".grok", "bin", "grok"),
	}
	if runtime.GOOS == "windows" {
		candidates = append([]string{
			filepath.Join(home, ".grok", "bin", "grok.exe"),
		}, candidates...)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return "grok"
}

// NewCLIClientFromEnv builds a CLI client. It does not require an API key.
func NewCLIClientFromEnv() *CLIClient {
	return &CLIClient{
		Command:         ResolveCLICommand(),
		ConfiguredModel: defaultChatModel, // only grok-4.5
	}
}

var _ provider.API = (*CLIClient)(nil)

func (c *CLIClient) Model() string {
	return defaultChatModel
}

func (c *CLIClient) ModelOptions() []provider.ModelOption {
	return ModelOptions
}

func (c *CLIClient) ImageModel() string {
	// Image gen is not exposed as a simple headless CLI path; use HTTP key for images.
	return ""
}

func (c *CLIClient) EffortOptions() []provider.ModelOption {
	return []provider.ModelOption{
		{ID: "", Label: "Grok 預設"},
		{ID: "low", Label: "Low"},
		{ID: "medium", Label: "Medium"},
		{ID: "high", Label: "High"},
	}
}

func (c *CLIClient) NormalizeModel(value string) (string, error) {
	// Same policy as HTTP client: only grok-4.5.
	return (&Client{ChatModel: defaultChatModel}).NormalizeModel(value)
}

func (c *CLIClient) NormalizeEffort(value string) (string, error) {
	effort := strings.TrimSpace(value)
	if effort == "" {
		return "", nil
	}
	for _, o := range c.EffortOptions() {
		if o.ID == effort {
			return effort, nil
		}
	}
	return "", errors.New("不支援的推理強度選項")
}

func (c *CLIClient) Status(ctx context.Context) provider.Status {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	if c.cachedStatus != nil && time.Since(c.cachedAt) < cliStatusTTL {
		return *c.cachedStatus
	}
	st := c.probeLogin()
	c.cachedStatus = &st
	c.cachedAt = time.Now()
	return st
}

func (c *CLIClient) probeLogin() provider.Status {
	// `grok models` succeeds and prints login state when the CLI is usable.
	runCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, c.Command, "models")
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return provider.Status{Configured: false, Provider: "Grok CLI", Model: c.Model(), Message: "Grok CLI 登入檢查逾時"}
		}
		if errors.Is(err, exec.ErrNotFound) {
			return provider.Status{Configured: false, Provider: "Grok CLI", Model: c.Model(), Message: "找不到 Grok CLI；請安裝（curl -fsSL https://x.ai/cli/install.sh | bash）或設定 GROK_CLI_PATH"}
		}
		// Not logged in typically still exits non-zero with a message.
		msg := strings.TrimSpace(text)
		if msg == "" {
			msg = "Grok CLI 尚未登入，請先執行 grok login"
		}
		return provider.Status{Configured: false, Provider: "Grok CLI", Model: c.Model(), Message: msg}
	}
	if strings.Contains(strings.ToLower(text), "not logged") || strings.Contains(text, "請先") {
		return provider.Status{Configured: false, Provider: "Grok CLI", Model: c.Model(), Message: "Grok CLI 尚未登入，請先執行 grok login"}
	}
	return provider.Status{Configured: true, Provider: "Grok CLI（grok login）", Model: c.Model()}
}

func (c *CLIClient) Connect(ctx context.Context, storyID string) error {
	st := c.Status(ctx)
	if !st.Configured {
		if st.Message != "" {
			return errors.New(st.Message)
		}
		return errors.New("Grok CLI 尚未就緒")
	}
	storyID = strings.TrimSpace(storyID)
	if storyID == "" {
		return errors.New("缺少 campaignId")
	}
	c.mu.Lock()
	c.boundStory = storyID
	c.boundAlive = true
	c.mu.Unlock()
	return nil
}

func (c *CLIClient) ConnectionState() provider.ConnState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return provider.ConnState{Alive: c.boundAlive && c.boundStory != "", StoryID: c.boundStory}
}

// RunStructured invokes:
//
//	grok --prompt-file … --json-schema … --output-format json --max-turns 1 --always-approve
//
// using the user's `grok login` session (SuperGrok / X Premium), no API key.
func (c *CLIClient) RunStructured(ctx context.Context, prompt string, opts provider.StructuredOpts) (json.RawMessage, error) {
	storyID := strings.TrimSpace(opts.StoryID)
	if storyID != "" {
		cs := c.ConnectionState()
		if !cs.Alive || cs.StoryID != storyID {
			return nil, provider.ErrNeedsConsent
		}
	}
	// CLI is single-shot (max-turns 1): always fold system rules into the prompt.
	// Compact mode still helps when the server only attaches the short reminder.
	if sys := strings.TrimSpace(opts.SystemPrompt); sys != "" {
		prompt = sys + "\n\n" + prompt
	}
	if strings.TrimSpace(opts.SchemaPath) == "" {
		return nil, errors.New("缺少 structured-output schema 路徑")
	}
	schemaBytes, err := os.ReadFile(opts.SchemaPath)
	if err != nil {
		return nil, fmt.Errorf("讀取 schema 失敗: %w", err)
	}
	// Compact JSON for argv (must remain valid).
	var schemaObj any
	if err := json.Unmarshal(schemaBytes, &schemaObj); err != nil {
		return nil, fmt.Errorf("schema JSON 無效: %w", err)
	}
	schemaJSON, err := json.Marshal(schemaObj)
	if err != nil {
		return nil, err
	}

	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = c.ConfiguredModel
	}

	// Long prompts go via temp file (Windows argv limits).
	promptFile, err := os.CreateTemp("", "dnd-grok-prompt-*.txt")
	if err != nil {
		return nil, err
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)
	if _, err := promptFile.WriteString(prompt); err != nil {
		promptFile.Close()
		return nil, err
	}
	if err := promptFile.Close(); err != nil {
		return nil, err
	}

	cwd := strings.TrimSpace(opts.CWD)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	args := []string{
		"--prompt-file", promptPath,
		"--json-schema", string(schemaJSON),
		"--output-format", "json",
		"--max-turns", "1",
		"--always-approve",
		"--no-subagents",
		"--disable-web-search",
		"--cwd", cwd,
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if effort := strings.TrimSpace(opts.Effort); effort != "" {
		args = append(args, "--reasoning-effort", effort)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 180 * time.Second
	}

	stdout, stderr, err := cliexec.Run(ctx, cliexec.Options{
		Command: c.Command,
		Args:    args,
		Timeout: timeout,
		Env:     os.Environ(),
	})
	if err != nil {
		if errors.Is(err, cliexec.ErrTimeout) {
			return nil, errors.New("Grok CLI 逾時")
		}
		if errors.Is(err, cliexec.ErrNotFound) {
			return nil, errors.New("找不到 Grok CLI；請安裝或設定 GROK_CLI_PATH")
		}
		var exitErr *cliexec.ExitError
		if errors.As(err, &exitErr) {
			msg := strings.TrimSpace(exitErr.Stderr)
			if msg == "" {
				msg = strings.TrimSpace(stdout)
			}
			if len(msg) > 500 {
				msg = msg[:500] + "…"
			}
			if msg == "" {
				msg = fmt.Sprintf("Grok CLI 結束代碼 %d", exitErr.Code)
			}
			return nil, errors.New(msg)
		}
		return nil, err
	}

	raw := extractCLIStructuredJSON(stdout)
	if raw == nil {
		log.Printf("[grok-cli] no structured JSON in stdout (len=%d) stderr=%q", len(stdout), truncate(stderr, 200))
		return nil, errors.New("Grok CLI 沒有回傳有效的結構化 JSON")
	}
	return raw, nil
}

// RunImageGeneration is not used: scene/portrait art is GPT-only via Codex.
func (c *CLIClient) RunImageGeneration(ctx context.Context, prompt string, opts provider.ImageOpts) (string, error) {
	return "", errors.New("圖片生成僅支援 Codex（GPT $imagegen）；請用 codex login，Grok 僅可作為 DM")
}

func extractCLIStructuredJSON(stdout string) json.RawMessage {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil
	}
	// Preferred envelope from --output-format json
	var envelope struct {
		StructuredOutput json.RawMessage `json:"structuredOutput"`
		Text             string          `json:"text"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err == nil {
		if len(envelope.StructuredOutput) > 0 && json.Valid(envelope.StructuredOutput) {
			// structuredOutput may be an object already
			if envelope.StructuredOutput[0] == '{' || envelope.StructuredOutput[0] == '[' {
				return envelope.StructuredOutput
			}
		}
		if t := strings.TrimSpace(envelope.Text); t != "" {
			t = stripJSONFences(t)
			if json.Valid([]byte(t)) {
				return json.RawMessage(t)
			}
		}
	}
	// Raw JSON body
	if json.Valid([]byte(stdout)) {
		// If it's the envelope without our fields, try again
		var generic map[string]any
		if err := json.Unmarshal([]byte(stdout), &generic); err == nil {
			if so, ok := generic["structuredOutput"]; ok {
				b, _ := json.Marshal(so)
				if json.Valid(b) {
					return b
				}
			}
			if t, ok := generic["text"].(string); ok {
				t = stripJSONFences(strings.TrimSpace(t))
				if json.Valid([]byte(t)) {
					return json.RawMessage(t)
				}
			}
		}
		// Entire stdout is the DM turn
		if _, hasText := generic["text"]; !hasText {
			if _, hasNarration := generic["narration"]; hasNarration {
				return json.RawMessage(stdout)
			}
		}
	}
	// Last resort: find first {...} block
	if i := strings.Index(stdout, "{"); i >= 0 {
		if j := strings.LastIndex(stdout, "}"); j > i {
			chunk := stdout[i : j+1]
			if json.Valid([]byte(chunk)) {
				return json.RawMessage(chunk)
			}
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// NewProviderFromEnv selects Grok access mode:
//  1. GROK_MODE=api  → HTTP API key
//  2. GROK_MODE=cli  → Grok Build CLI (grok login)
//  3. default        → CLI if logged in, else API key, else nil
func NewProviderFromEnv() provider.API {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GROK_MODE")))
	switch mode {
	case "api", "http", "key":
		return NewClientFromEnv()
	case "cli", "login", "oauth":
		return NewCLIClientFromEnv()
	}

	// Auto: prefer CLI when the binary is available and already logged in.
	cli := NewCLIClientFromEnv()
	st := cli.Status(context.Background())
	if st.Configured {
		log.Printf("[grok] 使用 CLI 登入（%s）", cli.Command)
		return cli
	}
	if api := NewClientFromEnv(); api != nil {
		log.Printf("[grok] CLI 未登入，改用 XAI_API_KEY HTTP")
		return api
	}
	// Return CLI anyway so Status message guides the user to `grok login`.
	return cli
}
