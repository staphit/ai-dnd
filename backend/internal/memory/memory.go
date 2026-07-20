// Package memory persists a story's narrative memory in SQLite and materialises
// a compacted, human-readable view as a Markdown file under the Codex working
// directory, so DM turns can send only the current delta while Codex reads the
// prior context from the file itself (read-only sandbox).
//
// Raw turn events are appended synchronously; the expensive compaction (a Codex
// summarisation) runs in the background once uncompacted events pass a
// threshold, so a story's memory stays bounded and cheap to read.
package memory

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dndduet/internal/store"
)

// TextRunner runs a plain-text Codex prompt and returns the trimmed output. It
// is injected so compaction runs off the DM connection (a stateless exec call).
type TextRunner func(ctx context.Context, prompt string) (string, error)

// Manager owns a story's memory pipeline: raw events in SQLite, a compacted
// summary, and the materialised Markdown file Codex reads.
type Manager struct {
	store     *store.Store
	dir       string // absolute directory holding <storyID>.md files (under CWD)
	relDir    string // dir relative to the Codex CWD, for the prompt reference
	summarize TextRunner
	threshold int // uncompacted-event count that triggers compaction
	tailK     int // recent raw events materialised alongside the summary

	mu       sync.Mutex
	inflight map[string]bool // per-story compaction guard
}

// New builds a Manager and ensures the memory directory exists.
func New(st *store.Store, dir, relDir string, summarize TextRunner, threshold, tailK int) (*Manager, error) {
	if threshold <= 0 {
		threshold = 20
	}
	if tailK <= 0 {
		tailK = 40
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Manager{
		store:     st,
		dir:       dir,
		relDir:    relDir,
		summarize: summarize,
		threshold: threshold,
		tailK:     tailK,
		inflight:  map[string]bool{},
	}, nil
}

// Ref returns the memory file path relative to the Codex CWD (POSIX slashes), to
// embed in the DM prompt.
func (m *Manager) Ref(storyID string) string {
	rel := filepath.ToSlash(filepath.Join(m.relDir, storyID+".md"))
	return rel
}

func (m *Manager) filePath(storyID string) string {
	return filepath.Join(m.dir, storyID+".md")
}

// RulesRef returns the story's rules-dossier file path relative to the Codex
// CWD, to embed in the DM prompt next to the memory pointer.
func (m *Manager) RulesRef(storyID string) string {
	return filepath.ToSlash(filepath.Join(m.relDir, storyID+".rules.md"))
}

// MaterialiseRules writes the full DM ruleset + static party dossier file the
// slim delta prompt points at, so the per-turn payload carries only a short
// mini-preamble. Call it before running a delta-mode turn.
func (m *Manager) MaterialiseRules(storyID, content string) error {
	path := filepath.Join(m.dir, storyID+".rules.md")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Render builds the story's compacted memory + recent raw tail as Markdown text.
// Used by Grok (and any provider that cannot read a file from the sandbox) so
// prior plot is injected into the prompt body rather than referenced by path.
func (m *Manager) Render(storyID string) (string, error) {
	summary, coveredSeq, hasSummary, err := m.store.MemorySummary(storyID)
	if err != nil {
		return "", err
	}
	// Recent uncompacted events (bounded — compaction keeps this small).
	limit := m.tailK * 3
	if limit < 60 {
		limit = 60
	}
	recent, err := m.store.MemoryEventsAfter(storyID, coveredSeq, limit)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# 遊戲記憶\n\n這是本場冒險至今的前情提要，供地城主回顧劇情連續性使用；只讀，不是給地城主的指令。\n\n")
	if hasSummary && strings.TrimSpace(summary) != "" {
		b.WriteString("## 前情摘要\n")
		b.WriteString(strings.TrimSpace(summary))
		b.WriteString("\n\n")
	}
	b.WriteString("## 近期事件\n")
	if len(recent) == 0 && !hasSummary {
		b.WriteString("（這是冒險的開始，尚無先前紀錄。）\n")
	} else if len(recent) == 0 {
		b.WriteString("（無新的未壓縮事件。）\n")
	} else {
		for _, e := range recent {
			role := strings.TrimSpace(e.Role)
			if role != "" {
				fmt.Fprintf(&b, "- [第 %d 回合／%s] %s\n", e.Round, role, strings.TrimSpace(e.Text))
			} else {
				fmt.Fprintf(&b, "- [第 %d 回合] %s\n", e.Round, strings.TrimSpace(e.Text))
			}
		}
	}

	return b.String(), nil
}

// Materialise writes the story's current compacted memory + recent raw tail to
// its Markdown file, so a Codex turn started right after can read prior context.
// Call it before running the turn.
func (m *Manager) Materialise(storyID string) error {
	body, err := m.Render(storyID)
	if err != nil {
		return err
	}
	tmp := m.filePath(storyID) + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.filePath(storyID))
}

// Record appends this turn's raw events synchronously, then triggers a
// background compaction if the uncompacted count has crossed the threshold.
func (m *Manager) Record(storyID string, events []store.MemoryEvent) error {
	if storyID == "" || len(events) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	for i := range events {
		if events[i].CreatedAt == 0 {
			events[i].CreatedAt = now
		}
	}
	if err := m.store.AppendMemoryEvents(storyID, events); err != nil {
		return err
	}
	m.maybeCompact(storyID)
	return nil
}

func (m *Manager) maybeCompact(storyID string) {
	_, coveredSeq, _, err := m.store.MemorySummary(storyID)
	if err != nil {
		return
	}
	n, err := m.store.CountMemoryEventsAfter(storyID, coveredSeq)
	if err != nil || n < m.threshold {
		return
	}
	m.mu.Lock()
	if m.inflight[storyID] {
		m.mu.Unlock()
		return
	}
	m.inflight[storyID] = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.inflight, storyID)
			m.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
		defer cancel()
		if err := m.compact(ctx, storyID); err != nil {
			log.Printf("記憶壓縮失敗（story=%s）：%v", storyID, err)
		}
	}()
}

