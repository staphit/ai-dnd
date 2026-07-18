package game

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// A ScriptModule is a hand-authored branching campaign: a node graph the
// server walks. The DM AI narrates each node from its directive; the server
// owns advancement, alignment bookkeeping, scripted combat and treasure.
type ScriptModule struct {
	ScriptID      string       `json:"scriptId"`
	Entry         string       `json:"entry"`
	GoodThreshold int          `json:"goodThreshold"`
	Notes         string       `json:"notes,omitempty"`
	Nodes         []ScriptNode `json:"nodes"`
	// StageObjectives carries the campaign objective/context/stakes to install
	// when the party crosses into that stage (中期/後期/結局).
	StageObjectives map[string]ScriptObjective `json:"stageObjectives,omitempty"`

	byID map[string]*ScriptNode
}

// ScriptObjective is one stage's mission summary for the campaign header.
type ScriptObjective struct {
	Objective string `json:"objective"`
	Context   string `json:"context"`
	Stakes    string `json:"stakes"`
}

// ScriptNode is one beat of the module: explore, town, combat, boss,
// treasure, or ending.
type ScriptNode struct {
	ID        string `json:"id"`
	Stage     string `json:"stage"` // 前期 | 中期 | 後期 | 結局
	Type      string `json:"type"`  // explore | town | combat | boss | treasure | ending
	Title     string `json:"title"`
	Directive string `json:"directive"`
	// Narration is the pre-written player-facing prose for the local (no-AI)
	// scripted turn resolver; Directive remains the AI-DM instruction.
	Narration  string          `json:"narration,omitempty"`
	Choices    []ScriptChoice  `json:"choices,omitempty"`
	Combat     *ScriptCombat   `json:"combat,omitempty"`
	Treasure   *ScriptTreasure `json:"treasure,omitempty"`
	EndingKind string          `json:"endingKind,omitempty"` // good | bad (ending nodes)
}

// playerText is the prose shown to players when the server narrates a node
// itself: the authored narration, falling back to the directive for modules
// that predate the field.
func (n *ScriptNode) playerText() string {
	if t := strings.TrimSpace(n.Narration); t != "" {
		return t
	}
	return strings.TrimSpace(n.Directive)
}

// ScriptChoice is one player-facing option; picking it moves the campaign to
// Next and shifts the party's alignment toward the good or bad ending.
type ScriptChoice struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Next      string `json:"next"`
	Alignment int    `json:"alignment"`
	CheckHint string `json:"checkHint,omitempty"`
}

// ScriptCombat describes a scripted encounter, scaled to party size by
// appending one AddPerExtraPlayer enemy per adventurer beyond the first.
type ScriptCombat struct {
	Enemies           []ScriptEnemy `json:"enemies"`
	AddPerExtraPlayer []ScriptEnemy `json:"addPerExtraPlayer,omitempty"`
	Intro             string        `json:"intro,omitempty"`
}

// ScriptEnemy mirrors EnemySpec with JSON tags matching the module files.
type ScriptEnemy struct {
	Name            string `json:"name"`
	AC              int    `json:"ac"`
	HP              int    `json:"hp"`
	InitiativeBonus int    `json:"initiativeBonus"`
	AttackBonus     int    `json:"attackBonus"`
	Damage          string `json:"damage"`
	DamageType      string `json:"damageType"`
}

// ScriptTreasure is loot granted when the party enters the node.
type ScriptTreasure struct {
	Gold  int          `json:"gold"`
	Items []ScriptItem `json:"items,omitempty"`
	Intro string       `json:"intro,omitempty"`
}

// ScriptItem is one treasure item; items with weapon dice become attacks.
type ScriptItem struct {
	Name       string `json:"name"`
	Damage     string `json:"damage,omitempty"`
	DamageType string `json:"damageType,omitempty"`
	Properties string `json:"properties,omitempty"`
}

