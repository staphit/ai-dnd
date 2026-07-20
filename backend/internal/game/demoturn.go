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
// engine in React.
func BuildDemoTurn(prepared PreparedDMTurn) *dm.Turn {
	in := prepared.Input
	turn := &dm.Turn{
		Scene: in.Scene, Objective: in.Objective, ObjectiveContext: in.ObjectiveContext, Stakes: in.Stakes,
		ImagePrompt: "atmospheric candlelit fantasy tabletop scene, cinematic environmental storytelling",
		Choices: []dm.Choice{
			{Text: "仔細搜索眼前的新線索"},
			{Text: "詢問附近的人並確認說法"},
			{Text: "保持警戒，朝目前目標前進"},
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
		result := "失敗"
		consequence := "代價立即顯現，但也讓隊伍看清另一條可以前進的路。"
		if in.Resolution.Success {
			result = "成功"
			consequence = "阻礙被穩穩克服，原本隱藏的線索因此浮現。"
		}
		turn.Narration = fmt.Sprintf(
			"%s的%s（%s）檢定%s。%s局勢已向前推進，不需要重複剛才的行動。",
			in.Resolution.Character, in.Resolution.Ability, in.Resolution.Skill, result, consequence,
		)
	case in.Conclusion != nil:
		outcome := map[string]string{"victory": "隊伍取得勝利", "defeat": "隊伍遭到擊敗", "withdrawal": "雙方脫離交戰"}[in.Conclusion.Outcome]
		if outcome == "" {
			outcome = "戰鬥告一段落"
		}
		if in.Conclusion.Final {
			turn.Narration = outcome + "。這場冒險在此留下最後一頁；眾人的選擇成為此地往後反覆傳述的故事。"
			turn.Choices = []dm.Choice{}
		} else {
			turn.Narration = outcome + "。兵刃聲逐漸止息，倖存者開始處理傷勢與現場，而下一條線索也在混亂過後變得清晰。"
		}
	default:
		var actions []string
		for _, p := range prepared.Players {
			if text := strings.TrimSpace(prepared.Actions[p.ID]); text != "" {
				actions = append(actions, p.Name+"「"+clampStr(text, 120)+"」")
			}
			turn.ExperienceAwards = append(turn.ExperienceAwards, dm.ExperienceAward{
				PlayerID: p.ID, Amount: 75, Reason: "推進示範冒險並取得新線索",
			})
		}
		turn.Narration = "隊伍的宣告依序落實：" + strings.Join(actions, "；") + "。環境對這些選擇產生了清楚回應，並露出足以繼續追查的新線索。"
		// Every second declaration demonstrates the required-check continuation
		// without asking the model to invent or calculate mechanics.
		if in.Round%2 == 0 && len(prepared.Players) > 0 {
			actor := prepared.Players[0]
			turn.RequiresCheck = true
			turn.Check = &dm.Check{
				Character: actor.Name, PlayerID: actor.ID, Ability: "感知", Skill: "調查",
				DC: 12, Reason: "從混亂的現場辨認真正有用的線索",
			}
		}
	}

	if strings.TrimSpace(turn.Scene) == "" {
		turn.Scene = "未知場景"
	}
	if strings.TrimSpace(turn.Objective) == "" {
		turn.Objective = "追查眼前線索"
	}
	if strings.TrimSpace(turn.ObjectiveContext) == "" {
		turn.ObjectiveContext = "隊伍正在確認下一步。"
	}
	if strings.TrimSpace(turn.Stakes) == "" {
		turn.Stakes = "拖延會讓局勢變得更危險。"
	}
	return turn
}
