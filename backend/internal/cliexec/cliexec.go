// Package cliexec runs an external CLI with a timeout, feeding it stdin and
// capturing a bounded amount of stdout/stderr. It returns typed errors so each
// provider can format its own user-facing messages.
package cliexec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// MaxCapturedOutput bounds how many trailing bytes of each stream are kept.
const MaxCapturedOutput = 4_000_000

// Sentinel errors distinguishing failure modes.
var (
	ErrTimeout  = errors.New("cliexec: timed out")
	ErrCanceled = errors.New("cliexec: canceled")
	ErrNotFound = errors.New("cliexec: executable not found")
)

// ExitError reports a non-zero exit code and the captured stderr.
type ExitError struct {
	Code   int
	Stderr string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("cliexec: exit code %d", e.Code)
}

// Options configures a Run.
type Options struct {
	Command string
	Args    []string
	Input   string
	Timeout time.Duration
	Env     []string
}

// cappedBuffer keeps only the final MaxCapturedOutput bytes, bounding memory.
type cappedBuffer struct {
	buf []byte
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	if len(b.buf) > MaxCapturedOutput {
		b.buf = b.buf[len(b.buf)-MaxCapturedOutput:]
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string { return string(b.buf) }

// Run spawns the command, writes Input to its stdin, and returns the captured
// output. On failure it returns one of the sentinel errors, an *ExitError, or
// the raw error.
func Run(ctx context.Context, opts Options) (string, string, error) {
	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, opts.Command, opts.Args...)
	cmd.Env = opts.Env
	cmd.Stdin = strings.NewReader(opts.Input)
	var stdout, stderr cappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return stdout.String(), stderr.String(), ErrTimeout
		}
		if errors.Is(runCtx.Err(), context.Canceled) {
			return stdout.String(), stderr.String(), ErrCanceled
		}
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return stdout.String(), stderr.String(), ErrNotFound
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.String(), stderr.String(), &ExitError{Code: exitErr.ExitCode(), Stderr: stderr.String()}
		}
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}
