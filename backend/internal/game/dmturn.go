package game

import (
	"fmt"
	"strings"

	"dndduet/internal/apperr"
	"dndduet/internal/dm"
	"dndduet/internal/rules"
	"dndduet/internal/store"
)

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
}

func (s *Service) prepareBase(st *gameState) dm.TurnInputV2 {
	choices := []rules.Choice{}
	if st.row.Choices != "" {
		_ = jsonUnmarshal(st.row.Choices, &choices)
	}
	return dm.TurnInputV2{
		Title:            st.row.Title,
		Scene:            st.row.Scene,
		Objective:        st.row.Objective,
		ObjectiveContext: st.row.ObjectiveContext,
		Stakes:           st.row.Stakes,
		Round:            st.row.Round,
		PrevChoices:      choiceTexts(choices),
		CombatLine:       combatLine(st.combat),
		Players:          sanitizedPlayers(st.players),
	}
}

// PrepareActionsTurn merges the submitted actions into the pending lock,
// requires a declaration from every player, and runs mechanical validation.
// The AI is never called for a mechanically illegal round.
func (s *Service) PrepareActionsTurn(id string, actions map[string]string, intents map[string]Intent) (PreparedDMTurn, error) {
	unlock := s.Lock(id)
	defer unlock()
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
	return PreparedDMTurn{Input: input, Players: input.Players, Actions: merged, Round: st.row.Round}, nil
}

// PrepareCheckTurn turns a player's natural d20 roll into the resolution
// continuation, computing modifier/total/success from the stored check and
// the authoritative sheet. The stored check is cleared only when the turn is
// applied, so a failed AI call keeps the dice tray up.
func (s *Service) PrepareCheckTurn(id string, natural int) (PreparedDMTurn, error) {
	unlock := s.Lock(id)
	defer unlock()
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
	total := natural + modifier

	input := s.prepareBase(st)
	input.Resolution = &dm.ResolutionV2{
		Character: st.check.Character,
		Ability:   st.check.Ability,
		Skill:     st.check.Skill,
		Reason:    st.check.Reason,
		DC:        st.check.DC,
		Natural:   natural,
		Modifier:  modifier,
		Total:     total,
		Success:   total >= st.check.DC,
	}
	return PreparedDMTurn{Input: input, Players: input.Players, Round: st.row.Round, IsContinuation: true}, nil
}

// PrepareConclusionTurn builds the post-combat narration turn from a
// conclusion the server computed in Conclude.
func (s *Service) PrepareConclusionTurn(id, outcome, summary string) (PreparedDMTurn, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return PreparedDMTurn{}, err
	}
	if outcome != "victory" && outcome != "defeat" && outcome != "withdrawal" {
		outcome = "withdrawal"
	}
	input := s.prepareBase(st)
	input.Conclusion = &dm.ConclusionV2{Outcome: outcome, Summary: clampStr(strings.TrimSpace(summary), 3000)}
	return PreparedDMTurn{Input: input, Players: input.Players, Round: st.row.Round, IsContinuation: true}, nil
}

// ApplyResult is the outcome of applying a DM turn.
type ApplyResult struct {
	View     View
	Rejected []ActionIssue // non-empty when the AI vetoed actions narratively
}

// ApplyDMTurn ports the App.tsx post-turn state update: narrative action
// rejection, effects, XP, combat sync/start, campaign meta, journal entries,
// pending/choices/requiredCheck. Runs under the campaign lock on fresh state.
func (s *Service) ApplyDMTurn(id string, prepared PreparedDMTurn, turn *dm.Turn) (ApplyResult, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return ApplyResult{}, err
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
		if err := s.AppendStory(id, []store.StoryRow{{Speaker: "dm", Text: "【行動駁回】" + strings.Join(details, "；") + "。故事尚未推進；請依照理由修改後重新鎖定。"}}); err != nil {
			return ApplyResult{}, err
		}
		view, err := s.persist(st, nil)
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

	// DM-declared combat start.
	if turn.Combat.Starts && (st.combat == nil || !st.combat.Active) && len(turn.Combat.Enemies) > 0 {
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
	for i, c := range turn.Choices {
		if i >= 8 {
			break
		}
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
		st.check = check
	}

	if err := s.AppendStory(id, entries); err != nil {
		return ApplyResult{}, err
	}
	view, err := s.persist(st, nil)
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{View: view}, nil
}
