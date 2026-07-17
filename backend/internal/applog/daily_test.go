package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathForInsertsDate(t *testing.T) {
	got := PathFor(filepath.Join("logs", "server.log"), time.Date(2026, 7, 17, 15, 0, 0, 0, time.Local))
	if !strings.HasSuffix(got, "server-2026-07-17.log") {
		t.Fatalf("got %q", got)
	}
}

func TestPathForNoDoubleDate(t *testing.T) {
	got := PathFor(filepath.Join("logs", "server-2026-07-16.log"), time.Date(2026, 7, 17, 0, 0, 0, 0, time.Local))
	if !strings.HasSuffix(got, "server-2026-07-17.log") {
		t.Fatalf("got %q", got)
	}
}

func TestDailyWriterWritesDatedFile(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "server.log")
	w := NewDailyWriter(base)
	defer w.Close()

	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	path := PathFor(base, time.Now())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("content %q", data)
	}
}
