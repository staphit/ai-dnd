package game

import (
	"context"
	"fmt"
	"log"
	"strings"

	"dndduet/internal/apperr"
	"dndduet/internal/dm"
	"dndduet/internal/rules"
)

// TacticsRunner asks the AI for the current enemy's tactic. Wired from the
// HTTP layer (provider + schema path); nil disables AI tactics entirely.
type TacticsRunner func(ctx context.Context, input dm.TacticsInput) (dm.Tactic, error)

// EnemyTurnResult is the outcome of one AI-driven enemy turn.
type EnemyTurnResult struct {
	View             View                   `json:"view"`
	Resolution       rules.AttackResolution `json:"resolution"`
	Intent           string                 `json:"intent"`
	Fallback         bool                   `json:"fallback"`
	EnemyTurnPending bool                   `json:"enemyTurnPending"`
}

// EnemyTurn runs the current enemy combatant's turn: the AI picks target and
// intent (falling back to lowest-HP targeting on any AI failure so combat
// never blocks), the server rolls the attack, spends the action, logs
// 【敵方】intent — result, and advances to the next combatant.
func (s *Service) EnemyTurn(ctx context.Context, id string, run TacticsRunner) (EnemyTurnResult, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return EnemyTurnResult{}, err
	}
	if st.combat == nil || !st.combat.Active {
		return EnemyTurnResult{}, apperr.New(400, "目前沒有進行中的戰鬥。")
	}
	if !s.enemyTurnPending(st) {
		return EnemyTurnResult{}, apperr.New(400, "現在不是敵方回合。")
	}
	current := st.combat.Combatants[st.combat.TurnIndex]

	// Living opposing targets, keyed for the AI by combatant id.
	var targets []dm.TacticsTarget
	condition := func(playerID string) string {
		for _, p := range st.players {
			if p.ID == playerID {
				return p.Condition
			}
		}
		return ""
	}
	for _, c := range st.combat.Combatants {
		if c.Side != "party" || c.Defeated {
			continue
		}
		targets = append(targets, dm.TacticsTarget{
			ID: c.ID, Name: c.Name, HP: c.HP, MaxHP: c.MaxHP, AC: c.AC, Condition: condition(c.PlayerID),
		})
	}
	if len(targets) == 0 {
		return EnemyTurnResult{}, apperr.New(400, "沒有可攻擊的目標。")
	}

	recent := []string{}
	if tail, err := s.store.StoryTail(id, 2); err == nil {
		for _, e := range tail {
			recent = append(recent, clampStr(e.Text, 160))
		}
	}

	tactic := dm.Tactic{}
	fallback := true
	if run != nil {
		input := dm.TacticsInput{
			EnemyName:        current.Name,
			EnemyHP:          current.HP,
			EnemyMaxHP:       current.MaxHP,
			EnemyAttackBonus: current.AttackBonus,
			EnemyDamage:      current.Damage,
			EnemyDamageType:  current.DamageType,
			Round:            st.combat.Round,
			Targets:          targets,
			RecentLog:        recent,
		}
		if got, err := run(ctx, input); err == nil {
			// Accept only a target from the candidate list.
			for _, t := range targets {
				if got.TargetID == t.ID {
					tactic = got
					fallback = false
					break
				}
			}
			if fallback {
				log.Printf("[tactics] invalid targetId %q from AI, falling back", got.TargetID)
			}
		} else {
			log.Printf("[tactics] AI tactic failed, falling back: %v", err)
		}
	}
	if fallback {
		lowest := targets[0]
		for _, t := range targets[1:] {
			if t.HP < lowest.HP {
				lowest = t
			}
		}
		tactic = dm.Tactic{
			TargetID: lowest.ID,
			Attack:   strings.TrimSpace(current.Damage),
			Intent:   fmt.Sprintf("%s撲向最虛弱的%s。", current.Name, lowest.Name),
		}
	}

	resolved, resolution, err := rules.ResolveAttack(*st.combat, current.ID, tactic.TargetID, s.dice, "normal")
	if err != nil {
		return EnemyTurnResult{}, apperr.New(400, err.Error())
	}
	spent, err := rules.SpendCombatResource(resolved, current.ID, "action")
	if err != nil {
		return EnemyTurnResult{}, apperr.New(400, err.Error())
	}
	logs := s.applyCombatChange(st, spent)
	next := rules.AdvanceTurn(*st.combat)
	logs = append([]string{fmt.Sprintf("【敵方】%s — %s", clampStr(strings.TrimSpace(tactic.Intent), 160), resolution.Text)}, append(logs, s.applyCombatChange(st, next)...)...)

	view, err := s.persist(st, logs)
	if err != nil {
		return EnemyTurnResult{}, err
	}
	return EnemyTurnResult{
		View:             view,
		Resolution:       resolution,
		Intent:           tactic.Intent,
		Fallback:         fallback,
		EnemyTurnPending: s.enemyTurnPending(st),
	}, nil
}
