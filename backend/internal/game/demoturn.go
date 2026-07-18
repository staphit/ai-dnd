package game

import (
	"fmt"
	"strings"

	"dndduet/internal/dm"
)

// BuildDemoTurn produces a deterministic, provider-free turn while keeping all
// state changes on the same server-authoritative ApplyDMTurn path as real AI
// and scripted turns. It is intentionally generic: demo mode proves the full
// lock/check/XP/persistence flow without pretending to be a second campaign
// engine in React. Text follows prepared.Input.Language ("en" narrates in
// English; check identifiers stay Chinese — they are mechanical identifiers).
func BuildDemoTurn(prepared PreparedDMTurn) *dm.Turn {
	in := prepared.Input
	lang := in.Language
	tr := func(zh, en string) string { return pick(lang, zh, en) }
	turn := &dm.Turn{
		Scene: in.Scene, Objective: in.Objective, ObjectiveContext: in.ObjectiveContext, Stakes: in.Stakes,
		ImagePrompt: "atmospheric candlelit fantasy tabletop scene, cinematic environmental storytelling",
		Choices: []dm.Choice{
			{Text: tr("仔細搜索眼前的新線索", "Search the new lead carefully")},
			{Text: tr("詢問附近的人並確認說法", "Question people nearby and cross-check their story")},
			{Text: tr("保持警戒，朝目前目標前進", "Stay alert and press on toward the current goal")},
		},
		Effects:          []dm.Effect{},
		PrivateMessages:  []dm.PrivateMessage{},
		ActionIssues:     []dm.ActionIssue{},
		ExperienceAwards: []dm.ExperienceAward{},
		Combat:           dm.Combat{FirstTurn: "initiative", Enemies: []dm.Enemy{}},
		Loot:             dm.Loot{Items: []dm.LootItem{}},
	}

	switch {
	case in.Resolution != nil:
		result := tr("失敗", "fails")
		consequence := tr("代價立即顯現，但也讓隊伍看清另一條可以前進的路。", "The cost shows itself at once — but so does another way forward.")
		if in.Resolution.Success {
			result = tr("成功", "succeeds")
			consequence = tr("阻礙被穩穩克服，原本隱藏的線索因此浮現。", "The obstacle is firmly overcome, and a hidden lead surfaces.")
		}
		turn.Narration = fmt.Sprintf(
			tr("%s的%s（%s）檢定%s。%s局勢已向前推進，不需要重複剛才的行動。", "%s's %s (%s) check %s. %s The situation has moved on; there is no need to repeat that action."),
			in.Resolution.Character, in.Resolution.Ability, in.Resolution.Skill, result, consequence,
		)
	case in.Conclusion != nil:
		outcome := map[string]string{
			"victory":    tr("隊伍取得勝利", "The party is victorious"),
			"defeat":     tr("隊伍遭到擊敗", "The party has been defeated"),
			"withdrawal": tr("雙方脫離交戰", "Both sides break off the fight"),
		}[in.Conclusion.Outcome]
		if outcome == "" {
			outcome = tr("戰鬥告一段落", "The fight comes to a pause")
		}
		if in.Conclusion.Final {
			turn.Narration = outcome + tr("。這場冒險在此留下最後一頁；眾人的選擇成為此地往後反覆傳述的故事。", ". The adventure writes its final page here; the choices made become the story this place will tell for years.")
			turn.Choices = []dm.Choice{}
		} else {
			turn.Narration = outcome + tr("。兵刃聲逐漸止息，倖存者開始處理傷勢與現場，而下一條線索也在混亂過後變得清晰。", ". The clash of arms fades, the survivors tend wounds and the scene — and past the chaos, the next lead comes clear.")
		}
	default:
		var actions []string
		for _, p := range prepared.Players {
			if text := strings.TrimSpace(prepared.Actions[p.ID]); text != "" {
				actions = append(actions, p.Name+"「"+clampStr(text, 120)+"」")
			}
			turn.ExperienceAwards = append(turn.ExperienceAwards, dm.ExperienceAward{
				PlayerID: p.ID, Amount: 75, Reason: tr("推進示範冒險並取得新線索", "Advancing the demo adventure and gaining a new lead"),
			})
		}
		turn.Narration = tr("隊伍的宣告依序落實：", "The party's declarations take effect in order: ") + strings.Join(actions, tr("；", "; ")) + tr("。環境對這些選擇產生了清楚回應，並露出足以繼續追查的新線索。", ". The world answers those choices clearly, and a new lead emerges worth pursuing.")
		// Every second declaration demonstrates the required-check continuation
		// without asking the model to invent or calculate mechanics.
		if in.Round%2 == 0 && len(prepared.Players) > 0 {
			actor := prepared.Players[0]
			turn.RequiresCheck = true
			turn.Check = &dm.Check{
				Character: actor.Name, PlayerID: actor.ID, Ability: "感知", Skill: "調查",
				DC: 12, Reason: tr("從混亂的現場辨認真正有用的線索", "Picking the truly useful clue out of a chaotic scene"),
			}
		}
	}

	if strings.TrimSpace(turn.Scene) == "" {
		turn.Scene = tr("未知場景", "Unknown scene")
	}
	if strings.TrimSpace(turn.Objective) == "" {
		turn.Objective = tr("追查眼前線索", "Follow the lead at hand")
	}
	if strings.TrimSpace(turn.ObjectiveContext) == "" {
		turn.ObjectiveContext = tr("隊伍正在確認下一步。", "The party is working out its next step.")
	}
	if strings.TrimSpace(turn.Stakes) == "" {
		turn.Stakes = tr("拖延會讓局勢變得更危險。", "Delay will make things more dangerous.")
	}
	return turn
}
