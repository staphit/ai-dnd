package codex

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"dndduet/internal/provider"
)

const statusCacheTTL = 30 * time.Second

// Status probes `codex login status`, caching the result for 30 seconds.
func (c *Client) Status(ctx context.Context) provider.Status {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()

	nowFn := c.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()
	if c.cachedValue != nil && now.Sub(c.cachedAt) < statusCacheTTL {
		return *c.cachedValue
	}

	var value provider.Status
	configured, err := c.probeLogin()
	if err != nil {
		value = provider.Status{Configured: false, Provider: "Codex CLI", Model: c.Model(), Message: err.Error()}
	} else if configured {
		value = provider.Status{Configured: true, Provider: "Codex CLI（ChatGPT 登入）", Model: c.Model()}
	} else {
		value = provider.Status{Configured: false, Provider: "Codex CLI", Model: c.Model(), Message: "Codex CLI 尚未登入，請先執行 codex login"}
	}

	c.cachedValue = &value
	c.cachedAt = now
	return value
}

// probeLogin returns true when `codex login status` exits 0. A non-zero exit
// means "not logged in" (not an error); missing CLI or timeout are errors.
//
// The probe uses a detached context (not the caller's request context) so a
// client disconnect cannot cancel it and poison the 30s status cache — matching
// the original Node backend, whose probe was independent of any request.
func (c *Client) probeLogin() (bool, error) {
	runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, c.Command, "login", "status")
	cmd.Env = statusEnv()
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return false, errors.New("Codex CLI 登入檢查逾時")
	}
	if errors.Is(err, exec.ErrNotFound) {
		return false, errors.New("找不到 Codex CLI；請先安裝 Codex，或設定 CODEX_CLI_PATH")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
}
