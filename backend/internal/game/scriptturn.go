package game

import (
	"fmt"
	"strings"

	"dndduet/internal/dm"
)

// scriptAdvanceXP is the per-player reward for reaching a new script node.
const scriptAdvanceXP = 25

// BuildScriptTurn resolves a DM request locally for scripted campaigns: the
// node graph already contains the branches and their prose, so no AI call is
// needed and the turn returns instantly. ok is false when the campaign is
// freeform (caller falls through to the AI DM).
func (s *Service) BuildScriptTurn(id string, prepared PreparedDMTurn) (*dm.Turn, bool, error) {
	unlock := s.Lock(id)
	st, err := s.loadState(id)
	unlock()
	if err != nil {
		return nil, false, err
	}
	mod := scriptModuleFor(st.script)
	if mod == nil {
		return nil, false, nil
	}
	node := mod.node(st.script.NodeID)
	if node == nil {
		return nil, false, nil
	}

	turn := &dm.Turn{
		Scene:  node.Title,
		Combat: dm.Combat{FirstTurn: "initiative", Enemies: []dm.Enemy{}},
	}

	switch {
	case st.script.Ended:
		turn.Narration = fmt.Sprintf("故事已在「%s」（%s）落幕。%s", node.Title, endingKindLabel(st.script.Ending), node.playerText())

	case prepared.Input.Resolution != nil:
		r := prepared.Input.Resolution
		if r.Success {
			turn.Narration = fmt.Sprintf("%s的%s（%s）檢定成功：骰出 %d，總值 %d 對 DC %d。%s——這一步站穩了，隊伍可以繼續行動。",
				r.Character, r.Ability, r.Skill, r.Natural, r.Total, r.DC, r.Reason)
		} else {
			turn.Narration = fmt.Sprintf("%s的%s（%s）檢定失敗：骰出 %d，總值 %d 未達 DC %d。%s——這條路走不通，得換個做法。",
				r.Character, r.Ability, r.Skill, r.Natural, r.Total, r.DC, r.Reason)
		}

	case prepared.Input.Conclusion != nil:
		outcome := "隊伍撤出了戰鬥"
		switch prepared.Input.Conclusion.Outcome {
		case "victory":
			outcome = "隊伍取得勝利"
		case "defeat":
			outcome = "隊伍不敵倒下"
		}
		turn.Narration = fmt.Sprintf("%s。塵埃落定，「%s」的局勢仍在眼前，接下來怎麼走由你們決定。", outcome, node.Title)

	case st.combat != nil && st.combat.Active:
		turn.Narration = fmt.Sprintf("「%s」的戰鬥仍在進行，請在戰鬥追蹤器中行動。", node.Title)

	default:
		ordered := make([]string, 0, len(st.players))
		for _, p := range st.players {
			ordered = append(ordered, prepared.Actions[p.ID])
		}
		choice := matchScriptChoice(node, "", ordered)
		if choice == nil {
			turn.Narration = fmt.Sprintf("隊伍在「%s」整備。%s", node.Title, node.playerText())
			break
		}
		next := mod.node(choice.Next)
		if next == nil {
			turn.Narration = node.playerText()
			break
		}
		turn.Script = dm.ScriptSignal{ChosenOption: choice.ID}
		turn.Scene = next.Title

		// Crossing an act boundary installs the new stage's mission summary
		// and completes the matching story-arc phase (timed reward included).
		if node.Stage != next.Stage {
			if so, ok := mod.StageObjectives[next.Stage]; ok {
				turn.Objective = so.Objective
				turn.ObjectiveContext = so.Context
				turn.Stakes = so.Stakes
			}
			nextGoal := ""
			if next.Stage != "結局" {
				nextGoal = mod.StageObjectives[next.Stage].Objective
			}
			turn.Arc = dm.ArcSignal{PhaseComplete: true, NextGoal: nextGoal}
		}

		// Lead the reply with the option just taken, so the choice reads as
		// part of the dialogue instead of sinking into the journal history.
		var parts []string
		parts = append(parts, "【選擇】"+choice.Text)
		parts = append(parts, next.playerText())
		firstEntry := !containsStr(st.script.Visited, next.ID)
		if firstEntry && next.Treasure != nil && strings.TrimSpace(next.Treasure.Intro) != "" {
			parts = append(parts, next.Treasure.Intro)
		}
		if firstEntry && next.Combat != nil && strings.TrimSpace(next.Combat.Intro) != "" {
			parts = append(parts, next.Combat.Intro)
		}
		turn.Narration = strings.Join(parts, "\n\n")

		if firstEntry {
			for _, p := range st.players {
				turn.ExperienceAwards = append(turn.ExperienceAwards, dm.ExperienceAward{
					PlayerID: p.ID, Amount: scriptAdvanceXP, Reason: "劇本推進：" + next.Title,
				})
			}
		}
	}

	return turn, true, nil
}
