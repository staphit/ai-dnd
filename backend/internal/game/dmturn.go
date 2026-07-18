package game

import (
	"fmt"
	"regexp"
	"strings"

	"dndduet/internal/apperr"
	"dndduet/internal/dm"
	"dndduet/internal/rules"
	"dndduet/internal/store"
)

// lootDicePattern accepts weapon damage expressions on loot items (1d8, 2d6+1).
var lootDicePattern = regexp.MustCompile(`^[0-9]+d[0-9]+([+-][0-9]+)?$`)

// clampCheckDC keeps every required check winnable but never trivial: the
// needed natural roll stays in [3, 15], i.e. a 30%–90% success chance for the
// character actually rolling (natural + modifier >= DC).
func clampCheckDC(dc, modifier int) int {
	if low := modifier + 3; dc < low {
		return low
	}
	if high := modifier + 15; dc > high {
		return high
	}
	return dc
}

// CapabilityDigest renders one character as a single prompt line: enough for
// the AI to tailor narration and choices, nothing it needs for validation
// (the server owns legality). ~60-80 CJK tokens per player instead of the old
// multi-line snapshot.
func CapabilityDigest(c rules.Character) string {
	classLine := fmt.Sprintf("%s%d", c.ClassName, c.Level)
	if strings.TrimSpace(c.Subclass) != "" {
		classLine += "／" + c.Subclass
	}
	parts := []string{
		c.Name,
		classLine,
		fmt.Sprintf("HP %d/%d", c.HP, c.MaxHP),
		fmt.Sprintf("AC %d", c.AC),
		fmt.Sprintf("金幣 %d", c.Gold),
	}
	if c.TemporaryHP > 0 {
		parts[2] += fmt.Sprintf("（另有暫時 %d）", c.TemporaryHP)
	}

	if c.Spellcasting != nil {
		var slots []string
		for _, slot := range c.Spellcasting.Slots {
			if slot.Max > 0 {
				slots = append(slots, fmt.Sprintf("%d環%d/%d", slot.Level, slot.Current, slot.Max))
			}
		}
		if len(slots) > 0 {
			parts = append(parts, "法術位 "+strings.Join(slots, "、"))
		}
	}

	var resources []string
	for i, r := range c.Resources {
		if i >= 6 {
			break
		}
		resources = append(resources, fmt.Sprintf("%s%d/%d", r.Name, r.Current, r.Max))
	}
	if len(resources) > 0 {
		parts = append(parts, "資源 "+strings.Join(resources, "、"))
	}

	if c.Spellcasting != nil {
		var spells []string
		truncated := false
		for _, sp := range c.Spellcasting.Spells {
			if sp.Level != 0 && !sp.Prepared && !sp.AlwaysPrepared {
				continue
			}
			if len(spells) >= 12 {
				truncated = true
				break
			}
			spells = append(spells, sp.Name)
		}
		if len(spells) > 0 {
			suffix := ""
			if truncated {
				suffix = "等"
			}
			parts = append(parts, "可施法術："+strings.Join(spells, "、")+suffix)
		}
	}

	if strings.TrimSpace(c.Concentration) != "" {
		parts = append(parts, "專注："+clampStr(c.Concentration, 40))
	}
	if c.Condition != "" && c.Condition != "正常" {
		parts = append(parts, "狀態 "+clampStr(c.Condition, 40))
	}
	return strings.Join(parts, "｜")
}

func sanitizedPlayers(players []rules.Character) []dm.SanitizedPlayer {
	out := make([]dm.SanitizedPlayer, 0, len(players))
	for _, p := range players {
		subclass := p.Subclass
		out = append(out, dm.SanitizedPlayer{
			ID:        p.ID,
			Name:      p.Name,
			ClassName: p.ClassName,
			Subclass:  subclass,
			Summary:   CapabilityDigest(p),
		})
	}
	return out
}

func combatLine(combat *rules.CombatState) string {
	if combat == nil || !combat.Active {
		return ""
	}
	parts := make([]string, 0, len(combat.Combatants))
	for _, c := range combat.Combatants {
		parts = append(parts, fmt.Sprintf("%s HP %d/%d AC %d 先攻 %d", c.Name, c.HP, c.MaxHP, c.AC, c.Initiative))
	}
	return fmt.Sprintf("戰鬥第 %d 輪：%s", combat.Round, strings.Join(parts, "；"))
}

