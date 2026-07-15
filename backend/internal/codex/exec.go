// Package codex wraps the local Codex CLI. It mirrors the behaviour of the
// original Node modules (codex-exec.mjs, codex-status.mjs) so the Go backend
// reuses the same local ChatGPT login without an OPENAI_API_KEY.
package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"dndduet/internal/cliexec"
	"dndduet/internal/provider"
)

// ImageModel is the fixed label reported for image generation.
const ImageModel = "Codex $imagegen（gpt-image-2）"

// ModelOptions lists the documented Codex model choices, matching codex-exec.mjs.
var ModelOptions = []provider.ModelOption{
	{ID: "", Label: "Codex 預設（沿用目前設定）"},
	{ID: "gpt-5.6-sol", Label: "GPT-5.6 Sol（品質）"},
	{ID: "gpt-5.6-terra", Label: "GPT-5.6 Terra（平衡）"},
	{ID: "gpt-5.6-luna", Label: "GPT-5.6 Luna（速度）"},
	{ID: "gpt-5.6", Label: "GPT-5.6（通用）"},
}

// EffortOptions lists the selectable reasoning-effort levels for `codex exec
// -c model_reasoning_effort=…` and the app-server turn `effort` field.
var EffortOptions = []provider.ModelOption{
	{ID: "", Label: "Codex 預設推理強度"},
	{ID: "minimal", Label: "Minimal（最快）"},
	{ID: "low", Label: "Low（快速）"},
	{ID: "medium", Label: "Medium（平衡）"},
	{ID: "high", Label: "High（深入）"},
	{ID: "xhigh", Label: "XHigh（最深入）"},
}

var threadIDPattern = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)

// Client is the concrete Codex CLI wrapper.
type Client struct {
	// Command is the codex executable (CODEX_CLI_PATH or "codex").
	Command string
	// ConfiguredModel is CODEX_MODEL; empty means "use the CLI default".
	ConfiguredModel string
	// ConfiguredEffort is CODEX_EFFORT; empty means "use the CLI default".
	ConfiguredEffort string

	now         func() time.Time
	statusMu    sync.Mutex
	cachedValue *provider.Status
	cachedAt    time.Time
}

// NewClient builds a Client from the environment.
func NewClient() *Client {
	cmd := strings.TrimSpace(os.Getenv("CODEX_CLI_PATH"))
	if cmd == "" {
		cmd = "codex"
	}
	return &Client{
		Command:          cmd,
		ConfiguredModel:  strings.TrimSpace(os.Getenv("CODEX_MODEL")),
		ConfiguredEffort: strings.TrimSpace(os.Getenv("CODEX_EFFORT")),
		now:              time.Now,
	}
}

var _ provider.API = (*Client)(nil)

// Model is the human-readable model label, matching codex-exec.mjs `codexModel`.
func (c *Client) Model() string {
	if c.ConfiguredModel != "" {
		return c.ConfiguredModel
	}
	return "Codex 預設模型"
}

// ModelOptions returns the selectable Codex models.
func (c *Client) ModelOptions() []provider.ModelOption { return ModelOptions }

// ImageModel returns the Codex image-generation model label.
func (c *Client) ImageModel() string { return ImageModel }

// NormalizeModel validates a requested model id. An empty request keeps the
// configured default; an unknown id is rejected.
func (c *Client) NormalizeModel(value string) (string, error) {
	model := strings.TrimSpace(value)
	if model == "" {
		return c.ConfiguredModel, nil
	}
	for _, o := range ModelOptions {
		if o.ID == model {
			return model, nil
		}
	}
	return "", errors.New("不支援的 Codex 模型選項")
}

// EffortOptions returns the selectable reasoning-effort levels.
func (c *Client) EffortOptions() []provider.ModelOption { return EffortOptions }

