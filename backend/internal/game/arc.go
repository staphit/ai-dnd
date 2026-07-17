package game

import "fmt"

// ArcPhase is one act of the campaign: a goal, a round deadline, and the XP
// bonus each character earns when the goal lands before the deadline.
type ArcPhase struct {
	Stage          string `json:"stage"` // 前期 | 中期 | 後期
	Goal           string `json:"goal"`
	DeadlineRound  int    `json:"deadlineRound"`
	RewardXP       int    `json:"rewardXp"`
	CompletedRound int    `json:"completedRound,omitempty"` // 0 = not yet
	RewardGranted  bool   `json:"rewardGranted,omitempty"`
}

// StoryArc paces a campaign into three acts so the story has a bounded shape
// instead of extending forever. The DM signals phase completion via the
// arc.phaseComplete output; the server owns deadlines and rewards.
type StoryArc struct {
	Phases  []ArcPhase `json:"phases"`
	Current int        `json:"current"`
	Ended   bool       `json:"ended,omitempty"`
}

// defaultStoryArc lays out 20-round acts starting from the round the arc is
// first created, so campaigns that predate the feature get a fair clock.
func defaultStoryArc(startRound int, objective string) *StoryArc {
	if startRound < 1 {
		startRound = 1
	}
	base := startRound - 1
	return &StoryArc{Phases: []ArcPhase{
		{Stage: "前期", Goal: objective, DeadlineRound: base + 20, RewardXP: 250},
		{Stage: "中期", DeadlineRound: base + 40, RewardXP: 400},
		{Stage: "後期", DeadlineRound: base + 60, RewardXP: 600},
	}}
}

// phase returns the active phase, or nil once the arc has ended.
func (a *StoryArc) phase() *ArcPhase {
	if a == nil || a.Ended || a.Current < 0 || a.Current >= len(a.Phases) {
		return nil
	}
	return &a.Phases[a.Current]
}

// arcPromptLines renders the pacing directives for the DM prompt.
func arcPromptLines(arc *StoryArc, round int) []string {
	if arc == nil {
		return nil
	}
	if arc.Ended {
		return []string{"劇本三階段目標皆已完成：請將故事收束，在少數回合內給出完整結局，不要再開啟新的支線。"}
	}
	p := arc.phase()
	if p == nil {
		return nil
	}
	goal := p.Goal
	if goal == "" {
		goal = "（由你依劇情訂出本階段目標，並寫入 objective）"
	}
	lines := []string{fmt.Sprintf(
		"劇本節奏：目前為%s（第 %d 回合，本階段期限第 %d 回合，期限內達成每人獎勵 %d XP）。階段目標：%s",
		p.Stage, round, p.DeadlineRound, p.RewardXP, goal,
	)}
	remaining := p.DeadlineRound - round
	switch {
	case remaining < 0:
		lines = append(lines, "已超過本階段期限：立即讓關鍵事件主動發生，把劇情直接推進到階段目標；限時獎勵已失效，但目標仍須完成。")
	case remaining <= 5:
		lines = append(lines, fmt.Sprintf("期限將至（剩 %d 回合）：讓階段目標與關鍵線索主動出現在玩家面前，加速收攏劇情，避免玩家漫遊。", remaining))
	}
	lines = append(lines, "當本階段目標實質達成時，將輸出的 arc.phaseComplete 設為 true，並在 arc.nextGoal 用一句話給出下一階段目標（最後階段完成則給空字串）。未達成時 phaseComplete 必須為 false。")
	return lines
}
