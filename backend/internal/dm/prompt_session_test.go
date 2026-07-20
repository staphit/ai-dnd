package dm

import "testing"

func TestPromptSessionPlanAndCommit(t *testing.T) {
	s := NewPromptSession()
	s.Reset("story-a")

	// First turn on a live thread: full rules + full body.
	p1 := s.Plan("story-a", true)
	if !p1.FullRules || p1.CompactBody {
		t.Fatalf("first plan: %+v", p1)
	}
	sent := &TurnSnapshot{
		PlayerStable:     map[string]string{"player1": "stable-v1"},
		PlayerVolatile:   map[string]string{"player1": "hp 20"},
		ObjectiveContext: "long context",
	}
	s.Commit("story-a", sent, true, true)

	// Second turn: compact rules + compact body with prev.
	p2 := s.Plan("story-a", true)
	if p2.FullRules || !p2.CompactBody || p2.Prev == nil {
		t.Fatalf("second plan: %+v", p2)
	}
	if p2.Prev.PlayerStable["player1"] != "stable-v1" {
		t.Errorf("prev stable = %q", p2.Prev.PlayerStable["player1"])
	}
	s.Commit("story-a", sent, false, false)

	// Dead thread always full.
	pDead := s.Plan("story-a", false)
	if !pDead.FullRules || pDead.CompactBody {
		t.Fatalf("dead plan: %+v", pDead)
	}

	// Reset after Connect restores full rules.
	s.Reset("story-a")
	p3 := s.Plan("story-a", true)
	if !p3.FullRules || p3.CompactBody {
		t.Fatalf("after reset: %+v", p3)
	}
}

func TestPromptSessionFullRefreshEvery(t *testing.T) {
	s := NewPromptSession()
	s.Reset("s")
	sent := &TurnSnapshot{
		PlayerStable:   map[string]string{"player1": "x"},
		PlayerVolatile: map[string]string{"player1": "y"},
	}
	s.Commit("s", sent, true, true)

	for i := 0; i < fullRefreshEvery; i++ {
		p := s.Plan("s", true)
		if !p.CompactBody {
			t.Fatalf("turn %d: expected compact body", i+1)
		}
		s.Commit("s", sent, false, false)
	}
	// Next plan after fullRefreshEvery compact commits should force full body.
	p := s.Plan("s", true)
	if p.CompactBody {
		t.Fatalf("expected full body after %d compact turns", fullRefreshEvery)
	}
}

func TestPromptSessionNilSafe(t *testing.T) {
	var s *PromptSession
	s.Reset("x")
	p := s.Plan("x", true)
	if !p.FullRules {
		t.Fatalf("nil plan: %+v", p)
	}
	s.Commit("x", &TurnSnapshot{}, true, true)
}