// NormalizeEffort validates a requested reasoning-effort id. An empty request
// keeps the configured default; an unknown id is rejected.
func (c *Client) NormalizeEffort(value string) (string, error) {
	effort := strings.TrimSpace(value)
	if effort == "" {
		return c.ConfiguredEffort, nil
	}
	for _, o := range EffortOptions {
		if o.ID == effort {
			return effort, nil
		}
	}
	return "", errors.New("不支援的 Codex 推理強度選項")
}

// execEnv returns the environment for `codex exec`, stripped of API-key and
// override variables so the run uses the ChatGPT login rather than API billing.
func execEnv() []string {
	return cleanEnv("OPENAI_API_KEY", "CODEX_API_KEY", "OPENAI_MODEL", "OPENAI_IMAGE_MODEL", "OPENAI_AGENT_TRACING")
}

// statusEnv returns the environment for `codex login status`.
func statusEnv() []string {
	return cleanEnv("OPENAI_API_KEY", "CODEX_API_KEY")
}

func cleanEnv(remove ...string) []string {
	drop := make(map[string]struct{}, len(remove))
	for _, k := range remove {
		drop[k] = struct{}{}
	}
	src := os.Environ()
	out := make([]string, 0, len(src))
	for _, kv := range src {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		if _, skip := drop[key]; skip {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func formatCLIError(stderr, fallback string) string {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return fallback
	}
	lines := regexp.MustCompile(`\r?\n`).Split(trimmed, -1)
	if len(lines) > 12 {
		lines = lines[len(lines)-12:]
	}
	detail := strings.Join(lines, "\n")
	if detail == "" {
		return fallback
	}
	return detail
}

// baseExecArgs builds the `codex exec` argument list. model is already resolved
// by the caller (handleDm normalises the untrusted body.model against the
// allowlist, or it is the operator-configured CODEX_MODEL), so it is used
// directly — re-validating here would reject a CODEX_MODEL outside the allowlist.
func (c *Client) baseExecArgs(cwd, sandbox, model string) []string {
	// --skip-git-repo-check lets exec run when cwd is not a git repo (e.g. a
	// custom CODEX_CWD or memory compaction directory); without it codex refuses
	// with "Not inside a trusted directory".
	args := []string{"exec", "--ephemeral", "--color", "never", "--skip-git-repo-check", "--sandbox", sandbox, "--cd", cwd}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	return args
}

// runProcess spawns the codex CLI, feeds it input on stdin and captures output,
// translating cliexec's typed errors into Codex-specific messages.
func (c *Client) runProcess(ctx context.Context, args []string, input string, timeout time.Duration, env []string) (string, string, error) {
	stdout, stderr, err := cliexec.Run(ctx, cliexec.Options{Command: c.Command, Args: args, Input: input, Timeout: timeout, Env: env})
	if err != nil {
		switch {
		case errors.Is(err, cliexec.ErrTimeout):
			return "", "", fmt.Errorf("Codex CLI 超過 %d 秒仍未完成", int(math.Ceil(timeout.Seconds())))
		case errors.Is(err, cliexec.ErrCanceled):
			return "", "", errors.New("Codex CLI 工作已取消")
		case errors.Is(err, cliexec.ErrNotFound):
			return "", "", errors.New("找不到 Codex CLI；請先安裝 Codex，或設定 CODEX_CLI_PATH")
		}
		var exitErr *cliexec.ExitError
		if errors.As(err, &exitErr) {
			return "", "", errors.New(formatCLIError(exitErr.Stderr, fmt.Sprintf("Codex CLI 結束代碼：%d", exitErr.Code)))
		}
		return "", "", err
	}
	return stdout, stderr, nil
}

// RunStructured runs `codex exec --output-schema` and returns the parsed JSON.
func (c *Client) RunStructured(ctx context.Context, prompt string, opts provider.StructuredOpts) (json.RawMessage, error) {
	args := c.baseExecArgs(opts.CWD, "read-only", opts.Model)
	// opts.Effort is resolved by the caller (NormalizeEffort allowlist), same as
	// the model. The quotes make the value a TOML string for `-c`.
	if strings.TrimSpace(opts.Effort) != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", opts.Effort))
	}
	args = append(args, "--output-schema", opts.SchemaPath)
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	stdout, _, err := c.runProcess(ctx, args, prompt, timeout, execEnv())
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(stdout)
	if !json.Valid([]byte(trimmed)) {
		return nil, errors.New("Codex CLI 沒有回傳有效的結構化 JSON")
	}
	return json.RawMessage(trimmed), nil
}

// Connect is a no-op for the stateless exec provider: every turn spawns a fresh
// `codex exec`, so there is no persistent connection to establish. It exists to
// satisfy provider.API.
func (c *Client) Connect(context.Context, string) error { return nil }

// ConnectionState reports the exec provider as always ready and unbound: it is
// stateless, so it never needs consent and never triggers delta mode.
func (c *Client) ConnectionState() provider.ConnState {
	return provider.ConnState{Alive: true, StoryID: ""}
}

// RunText runs `codex exec` (read-only, no output schema) and returns the
// trimmed stdout. It is used off the DM connection for background memory
// compaction, so it never touches the persistent app-server thread.
func (c *Client) RunText(ctx context.Context, prompt string, opts provider.StructuredOpts) (string, error) {
	args := c.baseExecArgs(opts.CWD, "read-only", opts.Model)
	if strings.TrimSpace(opts.Effort) != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", opts.Effort))
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	stdout, _, err := c.runProcess(ctx, args, prompt, timeout, execEnv())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func parseJSONLines(raw string) []map[string]any {
	var events []map[string]any
	for _, line := range regexp.MustCompile(`\r?\n`).Split(raw, -1) {
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err == nil {
			events = append(events, obj)
		}
	}
	return events
}

func findImages(dir string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if imageExtPattern.MatchString(d.Name()) {
			found = append(found, p)
		}
		return nil
	})
	return found, err
}