func choiceTexts(choices []rules.Choice) string {
	var texts []string
	for _, c := range choices {
		if t := strings.TrimSpace(c.Text); t != "" {
			texts = append(texts, t)
		}
	}
	return strings.Join(texts, "；")
}

// PreparedDMTurn is a ready-to-run DM turn: the prompt input plus what the
// apply step and memory recorder need afterwards.
type PreparedDMTurn struct {
	Input          dm.TurnInputV2
	Players        []dm.SanitizedPlayer
	Actions        map[string]string
	Round          int
	IsContinuation bool
	// TurnToken owns the single in-flight lease; ExpectedVersion is the campaign
	// document version captured after prepare-time writes.
	TurnToken       string
	ExpectedVersion int
	// InspirationPlayerID names the player whose queued 吟遊激勵 die was rolled
	// into this check's modifier; the die is only consumed when the turn is
	// applied, so a failed AI call doesn't burn it.
	InspirationPlayerID string
}

func (s *Service) prepareBase(st *gameState) dm.TurnInputV2 {
	choices := []rules.Choice{}
	if st.row.Choices != "" {
		_ = jsonUnmarshal(st.row.Choices, &choices)
	}
	combatActive := st.combat != nil && st.combat.Active
	return dm.TurnInputV2{
		Title:            st.row.Title,
		Scene:            st.row.Scene,
		Objective:        st.row.Objective,
		ObjectiveContext: st.row.ObjectiveContext,
		Stakes:           st.row.Stakes,
		Round:            st.row.Round,
		PrevChoices:      choiceTexts(choices),
		CombatLine:       combatLine(st.combat),
		ArcLines:         arcPromptLines(st.arc, st.row.Round),
		ScriptLines:      scriptPromptLines(scriptModuleFor(st.script), st.script, combatActive),
		Players:          sanitizedPlayers(st.players),
	}
}

// PrepareActionsTurn merges the submitted actions into the pending lock,
// requires a declaration from every player, and runs mechanical validation.
// The AI is never called for a mechanically illegal round.
func (s *Service) PrepareActionsTurn(id string, actions map[string]string, intents map[string]Intent) (PreparedDMTurn, error) {
	unlock := s.Lock(id)
	defer unlock()
	token, err := s.reserveDMTurn(id)
	if err != nil {
		return PreparedDMTurn{}, err
	}
	preparedOK := false
	defer func() {
		if !preparedOK {
			s.AbortDMTurn(id, token)
		}
	}()
	st, err := s.loadState(id)
	if err != nil {
		return PreparedDMTurn{}, err
	}
	if len(st.players) == 0 {
		return PreparedDMTurn{}, apperr.New(400, "需要隊伍中每位玩家的行動才能進行裁定。")
	}

	merged := map[string]string{}
	for pid, text := range st.pending {
		merged[pid] = text
	}
	for pid, text := range actions {
		if t := strings.TrimSpace(text); t != "" {
			merged[pid] = clampStr(t, 2000)
		}
	}
	for _, p := range st.players {
		if strings.TrimSpace(merged[p.ID]) == "" {
			return PreparedDMTurn{}, apperr.New(400, "需要隊伍中每位玩家的行動才能進行裁定。")
		}
	}

	if issues := s.validateActions(st, merged, intents); len(issues) > 0 {
		return PreparedDMTurn{}, &ActionIssuesError{Issues: issues}
	}

	// Persist the merged pending lock so a failed AI call keeps declarations.
	if data, err := jsonMarshal(merged); err == nil {
		st.row.Pending = data
		if st.row, err = s.touch(st.row); err != nil {
			return PreparedDMTurn{}, err
		}
	}

	input := s.prepareBase(st)
	input.Actions = merged
	preparedOK = true
	return PreparedDMTurn{
		Input: input, Players: input.Players, Actions: merged, Round: st.row.Round,
		TurnToken: token, ExpectedVersion: st.row.DocVersion,
	}, nil
}