// ScriptState is the per-campaign progress document (script_states table).
type ScriptState struct {
	ScriptID  string   `json:"scriptId"`
	NodeID    string   `json:"nodeId"`
	Alignment int      `json:"alignment"`
	Visited   []string `json:"visited,omitempty"`
	// Taken holds node:choice keys so revisit loops can't farm alignment.
	Taken  []string `json:"taken,omitempty"`
	Ended  bool     `json:"ended,omitempty"`
	Ending string   `json:"ending,omitempty"` // good | bad
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// ScriptProgress is the spoiler-safe slice of script state the View carries.
type ScriptProgress struct {
	ScriptID     string `json:"scriptId"`
	Stage        string `json:"stage"`
	NodeTitle    string `json:"nodeTitle"`
	NodeType     string `json:"nodeType"`
	Alignment    int    `json:"alignment"`
	VisitedCount int    `json:"visitedCount"`
	TotalNodes   int    `json:"totalNodes"`
	Ended        bool   `json:"ended"`
	Ending       string `json:"ending,omitempty"`
}

//go:embed scripts/*.json
var scriptFS embed.FS

// scriptModules holds every embedded module keyed by scriptId (= storyId).
var scriptModules = func() map[string]*ScriptModule {
	mods := map[string]*ScriptModule{}
	entries, err := scriptFS.ReadDir("scripts")
	if err != nil {
		panic(fmt.Sprintf("script modules: %v", err))
	}
	for _, e := range entries {
		data, err := scriptFS.ReadFile(path.Join("scripts", e.Name()))
		if err != nil {
			panic(fmt.Sprintf("script module %s: %v", e.Name(), err))
		}
		mod := &ScriptModule{}
		if err := json.Unmarshal(data, mod); err != nil {
			panic(fmt.Sprintf("script module %s: %v", e.Name(), err))
		}
		if err := mod.compile(); err != nil {
			panic(fmt.Sprintf("script module %s: %v", e.Name(), err))
		}
		if _, duplicate := mods[mod.ScriptID]; duplicate {
			panic(fmt.Sprintf("script module %s: duplicate scriptId %q", e.Name(), mod.ScriptID))
		}
		mods[mod.ScriptID] = mod
	}
	return mods
}()

// compile indexes nodes and validates the graph so a broken module fails at
// startup, not mid-campaign.
func (m *ScriptModule) compile() error {
	if strings.TrimSpace(m.ScriptID) == "" || strings.TrimSpace(m.Entry) == "" || len(m.Nodes) == 0 {
		return fmt.Errorf("scriptId, entry and nodes are required")
	}
	validStages := map[string]bool{"前期": true, "中期": true, "後期": true, "結局": true}
	validTypes := map[string]bool{"explore": true, "town": true, "combat": true, "boss": true, "treasure": true, "ending": true}
	m.byID = make(map[string]*ScriptNode, len(m.Nodes))
	for i := range m.Nodes {
		n := &m.Nodes[i]
		if strings.TrimSpace(n.ID) == "" {
			return fmt.Errorf("node %d has no id", i)
		}
		if _, dup := m.byID[n.ID]; dup {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		if !validStages[n.Stage] {
			return fmt.Errorf("node %q has invalid stage %q", n.ID, n.Stage)
		}
		if !validTypes[n.Type] {
			return fmt.Errorf("node %q has invalid type %q", n.ID, n.Type)
		}
		if strings.TrimSpace(n.Title) == "" || strings.TrimSpace(n.Directive) == "" {
			return fmt.Errorf("node %q needs title and directive", n.ID)
		}
		m.byID[n.ID] = n
	}
	if _, ok := m.byID[m.Entry]; !ok {
		return fmt.Errorf("entry node %q missing", m.Entry)
	}
	for i := range m.Nodes {
		n := &m.Nodes[i]
		if n.Type == "ending" {
			if n.Stage != "結局" {
				return fmt.Errorf("ending node %q must use 結局 stage", n.ID)
			}
			if n.EndingKind != "good" && n.EndingKind != "bad" && n.EndingKind != "neutral" {
				return fmt.Errorf("ending node %q needs endingKind good|bad|neutral", n.ID)
			}
			if len(n.Choices) != 0 {
				return fmt.Errorf("ending node %q must not have choices", n.ID)
			}
			continue
		}
		if len(n.Choices) == 0 {
			return fmt.Errorf("node %q has no choices", n.ID)
		}
		choiceIDs := make(map[string]bool, len(n.Choices))
		for _, c := range n.Choices {
			if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Text) == "" || strings.TrimSpace(c.Next) == "" {
				return fmt.Errorf("node %q has a choice missing id, text or next", n.ID)
			}
			if choiceIDs[c.ID] {
				return fmt.Errorf("node %q has duplicate choice id %q", n.ID, c.ID)
			}
			choiceIDs[c.ID] = true
			if _, ok := m.byID[c.Next]; !ok {
				return fmt.Errorf("node %q choice %q points at missing node %q", n.ID, c.ID, c.Next)
			}
		}
	}

	// Every authored node must be reachable from entry; otherwise content can
	// silently ship but never be played.
	reachable := map[string]bool{m.Entry: true}
	queue := []string{m.Entry}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, choice := range m.byID[id].Choices {
			if !reachable[choice.Next] {
				reachable[choice.Next] = true
				queue = append(queue, choice.Next)
			}
		}
	}
	for _, n := range m.Nodes {
		if !reachable[n.ID] {
			return fmt.Errorf("node %q is unreachable from entry %q", n.ID, m.Entry)
		}
	}

	// Reverse-walk from all endings. This allows intentional loops, but rejects
	// a branch trapped in a component that can never finish the adventure.
	reverse := make(map[string][]string, len(m.Nodes))
	canEnd := map[string]bool{}
	queue = queue[:0]
	for _, n := range m.Nodes {
		if n.Type == "ending" {
			canEnd[n.ID] = true
			queue = append(queue, n.ID)
		}
		for _, choice := range n.Choices {
			reverse[choice.Next] = append(reverse[choice.Next], n.ID)
		}
	}
	if len(queue) == 0 {
		return fmt.Errorf("module has no ending node")
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, previous := range reverse[id] {
			if !canEnd[previous] {
				canEnd[previous] = true
				queue = append(queue, previous)
			}
		}
	}
	for _, n := range m.Nodes {
		if !canEnd[n.ID] {
			return fmt.Errorf("node %q cannot reach an ending", n.ID)
		}
	}

	for stage, objective := range m.StageObjectives {
		if !validStages[stage] {
			return fmt.Errorf("stageObjectives has invalid stage %q", stage)
		}
		if strings.TrimSpace(objective.Objective) == "" || strings.TrimSpace(objective.Context) == "" || strings.TrimSpace(objective.Stakes) == "" {
			return fmt.Errorf("stageObjectives %q needs objective, context and stakes", stage)
		}
	}
	return nil
}

