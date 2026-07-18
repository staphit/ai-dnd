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
// freeform (caller falls through to the AI DM). Player-facing text follows
// prepared.Input.Language ("en" serves the module's English variants when
// authored; anything else, or a module without them, stays Chinese).
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
	lang := prepared.Input.Language
	en := lang == "en"

	turn := &dm.Turn{
		Scene:  node.title(lang),
		Combat: dm.Combat{FirstTurn: "initiative", Enemies: []dm.Enemy{}},
	}

	switch {
	case st.script.Ended:
		if en {
			turn.Narration = fmt.Sprintf("The story has already closed at 「%s」 (%s). %s", node.title(lang), endingKindLabelLang(st.script.Ending, lang), node.playerText(lang))
		} else {
			turn.Narration = fmt.Sprintf("故事已在「%s」（%s）落幕。%s", node.Title, endingKindLabel(st.script.Ending), node.playerText(lang))
		}

	case prepared.Input.Resolution != nil:
		r := prepared.Input.Resolution
		switch {
		case en && r.Success:
			turn.Narration = fmt.Sprintf("%s succeeds on the %s (%s) check: rolled %d for a total of %d against DC %d. %s — footing secured, the party can press on.",
				r.Character, r.Ability, r.Skill, r.Natural, r.Total, r.DC, r.Reason)
		case en:
			turn.Narration = fmt.Sprintf("%s fails the %s (%s) check: rolled %d for a total of %d, short of DC %d. %s — this way is barred; try another approach.",
				r.Character, r.Ability, r.Skill, r.Natural, r.Total, r.DC, r.Reason)
		case r.Success:
			turn.Narration = fmt.Sprintf("%s的%s（%s）檢定成功：骰出 %d，總值 %d 對 DC %d。%s——這一步站穩了，隊伍可以繼續行動。",
				r.Character, r.Ability, r.Skill, r.Natural, r.Total, r.DC, r.Reason)
		default:
			turn.Narration = fmt.Sprintf("%s的%s（%s）檢定失敗：骰出 %d，總值 %d 未達 DC %d。%s——這條路走不通，得換個做法。",
				r.Character, r.Ability, r.Skill, r.Natural, r.Total, r.DC, r.Reason)
		}

	case prepared.Input.Conclusion != nil:
		if en {
			outcome := "The party has withdrawn from the fight"
			switch prepared.Input.Conclusion.Outcome {
			case "victory":
				outcome = "The party is victorious"
			case "defeat":
				outcome = "The party has fallen"
			}
			turn.Narration = fmt.Sprintf("%s. As the dust settles, the situation at 「%s」 still lies before you — where to go next is yours to decide.", outcome, node.title(lang))
		} else {
			outcome := "隊伍撤出了戰鬥"
			switch prepared.Input.Conclusion.Outcome {
			case "victory":
				outcome = "隊伍取得勝利"
			case "defeat":
				outcome = "隊伍不敵倒下"
			}
			turn.Narration = fmt.Sprintf("%s。塵埃落定，「%s」的局勢仍在眼前，接下來怎麼走由你們決定。", outcome, node.Title)
		}

	case st.combat != nil && st.combat.Active:
		if en {
			turn.Narration = fmt.Sprintf("The battle at 「%s」 is still under way — act from the combat tracker.", node.title(lang))
		} else {
			turn.Narration = fmt.Sprintf("「%s」的戰鬥仍在進行，請在戰鬥追蹤器中行動。", node.Title)
		}

	default:
		ordered := make([]string, 0, len(st.players))
		for _, p := range st.players {
			ordered = append(ordered, prepared.Actions[p.ID])
		}
		choice := matchScriptChoice(node, "", ordered)
		if choice == nil {
			if en {
				turn.Narration = fmt.Sprintf("The party regroups at 「%s」. %s", node.title(lang), node.playerText(lang))
			} else {
				turn.Narration = fmt.Sprintf("隊伍在「%s」整備。%s", node.Title, node.playerText(lang))
			}
			break
		}
		next := mod.node(choice.Next)
		if next == nil {
			turn.Narration = node.playerText(lang)
			break
		}
		turn.Script = dm.ScriptSignal{ChosenOption: choice.ID}
		turn.Scene = next.title(lang)

		// Crossing an act boundary installs the new stage's mission summary
		// and completes the matching story-arc phase (timed reward included).
		if node.Stage != next.Stage {
			if so, ok := mod.StageObjectives[next.Stage]; ok {
				turn.Objective = pick(lang, so.Objective, so.ObjectiveEn)
				turn.ObjectiveContext = pick(lang, so.Context, so.ContextEn)
				turn.Stakes = pick(lang, so.Stakes, so.StakesEn)
			}
			nextGoal := ""
			if next.Stage != "結局" {
				so := mod.StageObjectives[next.Stage]
				nextGoal = pick(lang, so.Objective, so.ObjectiveEn)
			}
			turn.Arc = dm.ArcSignal{PhaseComplete: true, NextGoal: nextGoal}
		}

		// Lead the reply with the option just taken, so the choice reads as
		// part of the dialogue instead of sinking into the journal history.
		// The 【選擇】 marker is a frontend contract (story-choice chip).
		var parts []string
		parts = append(parts, "【選擇】"+choice.text(lang))
		parts = append(parts, next.playerText(lang))
		firstEntry := !containsStr(st.script.Visited, next.ID)
		if firstEntry && next.Treasure != nil {
			if intro := pick(lang, next.Treasure.Intro, next.Treasure.IntroEn); strings.TrimSpace(intro) != "" {
				parts = append(parts, intro)
			}
		}
		if firstEntry && next.Combat != nil {
			if intro := pick(lang, next.Combat.Intro, next.Combat.IntroEn); strings.TrimSpace(intro) != "" {
				parts = append(parts, intro)
			}
		}
		turn.Narration = strings.Join(parts, "\n\n")

		if firstEntry {
			reason := "劇本推進：" + next.Title
			if en {
				reason = "Script advance: " + next.title(lang)
			}
			for _, p := range st.players {
				turn.ExperienceAwards = append(turn.ExperienceAwards, dm.ExperienceAward{
					PlayerID: p.ID, Amount: scriptAdvanceXP, Reason: reason,
				})
			}
		}
	}

	return turn, true, nil
}