// PrepareCheckTurn turns a player's natural d20 roll into the resolution
// continuation, computing modifier/total/success from the stored check and
// the authoritative sheet. The stored check is cleared only when the turn is
// applied, so a failed AI call keeps the dice tray up.
func (s *Service) PrepareCheckTurn(id string, natural int) (PreparedDMTurn, error) {
	unlock := s.Lock(id)
	defer unlock()
	token, err := s.reserveDMTurn(id)
	if err != nil {
		return PreparedDMTurn{}, err
	}
	preparedOK := false
	defer func() {
		if !preparedOK {
			s.AbortDMTurn(id, token)
		}
	}()
	st, err := s.loadState(id)
	if err != nil {
		return PreparedDMTurn{}, err
	}
	if st.check == nil {
		return PreparedDMTurn{}, apperr.New(400, "目前沒有待完成的必要檢定。")
	}
	if natural < 1 {
		natural = 1
	}
	if natural > 20 {
		natural = 20
	}

	var actor *rules.Character
	if st.check.PlayerID != "" {
		for i := range st.players {
			if st.players[i].ID == st.check.PlayerID {
				actor = &st.players[i]
				break
			}
		}
	}
	if actor == nil {
		for i := range st.players {
			if strings.Contains(st.check.Character, st.players[i].Name) || strings.Contains(st.players[i].Name, st.check.Character) {
				actor = &st.players[i]
				break
			}
		}
	}
	modifier := st.check.Modifier
	if actor != nil {
		if m := rules.GetCheckBonus(*actor, st.check.Skill); m != 0 {
			modifier = m
		} else if m := rules.GetCheckBonus(*actor, st.check.Ability); m != 0 {
			modifier = m
		}
	}
	// 吟遊激勵 (bardic inspiration): a queued inspiration die from any party
	// member adds a server-rolled d6 to this required check. Nothing is
	// mutated here — the die is consumed in ApplyDMTurn so a failed AI call
	// keeps it queued (a retry just rolls the d6 again).
	skillLabel := st.check.Skill
	inspirationPlayer := ""
	for i := range st.players {
		if st.players[i].PendingInspiration > 0 {
			bonus := rules.Die(6, s.dice)
			modifier += bonus
			skillLabel = fmt.Sprintf("%s（含吟遊激勵+%d）", skillLabel, bonus)
			inspirationPlayer = st.players[i].ID
			break
		}
	}
	total := natural + modifier

	input := s.prepareBase(st)
	input.Resolution = &dm.ResolutionV2{
		Character: st.check.Character,
		Ability:   st.check.Ability,
		Skill:     skillLabel,
		Reason:    st.check.Reason,
		DC:        st.check.DC,
		Natural:   natural,
		Modifier:  modifier,
		Total:     total,
		Success:   total >= st.check.DC,
	}
	preparedOK = true
	return PreparedDMTurn{
		Input: input, Players: input.Players, Round: st.row.Round, IsContinuation: true,
		InspirationPlayerID: inspirationPlayer, TurnToken: token, ExpectedVersion: st.row.DocVersion,
	}, nil
}

// PrepareConclusionTurn builds the post-combat narration turn from a
// conclusion the server computed in Conclude.
func (s *Service) PrepareConclusionTurn(id, outcome, summary string, final bool) (PreparedDMTurn, error) {
	unlock := s.Lock(id)
	defer unlock()
	token, err := s.reserveDMTurn(id)
	if err != nil {
		return PreparedDMTurn{}, err
	}
	preparedOK := false
	defer func() {
		if !preparedOK {
			s.AbortDMTurn(id, token)
		}
	}()
	st, err := s.loadState(id)
	if err != nil {
		return PreparedDMTurn{}, err
	}
	if outcome != "victory" && outcome != "defeat" && outcome != "withdrawal" {
		outcome = "withdrawal"
	}
	input := s.prepareBase(st)
	input.Conclusion = &dm.ConclusionV2{Outcome: outcome, Summary: clampStr(strings.TrimSpace(summary), 3000), Final: final}
	preparedOK = true
	return PreparedDMTurn{
		Input: input, Players: input.Players, Round: st.row.Round, IsContinuation: true,
		TurnToken: token, ExpectedVersion: st.row.DocVersion,
	}, nil
}

// StageClear announces a completed act: the scripted stage boundary just
// crossed (or a freeform arc phase completed), for the frontend's success
// popup.
type StageClear struct {
	Cleared string `json:"cleared"` // the act that just completed (前期/中期/後期)
	Next    string `json:"next"`    // the act now entered (中期/後期/結局)
	Title   string `json:"title"`   // node title or phase goal for context
}

// ApplyResult is the outcome of applying a DM turn.
type ApplyResult struct {
	View       View
	Rejected   []ActionIssue // non-empty when the AI vetoed actions narratively
	StageClear *StageClear   `json:"stageClear,omitempty"`
}

