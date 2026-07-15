package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dndduet/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func ev(round int, text string) store.MemoryEvent {
	return store.MemoryEvent{Round: round, Role: "dm", Text: text}
}

func TestMaterialiseWritesSummaryAndTail(t *testing.T) {
	st := openStore(t)
	dir := t.TempDir()
	m, err := New(st, dir, "campaign-data/memory", func(context.Context, string) (string, error) { return "", nil }, 100, 40)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	if err := m.Record("s1", []store.MemoryEvent{ev(1, "隊伍進入禮拜堂"), ev(1, "找到符文")}); err != nil {
		t.Fatalf("record: %v", err)
	}
	_ = st.SaveMemorySummary("s1", "前情：伊薩克失蹤", 1, 1)

	if err := m.Materialise("s1"); err != nil {
		t.Fatalf("materialise: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "s1.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "前情：伊薩克失蹤") {
		t.Errorf("file missing summary:\n%s", s)
	}
	// covered_seq=1 so only the second event (seq 2) is uncompacted tail.
	if !strings.Contains(s, "找到符文") || strings.Contains(s, "隊伍進入禮拜堂") {
		t.Errorf("tail should hold only uncompacted events:\n%s", s)
	}

	if ref := m.Ref("s1"); ref != "campaign-data/memory/s1.md" {
		t.Errorf("Ref = %q", ref)
	}
}

func TestRecordTriggersBackgroundCompaction(t *testing.T) {
	st := openStore(t)
	dir := t.TempDir()
	called := make(chan string, 8)
	m, err := New(st, dir, "mem", func(_ context.Context, prompt string) (string, error) {
		called <- prompt
		return "壓縮後的摘要", nil
	}, 3, 40)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	// Below threshold: no compaction.
	_ = m.Record("s1", []store.MemoryEvent{ev(1, "a"), ev(1, "b")})
	select {
	case <-called:
		t.Fatal("compaction fired below threshold")
	case <-time.After(150 * time.Millisecond):
	}

	// Crossing the threshold (3) triggers async compaction.
	_ = m.Record("s1", []store.MemoryEvent{ev(2, "c")})
	select {
	case prompt := <-called:
		if !strings.Contains(prompt, "a") || !strings.Contains(prompt, "c") {
			t.Errorf("compaction prompt missing events: %q", prompt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("compaction did not fire after crossing threshold")
	}

	// Poll for the summary to be saved and covered_seq advanced to 3.
	deadline := time.Now().Add(2 * time.Second)
	for {
		summary, covered, ok, _ := st.MemorySummary("s1")
		if ok && summary == "壓縮後的摘要" && covered == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("summary not saved: summary=%q covered=%d ok=%v", summary, covered, ok)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
