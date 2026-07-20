// Package applog provides a small daily-rotating file writer for the server log.
package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DailyWriter tees log lines into logs named by calendar day, e.g.
//
//	logs/server-2006-01-02.log
//
// basePath is the logical log file path (default logs/server.log). The date is
// inserted before the extension. The file is reopened when the local date
// changes so long-running processes still split by day.
type DailyWriter struct {
	basePath string

	mu   sync.Mutex
	day  string // YYYY-MM-DD
	file *os.File
}

// NewDailyWriter prepares a writer for basePath. The file is opened on first Write.
func NewDailyWriter(basePath string) *DailyWriter {
	return &DailyWriter{basePath: strings.TrimSpace(basePath)}
}

// PathFor returns the dated log path for t (local time).
func PathFor(basePath string, t time.Time) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = "server.log"
	}
	day := t.Format("2006-01-02")
	ext := filepath.Ext(basePath)
	stem := strings.TrimSuffix(basePath, ext)
	if ext == "" {
		ext = ".log"
	}
	// Avoid double-dating if the base already ends with -YYYY-MM-DD.
	if len(stem) >= 11 && stem[len(stem)-11] == '-' {
		if _, err := time.Parse("2006-01-02", stem[len(stem)-10:]); err == nil {
			stem = stem[:len(stem)-11]
		}
	}
	return stem + "-" + day + ext
}

// Write implements io.Writer.
func (w *DailyWriter) Write(p []byte) (int, error) {
	if w == nil || w.basePath == "" {
		return len(p), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	day := time.Now().Format("2006-01-02")
	if w.file == nil || day != w.day {
		if err := w.rotateLocked(day); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

// CurrentPath returns the open file path, or "" if not yet opened.
func (w *DailyWriter) CurrentPath() string {
	if w == nil {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.day == "" {
		return PathFor(w.basePath, time.Now())
	}
	return PathFor(w.basePath, time.Now())
}

// Close closes the underlying file.
func (w *DailyWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.day = ""
	return err
}

func (w *DailyWriter) rotateLocked(day string) error {
	path := PathFor(w.basePath, time.Now())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if w.file != nil {
		_ = w.file.Close()
	}
	w.file = f
	w.day = day
	return nil
}