// ApplyDMTurn ports the App.tsx post-turn state update: narrative action
// rejection, effects, XP, combat sync/start, campaign meta, journal entries,
// pending/choices/requiredCheck. Runs under the campaign lock on fresh state.
func (s *Service) ApplyDMTurn(id string, prepared PreparedDMTurn, turn *dm.Turn) (ApplyResult, error) {
	unlock := s.Lock(id)
	defer unlock()
	defer s.AbortDMTurn(id, prepared.TurnToken)
	if !s.dmTurnCurrent(id, prepared.TurnToken) {
		return ApplyResult{}, apperr.New(409, "這個地城主回合已失效，請重新送出目前行動。")
	}
	st, err := s.loadState(id)
	if err != nil {
		return ApplyResult{}, err
	}
	if st.row.DocVersion != prepared.ExpectedVersion {
		return ApplyResult{}, apperr.New(409, "戰役在地城主思考期間已更新；舊裁定未套用，請重新送出目前行動。")
	}

	// Narrative action rejection: unlock the vetoed players, log, stop.
	if len(turn.ActionIssues) > 0 {
		if prepared.IsContinuation {
			return ApplyResult{}, apperr.New(503, "DM 未直接接續先前結果，請再試一次。")
		}
		var details []string
		var rejected []ActionIssue
		for _, issue := range turn.ActionIssues {
			delete(st.pending, issue.PlayerID)
			name := issue.PlayerID
			for _, p := range st.players {
				if p.ID == issue.PlayerID {
					name = p.Name
					break
				}
			}
			details = append(details, name+"："+issue.Message)
			rejected = append(rejected, ActionIssue{PlayerID: issue.PlayerID, Message: issue.Message})
		}
		view, err := s.persistEntries(st, []store.StoryRow{{Speaker: "dm", Text: "【行動駁回】" + strings.Join(details, "；") + "。故事尚未推進；請依照理由修改後重新鎖定。"}})
		if err != nil {
			return ApplyResult{}, err
		}
		return ApplyResult{View: view, Rejected: rejected}, nil
	}

	var entries []store.StoryRow
	if !prepared.IsContinuation {
		for _, p := range st.players {
			text := prepared.Actions[p.ID]
			if strings.TrimSpace(text) == "" {
				text = "本回合不行動，保持警戒。"
			}
			entries = append(entries, store.StoryRow{Speaker: p.ID, Text: text})
		}
	}

	// The 吟遊激勵 die rolled into this check's modifier is consumed only now
	// that the turn actually applied.
	if prepared.InspirationPlayerID != "" {
		for i := range st.players {
			if st.players[i].ID == prepared.InspirationPlayerID && st.players[i].PendingInspiration > 0 {
				st.players[i].PendingInspiration = 0
				entries = append(entries, store.StoryRow{Speaker: "system", Text: st.players[i].Name + "的「吟遊激勵」已用於這次檢定。"})
				break
			}
		}
	}
	entries = append(entries, store.StoryRow{Speaker: "dm", Text: turn.Narration})
	for _, pm := range turn.PrivateMessages {
		entries = append(entries, store.StoryRow{Speaker: "dm", Audience: pm.PlayerID, Text: pm.Text})
	}

	// Effects and XP are skipped on combat-conclusion turns (the tracker
	// already settled them), mirroring App.tsx advance().
	isConclusion := prepared.Input.Conclusion != nil
	if !isConclusion && len(turn.Effects) > 0 {
		effects := make([]rules.DMEffect, 0, len(turn.Effects))
		for _, e := range turn.Effects {
			effects = append(effects, rules.DMEffect{TargetID: e.TargetID, Kind: e.Kind, Amount: e.Amount, Condition: e.Condition, Reason: e.Reason})
		}
		settled, logs := rules.ApplyDmEffects(st.players, effects)
		st.players = settled
		for _, l := range logs {
			entries = append(entries, store.StoryRow{Speaker: "system", Text: "自動結算：" + l})
		}
	}
	if !isConclusion {
		for _, award := range turn.ExperienceAwards {
			if award.Amount <= 0 {
				continue
			}
			for i := range st.players {
				if st.players[i].ID == award.PlayerID {
					st.players[i] = rules.GrantExperience(st.players[i], award.Amount)
					entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("%s獲得 %d XP：%s", st.players[i].Name, award.Amount, award.Reason)})
					break
				}
			}
		}
	}

	// Treasure: gold splits evenly across the party (remainder to the front of
	// the marching order); items land in the named player's equipment.
	if turn.Loot.Gold > 0 && len(st.players) > 0 {
		share := turn.Loot.Gold / len(st.players)
		extra := turn.Loot.Gold % len(st.players)
		for i := range st.players {
			gain := share
			if i < extra {
				gain++
			}
			st.players[i].Gold += gain
		}
		entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("拾獲 %d 金幣，已平分給隊伍。", turn.Loot.Gold)})
	}
	for _, item := range turn.Loot.Items {
		for i := range st.players {
			if st.players[i].ID != item.PlayerID {
				continue
			}
			p := &st.players[i]
			owned := false
			for _, e := range p.Equipment {
				if e == item.Name {
					owned = true
					break
				}
			}
			if !owned {
				p.Equipment = append(p.Equipment, item.Name)
				entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("%s獲得物品：%s。", p.Name, item.Name)})
			}
			// Items with weapon stats become usable attacks (also the path for
			// "identifying" an already-carried story item: same name + stats).
			if lootDicePattern.MatchString(item.Damage) {
				hasAttack := false
				for _, a := range p.Attacks {
					if a.Name == item.Name {
						hasAttack = true
						break
					}
				}
				if !hasAttack {
					damageType := item.DamageType
					if damageType == "" {
						damageType = "揮砍"
					}
					p.Attacks = append(p.Attacks, rules.Attack{
						ID: "loot-" + item.Name, Name: item.Name,
						Damage: item.Damage, DamageType: damageType, Properties: item.Properties,
					})
					*p = rules.Recalculate(*p)
					entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("「%s」帶有武器數值（%s %s），已加入%s的攻擊選項。", item.Name, item.Damage, damageType, p.Name)})
				}
			}
			break
		}
	}

	// Story-arc phase completion: the server stamps the round, pays the timed
	// reward, and advances the campaign to its next act.
	var stageClear *StageClear
	if turn.Arc.PhaseComplete && st.arc != nil {
		if p := st.arc.phase(); p != nil {
			p.CompletedRound = st.row.Round
			if st.row.Round <= p.DeadlineRound {
				p.RewardGranted = true
				for i := range st.players {
					st.players[i] = rules.GrantExperience(st.players[i], p.RewardXP)
				}
				entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("階段達成：%s目標「%s」於期限內完成（第 %d/%d 回合）！每位角色獲得 %d XP。", p.Stage, p.Goal, st.row.Round, p.DeadlineRound, p.RewardXP)})
			} else {
				entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("階段達成：%s目標「%s」完成，但已超過期限（第 %d 回合／期限第 %d 回合），沒有限時獎勵。", p.Stage, p.Goal, st.row.Round, p.DeadlineRound)})
			}
			st.arc.Current++
			nextStage := "結局"
			if next := st.arc.phase(); next != nil {
				nextStage = next.Stage
			}
			stageClear = &StageClear{Cleared: p.Stage, Next: nextStage, Title: p.Goal}
			if next := st.arc.phase(); next != nil {
				goal := strings.TrimSpace(turn.Arc.NextGoal)
				if goal == "" {
					goal = clampStr(turn.Objective, 240)
				}
				next.Goal = goal
				entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("劇本進入%s：目標「%s」，期限第 %d 回合，期限內達成每人獎勵 %d XP。", next.Stage, next.Goal, next.DeadlineRound, next.RewardXP)})
			} else {
				st.arc.Ended = true
				entries = append(entries, store.StoryRow{Speaker: "system", Text: "劇本三階段目標全部完成！故事進入尾聲。"})
			}
		}
	}

	// Scripted-module advancement: the server owns the node graph. The DM's
	// script.chosenOption signal (or a verbatim scripted-choice declaration)
	// picks the branch; entering the next node settles its treasure, scripted
	// combat and endings authoritatively.
	var scriptNode *ScriptNode
	scriptFirstEntry := false
	scriptWasRunning := scriptModuleFor(st.script) != nil && !st.script.Ended
	// Advancement is settled-outcomes only: never mid-combat (stale signals),
	// never on conclusion narration (players haven't chosen yet), and never on
	// a turn that opens a new required check (the branch outcome is pending).
	scriptMayAdvance := (st.combat == nil || !st.combat.Active) &&
		prepared.Input.Conclusion == nil &&
		!(turn.RequiresCheck && turn.Check != nil)
	if mod := scriptModuleFor(st.script); mod != nil && !st.script.Ended && scriptMayAdvance {
		// Actions in party order so two players clicking different scripted
		// choices resolve deterministically (front of the marching order wins).
		ordered := make([]string, 0, len(st.players))
		for _, p := range st.players {
			ordered = append(ordered, prepared.Actions[p.ID])
		}
		fromNode := mod.node(st.script.NodeID)
		choice := matchScriptChoice(fromNode, turn.Script.ChosenOption, ordered)
		var logs []string
		if scriptNode, logs = advanceScript(mod, st.script, choice); scriptNode != nil {
			// Crossing an act boundary is announced by popup, not journal.
			if fromNode != nil && fromNode.Stage != scriptNode.Stage {
				stageClear = &StageClear{Cleared: fromNode.Stage, Next: scriptNode.Stage, Title: scriptNode.Title}
			}
			// Visited already holds every node the party has left, so a node
			// present there is a revisit: loot and ambushes fire only once.
			scriptFirstEntry = !containsStr(st.script.Visited[:len(st.script.Visited)-1], scriptNode.ID)
			for _, l := range logs {
				entries = append(entries, store.StoryRow{Speaker: "system", Text: l})
			}
			if scriptFirstEntry {
				entries = append(entries, applyScriptTreasure(st, scriptNode.Treasure)...)
			}
			if scriptNode.Type == "ending" && st.arc != nil && !st.arc.Ended {
				st.arc.Ended = true
			}
		}
	}

	// Scripted combat fires the moment the party first enters a combat/boss
	// node, scaled to party size; it outranks any DM-declared encounter.
	if scriptNode != nil && scriptFirstEntry && scriptNode.Combat != nil && (scriptNode.Type == "combat" || scriptNode.Type == "boss") && (st.combat == nil || !st.combat.Active) {
		if err := s.startCombatLocked(st, scriptNode.Combat.scaledEnemies(len(st.players)), "initiative"); err == nil {
			entries = append(entries, store.StoryRow{Speaker: "system", Text: "戰鬥開始。先攻順序：" + initiativeOrder(*st.combat)})
		}
	}

	// Sync surviving combat combatants with the updated sheets.
	if st.combat != nil {
		for i := range st.combat.Combatants {
			c := &st.combat.Combatants[i]
			if c.PlayerID == "" {
				continue
			}
			for _, p := range st.players {
				if p.ID == c.PlayerID {
					c.HP = p.HP
					c.TemporaryHP = p.TemporaryHP
					c.Defeated = p.HP == 0
					break
				}
			}
		}
	}

	// DM-declared combat start. Running scripted campaigns keep encounters on
	// script (the node graph owns every fight); after the scripted ending the
	// DM may improvise again.
	if turn.Combat.Starts && !scriptWasRunning && (st.combat == nil || !st.combat.Active) && len(turn.Combat.Enemies) > 0 {
		enemies := make([]EnemySpec, 0, len(turn.Combat.Enemies))
		for _, e := range turn.Combat.Enemies {
			enemies = append(enemies, EnemySpec{
				Name: e.Name, AC: e.AC, HP: e.HP, InitiativeBonus: e.InitiativeBonus,
				AttackBonus: e.AttackBonus, Damage: e.Damage, DamageType: e.DamageType,
			})
		}
		if err := s.startCombatLocked(st, enemies, turn.Combat.FirstTurn); err == nil {
			entries = append(entries, store.StoryRow{Speaker: "system", Text: "戰鬥開始。先攻順序：" + initiativeOrder(*st.combat)})
		}
	}

	// Campaign meta: DM updates win when non-empty.
	setIf := func(dst *string, v string, cap int) {
		if strings.TrimSpace(v) != "" {
			*dst = clampStr(v, cap)
		}
	}
	setIf(&st.row.Scene, turn.Scene, 240)
	setIf(&st.row.Objective, turn.Objective, 240)
	setIf(&st.row.ObjectiveContext, turn.ObjectiveContext, 600)
	setIf(&st.row.Stakes, turn.Stakes, 300)
	setIf(&st.row.ImagePrompt, turn.ImagePrompt, 600)
	if !prepared.IsContinuation {
		st.row.Round++
	}
	st.pending = map[string]string{}

	choices := make([]rules.Choice, 0, len(turn.Choices))
	// Scripted campaigns pin the node's options to the front of the choice
	// list (out of combat), so the main line is always one click away; the
	// DM's own suggestions fill the remaining slots.
	if mod := scriptModuleFor(st.script); mod != nil && !st.script.Ended && (st.combat == nil || !st.combat.Active) {
		if node := mod.node(st.script.NodeID); node != nil {
			for _, c := range node.Choices {
				choices = append(choices, rules.Choice{Text: c.Text})
			}
		}
	}
	seen := make(map[string]bool, len(choices))
	for _, c := range choices {
		seen[strings.TrimSpace(c.Text)] = true
	}
	for _, c := range turn.Choices {
		if len(choices) >= 8 {
			break
		}
		if seen[strings.TrimSpace(c.Text)] {
			continue
		}
		seen[strings.TrimSpace(c.Text)] = true
		choices = append(choices, rules.Choice{Text: c.Text, PlayerID: c.PlayerID})
	}
	if data, err := jsonMarshal(choices); err == nil {
		st.row.Choices = data
	}

	st.check = nil
	if turn.RequiresCheck && turn.Check != nil {
		check := &rules.RequiredCheck{
			Character: turn.Check.Character,
			Ability:   turn.Check.Ability,
			Skill:     turn.Check.Skill,
			DC:        turn.Check.DC,
			Reason:    turn.Check.Reason,
			PlayerID:  turn.Check.PlayerID,
		}
		var actor *rules.Character
		if check.PlayerID != "" {
			for i := range st.players {
				if st.players[i].ID == check.PlayerID {
					actor = &st.players[i]
					break
				}
			}
		}
		if actor == nil {
			for i := range st.players {
				if strings.Contains(check.Character, st.players[i].Name) || strings.Contains(st.players[i].Name, check.Character) {
					actor = &st.players[i]
					check.PlayerID = st.players[i].ID
					break
				}
			}
		}
		if actor != nil {
			if m := rules.GetCheckBonus(*actor, check.Skill); m != 0 {
				check.Modifier = m
			} else {
				check.Modifier = rules.GetCheckBonus(*actor, check.Ability)
			}
		}
		check.DC = clampCheckDC(check.DC, check.Modifier)
		st.check = check
	}

	view, err := s.persistEntries(st, entries)
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{View: view, StageClear: stageClear}, nil
}