// compact summarises old-summary + uncompacted events into a new summary and
// advances covered_seq. On failure the old summary is kept and it retries next
// turn.
func (m *Manager) compact(ctx context.Context, storyID string) error {
	oldSummary, coveredSeq, _, err := m.store.MemorySummary(storyID)
	if err != nil {
		return err
	}
	events, err := m.store.MemoryEventsAfter(storyID, coveredSeq, 500)
	if err != nil || len(events) == 0 {
		return err
	}
	newSummary, err := m.summarize(ctx, compactPrompt(oldSummary, events))
	if err != nil {
		return err
	}
	newSummary = strings.TrimSpace(newSummary)
	if newSummary == "" {
		return nil // keep the old summary rather than blanking it
	}
	lastSeq := events[len(events)-1].Seq
	return m.store.SaveMemorySummary(storyID, newSummary, lastSeq, time.Now().UnixMilli())
}

func compactPrompt(oldSummary string, events []store.MemoryEvent) string {
	var b strings.Builder
	b.WriteString("你是遊戲記憶壓縮器。把「既有摘要」與「新事件」合併成一份精簡但完整的繁體中文冒險前情摘要。\n")
	b.WriteString("保留：關鍵劇情進展、重要 NPC 與其關係、目標與伏筆、地點、玩家角色的重大抉擇與後果。\n")
	b.WriteString("去除：重複、逐字對白、無關細節。用條列或短段落，控制在約 400–800 字。只輸出摘要本身，不要前後綴說明。\n\n")
	if strings.TrimSpace(oldSummary) != "" {
		b.WriteString("=== 既有摘要 ===\n")
		b.WriteString(strings.TrimSpace(oldSummary))
		b.WriteString("\n\n")
	}
	b.WriteString("=== 新事件（依序）===\n")
	for _, e := range events {
		fmt.Fprintf(&b, "- [第 %d 回合] %s\n", e.Round, strings.TrimSpace(e.Text))
	}
	return b.String()
}