var imageExtPattern = regexp.MustCompile(`(?i)\.(?:png|jpe?g|webp)$`)

// RunImageGeneration runs `codex exec --json`, locates the single generated
// image inside Codex's generated_images directory and returns its real path.
func (c *Client) RunImageGeneration(ctx context.Context, prompt string, opts provider.ImageOpts) (string, error) {
	args := c.baseExecArgs(opts.CWD, "read-only", opts.Model)
	args = append(args, "--json")
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 420 * time.Second
	}
	stdout, _, err := c.runProcess(ctx, args, prompt, timeout, execEnv())
	if err != nil {
		return "", err
	}

	events := parseJSONLines(stdout)
	var threadID string
	for _, e := range events {
		if t, _ := e["type"].(string); t == "thread.started" {
			threadID, _ = e["thread_id"].(string)
			break
		}
	}
	if threadID == "" || !threadIDPattern.MatchString(threadID) {
		return "", errors.New("Codex CLI 沒有回報圖片工作識別碼")
	}

	codeHome := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if codeHome == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", herr
		}
		codeHome = filepath.Join(home, ".codex")
	}
	allowedRoot, err := filepath.EvalSymlinks(filepath.Join(codeHome, "generated_images"))
	if err != nil {
		return "", err
	}
	sessionRoot, err := filepath.EvalSymlinks(filepath.Join(allowedRoot, threadID))
	if err != nil {
		return "", err
	}
	if sessionRoot != allowedRoot && !strings.HasPrefix(sessionRoot, allowedRoot+string(filepath.Separator)) {
		return "", errors.New("Codex 圖片輸出位於不允許的位置")
	}

	images, err := findImages(sessionRoot)
	if err != nil {
		return "", err
	}
	if len(images) != 1 {
		if len(images) == 0 {
			return "", errors.New("Codex imagegen 沒有產生圖片檔案")
		}
		return "", errors.New("Codex imagegen 產生了多張圖片，無法判斷要使用哪一張")
	}
	return filepath.EvalSymlinks(images[0])
}