func (m *ScriptModule) node(id string) *ScriptNode {
	if m == nil {
		return nil
	}
	return m.byID[id]
}

// scriptModuleFor resolves the module for a state document, nil when the
// module no longer exists (state is then ignored, campaign turns freeform).
func scriptModuleFor(state *ScriptState) *ScriptModule {
	if state == nil {
		return nil
	}
	return scriptModules[state.ScriptID]
}

// ScriptedStoryIDs lists the story presets that ship a scripted module, so
// the setup UI can offer the scripted-vs-freeform choice.
func ScriptedStoryIDs() []string {
	ids := make([]string, 0, len(scriptModules))
	for id := range scriptModules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// newScriptState starts a module at its entry node; nil when storyID has no
// scripted module (the campaign stays freeform AI-driven).
func newScriptState(storyID string) *ScriptState {
	mod, ok := scriptModules[strings.TrimSpace(storyID)]
	if !ok {
		return nil
	}
	return &ScriptState{ScriptID: mod.ScriptID, NodeID: mod.Entry}
}

// scaledEnemies converts the scripted encounter into EnemySpecs sized for the
// party: base enemies plus one extra per adventurer beyond the first.
func (c *ScriptCombat) scaledEnemies(partySize int) []EnemySpec {
	if c == nil {
		return nil
	}
	specs := make([]EnemySpec, 0, len(c.Enemies)+partySize)
	add := func(e ScriptEnemy) {
		specs = append(specs, EnemySpec{
			Name: e.Name, AC: e.AC, HP: e.HP, InitiativeBonus: e.InitiativeBonus,
			AttackBonus: e.AttackBonus, Damage: e.Damage, DamageType: e.DamageType,
		})
	}
	for _, e := range c.Enemies {
		add(e)
	}
	for i := 0; i < partySize-1 && i < len(c.AddPerExtraPlayer); i++ {
		add(c.AddPerExtraPlayer[i])
	}
	return specs
}

// matchScriptChoice resolves the DM's chosenOption signal (preferred) or an
// exact player-declared choice text (the frontend sends clicked choices
// verbatim) to one of the current node's options. Actions arrive in party
// order so conflicting clicks resolve deterministically.
func matchScriptChoice(node *ScriptNode, chosenOption string, orderedActions []string) *ScriptChoice {
	if node == nil {
		return nil
	}
	if opt := strings.TrimSpace(chosenOption); opt != "" {
		for i := range node.Choices {
			if strings.EqualFold(node.Choices[i].ID, opt) {
				return &node.Choices[i]
			}
		}
	}
	for _, text := range orderedActions {
		t := strings.TrimSpace(text)
		if t == "" {
			continue
		}
		for i := range node.Choices {
			if t == strings.TrimSpace(node.Choices[i].Text) {
				return &node.Choices[i]
			}
		}
	}
	return nil
}

// scriptPromptLines renders the per-turn director's notes for the DM prompt,
// mirroring how arcPromptLines feeds pacing directives.
func scriptPromptLines(mod *ScriptModule, state *ScriptState, combatActive bool) []string {
	if mod == nil || state == nil {
		return nil
	}
	node := mod.node(state.NodeID)
	if node == nil {
		return nil
	}
	lines := []string{fmt.Sprintf("劇本模式：本戰役依既定劇本進行，目前節點「%s」（%s）。以下導演指示必須在敘事中落實，但不可剝奪玩家的行動自由。", node.Title, node.Stage)}
	if d := strings.TrimSpace(node.Directive); d != "" {
		lines = append(lines, "導演指示："+d)
	}
	if state.Ended || node.Type == "ending" {
		kind := "壞結局"
		if node.EndingKind == "good" || state.Ending == "good" {
			kind = "好結局"
		}
		lines = append(lines,
			fmt.Sprintf("劇本已抵達%s。請依導演指示給出完整、有餘韻的結局敘事，收束所有主要角色與線索，不要開啟新事件；script.chosenOption 回空字串。", kind))
		return lines
	}
	if combatActive {
		lines = append(lines, "劇本戰鬥進行中：由戰鬥追蹤器結算勝負，你只負責戰況敘事；本回合 script.chosenOption 回空字串。")
		return lines
	}
	if node.Type == "combat" || node.Type == "boss" {
		intro := ""
		if node.Combat != nil {
			intro = strings.TrimSpace(node.Combat.Intro)
		}
		if intro != "" {
			lines = append(lines, "本節點的戰鬥已由系統開啟或即將開啟："+intro)
		}
	}
	var opts []string
	for _, c := range node.Choices {
		hint := ""
		if strings.TrimSpace(c.CheckHint) != "" {
			hint = "（可能需要檢定：" + c.CheckHint + "）"
		}
		opts = append(opts, fmt.Sprintf("%s. %s%s", c.ID, c.Text, hint))
	}
	if len(opts) > 0 {
		lines = append(lines,
			"本節點的劇本選項："+strings.Join(opts, "；")+"。",
			"玩家可以自由行動，但當某位玩家本回合的行動實質上執行了上述某個選項、且其結果已在本回合敘事中落定（含必要檢定已解決），就在 script.chosenOption 填該選項代號（如 A）；否則填空字串。一次最多推進一個選項。",
			"你輸出的 choices 應包含這些劇本選項的內容（可依角色改寫措辭），並可另加至多 2 個貼合當前場景的自由建議。")
	}
	if node.Type == "town" {
		lines = append(lines, "這是城鎮節點：明確讓玩家知道可以在此購買裝備補給（系統設有商店）、休息恢復、與居民打聽情報。")
	}
	return lines
}

// advanceScript applies one chosen option: moves to the next node and returns
// the system-log lines describing what the server settled. Alignment counts
// only the first time a given option is taken (revisit loops can't farm it),
// and a good ending demands the accumulated alignment reach the module's
// threshold — otherwise the same door leads to the fall. Side effects on the
// entered node (treasure, scripted combat) are the caller's job via the
// returned node, gated on first entry.
func advanceScript(mod *ScriptModule, state *ScriptState, choice *ScriptChoice) (*ScriptNode, []string) {
	if mod == nil || state == nil || choice == nil || state.Ended {
		return nil, nil
	}
	from := mod.node(state.NodeID)
	next := mod.node(choice.Next)
	if from == nil || next == nil {
		return nil, nil
	}
	takenKey := from.ID + ":" + choice.ID
	if !containsStr(state.Taken, takenKey) {
		state.Taken = append(state.Taken, takenKey)
		state.Alignment += choice.Alignment
	}
	var logs []string
	// The good ending must be earned: below the threshold the party's past
	// choices twist even the right final move into a bad ending on the same
	// node (module directives narrate the corruption).
	if next.Type == "ending" && next.EndingKind == "good" && state.Alignment < mod.GoodThreshold {
		for _, alt := range from.Choices {
			if fallen := mod.node(alt.Next); fallen != nil && fallen.Type == "ending" && fallen.EndingKind == "bad" {
				next = fallen
				logs = append(logs, "命運的重量壓過了最後的抉擇：過往的選擇已將結局引向幽暗。")
				break
			}
		}
	}
	// The transition itself stays out of the dialogue feed (the narration
	// already carries the story); only endings leave a journal milestone.
	state.Visited = append(state.Visited, state.NodeID)
	state.NodeID = next.ID
	if next.Type == "ending" {
		state.Ended = true
		state.Ending = next.EndingKind
		logs = append(logs, fmt.Sprintf("劇本抵達%s「%s」。", endingKindLabel(next.EndingKind), next.Title))
	}
	return next, logs
}

// endingKindLabel names an ending kind for logs and local narration.
func endingKindLabel(kind string) string {
	switch kind {
	case "good":
		return "光明結局"
	case "neutral":
		return "蒼灰結局"
	default:
		return "沉沒結局"
	}
}

// scriptStageIndex maps a module stage to its story-arc phase index.
func scriptStageIndex(stage string) int {
	switch stage {
	case "中期":
		return 1
	case "後期":
		return 2
	case "結局":
		return 3
	default:
		return 0
	}
}

// stageBudgets derives per-act round deadlines from the module's size:
// roughly one round per node with half again for detours, so the mission
// panel's clock matches the script instead of the generic 20/40/60.
func (m *ScriptModule) stageBudgets() [3]int {
	counts := map[string]int{}
	for i := range m.Nodes {
		counts[m.Nodes[i].Stage]++
	}
	budget := func(n int) int {
		b := (n*3 + 1) / 2
		if b < 15 {
			b = 15
		}
		return b
	}
	d0 := budget(counts["前期"])
	d1 := d0 + budget(counts["中期"])
	d2 := d1 + budget(counts["後期"]+counts["結局"])
	return [3]int{d0, d1, d2}
}

// syncScriptArc aligns a scripted campaign's story arc with the module: goals
// come from stageObjectives, deadlines from stage size, and Current is
// fast-forwarded to the current node's stage (self-healing campaigns from
// before stage sync existed). It never grants XP — timed rewards only flow
// through live phase-completion events.
func syncScriptArc(arc *StoryArc, mod *ScriptModule, state *ScriptState, round int) {
	if arc == nil || mod == nil || state == nil {
		return
	}
	node := mod.node(state.NodeID)
	if node == nil || len(arc.Phases) < 3 {
		return
	}
	deadlines := mod.stageBudgets()
	for i := 0; i < 3; i++ {
		arc.Phases[i].DeadlineRound = deadlines[i]
	}
	if so, ok := mod.StageObjectives["中期"]; ok && so.Objective != "" {
		arc.Phases[1].Goal = so.Objective
	}
	if so, ok := mod.StageObjectives["後期"]; ok && so.Objective != "" {
		arc.Phases[2].Goal = so.Objective
	}
	expected := scriptStageIndex(node.Stage)
	if state.Ended && expected < 3 {
		expected = 3
	}
	for arc.Current < expected && arc.Current < len(arc.Phases) {
		p := &arc.Phases[arc.Current]
		if p.CompletedRound == 0 {
			p.CompletedRound = round
		}
		arc.Current++
	}
	if arc.Current >= len(arc.Phases) {
		arc.Ended = true
	}
}

// scriptProgress builds the spoiler-safe View slice.
func scriptProgress(state *ScriptState) *ScriptProgress {
	mod := scriptModuleFor(state)
	if mod == nil {
		return nil
	}
	node := mod.node(state.NodeID)
	if node == nil {
		return nil
	}
	unique := make(map[string]bool, len(state.Visited))
	for _, id := range state.Visited {
		unique[id] = true
	}
	return &ScriptProgress{
		ScriptID:     state.ScriptID,
		Stage:        node.Stage,
		NodeTitle:    node.Title,
		NodeType:     node.Type,
		Alignment:    state.Alignment,
		VisitedCount: len(unique),
		TotalNodes:   len(mod.Nodes),
		Ended:        state.Ended,
		Ending:       state.Ending,
	}
}