// applyScriptTreasure settles a scripted treasure node: gold splits evenly
// (remainder to the front of the marching order), items land on the lead
// character, and weapon-statted items become usable attacks — the same rules
// as DM-awarded loot.
func applyScriptTreasure(st *gameState, t *ScriptTreasure) []store.StoryRow {
	if t == nil || len(st.players) == 0 {
		return nil
	}
	var entries []store.StoryRow
	if t.Gold > 0 {
		share := t.Gold / len(st.players)
		extra := t.Gold % len(st.players)
		for i := range st.players {
			gain := share
			if i < extra {
				gain++
			}
			st.players[i].Gold += gain
		}
		entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("拾獲 %d 金幣，已平分給隊伍。", t.Gold)})
	}
	lead := &st.players[0]
	for _, item := range t.Items {
		owned := false
		for _, e := range lead.Equipment {
			if e == item.Name {
				owned = true
				break
			}
		}
		if !owned {
			lead.Equipment = append(lead.Equipment, item.Name)
			entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("%s獲得物品：%s。", lead.Name, item.Name)})
		}
		if lootDicePattern.MatchString(item.Damage) {
			hasAttack := false
			for _, a := range lead.Attacks {
				if a.Name == item.Name {
					hasAttack = true
					break
				}
			}
			if !hasAttack {
				damageType := item.DamageType
				if damageType == "" {
					damageType = "揮砍"
				}
				var props []string
				for _, p := range strings.FieldsFunc(item.Properties, func(r rune) bool { return r == '、' || r == ',' }) {
					if p = strings.TrimSpace(p); p != "" {
						props = append(props, p)
					}
				}
				lead.Attacks = append(lead.Attacks, rules.Attack{
					ID: "loot-" + item.Name, Name: item.Name,
					Damage: item.Damage, DamageType: damageType, Properties: props,
				})
				*lead = rules.Recalculate(*lead)
				entries = append(entries, store.StoryRow{Speaker: "system", Text: fmt.Sprintf("「%s」帶有武器數值（%s %s），已加入%s的攻擊選項。", item.Name, item.Damage, damageType, lead.Name)})
			}
		}
	}
	return entries
}
