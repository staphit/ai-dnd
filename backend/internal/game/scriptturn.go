package game

import (
	"fmt"
	"strings"

	"dndduet/internal/dm"
	"dndduet/internal/rules"
)

// scriptAdvanceXP is the per-player reward for reaching a new script node.
const scriptAdvanceXP = 25

// scriptCheckDC scales the difficulty of authored checks with the act, so
// late-game gambles feel riskier than the opening scenes.
func scriptCheckDC(stage string) int {
	switch stage {
	case "中期":
		return 13
	case "後期", "結局":
		return 14
	default:
		return 12
	}
}

// checkHintSynonyms maps module vocabulary onto the rules engine's canonical
// skill names so authored hints roll with the right sheet bonus.
var checkHintSynonyms = map[string]string{
	"洞察": "洞悉",
	"生存": "求生",
	"意志": "感知",
	"隱蔽": "隱匿",
}

// parseCheckHint turns an authored choice checkHint ("調查:翻找刻文與名冊",
// "力量/運動:推移沉重石壇", "感知檢定") into a rollable ability/skill pair and
// the hint's reason text. ok is false when the hint names no known skill or
// ability (pure DM-adjudication notes like 「由DM裁定其份量」) — those choices
// advance without a roll.
func parseCheckHint(hint string) (ability, skill, reason string, ok bool) {
	head := strings.TrimSpace(hint)
	if head == "" {
		return "", "", "", false
	}
	for _, sep := range []string{"：", ":"} {
		if i := strings.Index(head, sep); i >= 0 {
			reason = strings.TrimSpace(head[i+len(sep):])
			head = strings.TrimSpace(head[:i])
			break
		}
	}
	head = strings.NewReplacer("、", "/", "或", "/").Replace(head)
	tokens := strings.Split(head, "/")
	for i, token := range tokens {
		token = strings.TrimSuffix(strings.TrimSpace(token), "檢定")
		if canonical, mapped := checkHintSynonyms[token]; mapped {
			token = canonical
		}
		tokens[i] = token
	}
	// Skills first: they carry proficiency, so 「力量/運動」 rolls 運動.
	for _, token := range tokens {
		if governing, isSkill := rules.SkillGoverningAbility(token); isSkill {
			return governing, token, reason, true
		}
	}
	for _, token := range tokens {
		if rules.IsAbilityLabel(token) {
			return token, token, reason, true
		}
	}
	return "", "", "", false
}

// scriptCheckActor picks the player whose declared action matched the choice;
// the front of the marching order when no declaration matches verbatim.
func scriptCheckActor(players []rules.Character, actions map[string]string, choice *ScriptChoice) *rules.Character {
	for i := range players {
		t := strings.TrimSpace(actions[players[i].ID])
		if t == "" {
			continue
		}
		if t == strings.TrimSpace(choice.Text) || (choice.TextEn != "" && t == strings.TrimSpace(choice.TextEn)) {
			return &players[i]
		}
	}
	return &players[0]
}

// applyScriptAdvance fills turn with a settled branch: the chosen-option
// signal, next-node prose, act-boundary objective swap and first-entry XP.
// prefix (a resolved-check sentence) leads the narration when non-empty.
func applyScriptAdvance(turn *dm.Turn, st *gameState, mod *ScriptModule, node *ScriptNode, choice *ScriptChoice, next *ScriptNode, lang, prefix string) {
	en := lang == "en"
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
	if strings.TrimSpace(prefix) != "" {
		parts = append(parts, prefix)
	}
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
		// A check opened by a scripted choice settles that branch: success
		// advances the script, failure keeps the party on the node to try
		// another way (or roll the same gamble again).
		var pendingChoice *ScriptChoice
		if st.check != nil && st.check.ScriptChoiceID != "" {
			for i := range node.Choices {
				if node.Choices[i].ID == st.check.ScriptChoiceID {
					pendingChoice = &node.Choices[i]
					break
				}
			}
		}
		switch {
		case r.Success && pendingChoice != nil:
			next := mod.node(pendingChoice.Next)
			if next == nil {
				turn.Narration = node.playerText(lang)
				break
			}
			var prefix string
			if en {
				prefix = fmt.Sprintf("%s succeeds on the %s (%s) check: rolled %d for a total of %d against DC %d. %s",
					r.Character, r.Ability, r.Skill, r.Natural, r.Total, r.DC, r.Reason)
			} else {
				prefix = fmt.Sprintf("%s的%s（%s）檢定成功：骰出 %d，總值 %d 對 DC %d。%s",
					r.Character, r.Ability, r.Skill, r.Natural, r.Total, r.DC, r.Reason)
			}
			applyScriptAdvance(turn, st, mod, node, pendingChoice, next, lang, prefix)
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

		// A choice with an authored check hint demands the dice first, but only
		// on its first traversal (a branch already taken is settled ground): the
		// turn opens a required check instead of advancing, and the resolution
		// continuation settles the branch.
		if ability, skill, hintReason, needsCheck := parseCheckHint(choice.CheckHint); needsCheck &&
			!containsStr(st.script.Taken, node.ID+":"+choice.ID) && len(st.players) > 0 {
			actor := scriptCheckActor(st.players, prepared.Actions, choice)
			reason := hintReason
			if reason == "" {
				reason = choice.text(lang)
			}
			dc := scriptCheckDC(node.Stage)
			turn.RequiresCheck = true
			turn.Check = &dm.Check{
				Character: actor.Name, PlayerID: actor.ID, Ability: ability, Skill: skill,
				DC: dc, Reason: reason, ScriptChoiceID: choice.ID,
			}
			if en {
				turn.Narration = fmt.Sprintf("%s attempts 「%s」 — that calls for a %s (%s) check against DC %d: %s. Roll the d20.",
					actor.Name, choice.text(lang), ability, skill, dc, reason)
			} else {
				turn.Narration = fmt.Sprintf("%s嘗試「%s」——這需要一次%s（%s）檢定，DC %d：%s。請擲骰。",
					actor.Name, choice.text(lang), ability, skill, dc, reason)
			}
			break
		}

		applyScriptAdvance(turn, st, mod, node, choice, next, lang, "")
	}

	return turn, true, nil
}
