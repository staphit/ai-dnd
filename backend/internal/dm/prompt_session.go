package dm

import "sync"

// fullRefreshEvery forces a full rules re-send every N compact turns so the
// model (especially stateless-ish multi-turn Grok history) cannot drift too far.
const fullRefreshEvery = 6

// TurnSnapshot is what a turn actually put into the prompt, used to compute the
// next turn's deltas while the same Codex thread is alive.
type TurnSnapshot struct {
	PlayerStable   map[string]string
	PlayerVolatile map[string]string
	Title          string
	Scene          string
	Objective      string
	ObjectiveContext string
	Stakes         string
	Combat         string
}

// RequestOpts configures BuildDMRequest.
type RequestOpts struct {
	DeltaMode bool
	MemRef    string
	// MemoryInline is the materialised plot memory text embedded in the prompt.
	// Used for providers that cannot read a memory file from disk (e.g. Grok CLI/API).
	// When set (and DeltaMode is false), prior plot is injected before recent history.
	MemoryInline string
	// CompactBody omits stable character blocks and long unchanged header fields
	// when Prev has a matching snapshot. Full rules preamble is handled separately
	// by RunDungeonMaster(fullRules).
	CompactBody bool
	Prev        *TurnSnapshot
}

// Plan is the compact/full decision for one DM turn on a live thread.
type Plan struct {
	FullRules   bool
	CompactBody bool
	Prev        *TurnSnapshot
}

// PromptSession tracks what was already sent on the current Codex thread so
// subsequent turns can omit stable bulk. It is bound to one story at a time and
// must be Reset whenever Connect starts a fresh thread.
type PromptSession struct {
	mu             sync.Mutex
	storyID        string
	rulesOnThread  bool
	turnsSinceFull int
	snap           *TurnSnapshot
}

// NewPromptSession returns an empty session (next turn sends full rules + body).
func NewPromptSession() *PromptSession {
	return &PromptSession{}
}

// Reset clears compaction state after Connect (or story rebind). The next turn
// will send the full system preamble and full character sheets.
func (s *PromptSession) Reset(storyID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storyID = storyID
	s.rulesOnThread = false
	s.turnsSinceFull = 0
	s.snap = nil
}

// Plan decides full vs compact for this turn.
// threadAlive must be true only when a live Codex connection is bound to storyID.
func (s *PromptSession) Plan(storyID string, threadAlive bool) Plan {
	if s == nil || !threadAlive {
		return Plan{FullRules: true, CompactBody: false}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.storyID != storyID {
		s.storyID = storyID
		s.rulesOnThread = false
		s.turnsSinceFull = 0
		s.snap = nil
		return Plan{FullRules: true, CompactBody: false}
	}

	fullRules := !s.rulesOnThread
	needFullBody := s.snap == nil || s.turnsSinceFull >= fullRefreshEvery
	if needFullBody {
		return Plan{FullRules: fullRules, CompactBody: false}
	}
	return Plan{
		FullRules:   fullRules,
		CompactBody: true,
		Prev:        cloneSnapshot(s.snap),
	}
}

// Commit records what this turn sent after a successful generation.
func (s *PromptSession) Commit(storyID string, sent *TurnSnapshot, fullRules, fullBody bool) {
	if s == nil || sent == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if storyID != "" {
		s.storyID = storyID
	}
	if fullRules {
		s.rulesOnThread = true
	}
	s.snap = cloneSnapshot(sent)
	if fullBody {
		s.turnsSinceFull = 0
	} else {
		s.turnsSinceFull++
	}
}

func cloneSnapshot(in *TurnSnapshot) *TurnSnapshot {
	if in == nil {
		return nil
	}
	out := &TurnSnapshot{
		Title:            in.Title,
		Scene:            in.Scene,
		Objective:        in.Objective,
		ObjectiveContext: in.ObjectiveContext,
		Stakes:           in.Stakes,
		Combat:           in.Combat,
		PlayerStable:     map[string]string{},
		PlayerVolatile:   map[string]string{},
	}
	for k, v := range in.PlayerStable {
		out.PlayerStable[k] = v
	}
	for k, v := range in.PlayerVolatile {
		out.PlayerVolatile[k] = v
	}
	return out
}
