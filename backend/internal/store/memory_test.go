package store_test

import (
	"testing"

	"dndduet/internal/store"
)

func TestMemoryEventsAppendAndSeq(t *testing.T) {
	st := open(t)
	if err := st.AppendMemoryEvents("s1", []store.MemoryEvent{
		{Round: 1, Role: "player", Text: "甲：檢查符文", CreatedAt: 10},
		{Round: 1, Role: "dm", Text: "符文亮起", CreatedAt: 11},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := st.AppendMemoryEvents("s1", []store.MemoryEvent{{Round: 2, Role: "dm", Text: "門開了", CreatedAt: 12}}); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	seq, err := st.MaxMemorySeq("s1")
	if err != nil || seq != 3 {
		t.Fatalf("MaxMemorySeq = %d, %v; want 3", seq, err)
	}

	// Per-story isolation: another story starts its own seq.
	_ = st.AppendMemoryEvents("s2", []store.MemoryEvent{{Round: 1, Role: "dm", Text: "另一個故事", CreatedAt: 20}})
	if seq2, _ := st.MaxMemorySeq("s2"); seq2 != 1 {
		t.Errorf("story isolation: s2 seq = %d, want 1", seq2)
	}

	all, err := st.MemoryEventsAfter("s1", 0, 10)
	if err != nil || len(all) != 3 || all[0].Text != "甲：檢查符文" || all[2].Text != "門開了" {
		t.Fatalf("EventsAfter = %+v, %v", all, err)
	}
}

func TestMemorySummaryRoundtripAndCount(t *testing.T) {
	st := open(t)
	_ = st.AppendMemoryEvents("s1", []store.MemoryEvent{
		{Round: 1, Role: "dm", Text: "e1", CreatedAt: 1},
		{Round: 1, Role: "dm", Text: "e2", CreatedAt: 2},
		{Round: 2, Role: "dm", Text: "e3", CreatedAt: 3},
	})

	if _, _, ok, _ := st.MemorySummary("s1"); ok {
		t.Fatal("no summary expected yet")
	}
	if n, _ := st.CountMemoryEventsAfter("s1", 0); n != 3 {
		t.Fatalf("count after 0 = %d, want 3", n)
	}

	if err := st.SaveMemorySummary("s1", "壓縮摘要", 2, 100); err != nil {
		t.Fatalf("save summary: %v", err)
	}
	summary, covered, ok, err := st.MemorySummary("s1")
	if err != nil || !ok || summary != "壓縮摘要" || covered != 2 {
		t.Fatalf("summary = %q covered=%d ok=%v err=%v", summary, covered, ok, err)
	}
	// Only events past covered_seq remain uncompacted.
	if n, _ := st.CountMemoryEventsAfter("s1", covered); n != 1 {
		t.Errorf("uncompacted count = %d, want 1", n)
	}

	// Upsert replaces.
	_ = st.SaveMemorySummary("s1", "新摘要", 3, 200)
	if summary, covered, _, _ := st.MemorySummary("s1"); summary != "新摘要" || covered != 3 {
		t.Errorf("upsert failed: %q covered=%d", summary, covered)
	}
}
