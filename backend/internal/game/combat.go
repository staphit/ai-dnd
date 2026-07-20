package game

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"dndduet/internal/apperr"
	"dndduet/internal/rules"
)

// EnemySpec describes one enemy for combat start (manual builder or the DM's
// structured combat.enemies block).
type EnemySpec struct {
	Name            string `json:"name"`
	AC              int    `json:"ac"`
	HP              int    `json:"hp"`
	InitiativeBonus int    `json:"initiativeBonus"`
	AttackBonus     int    `json:"attackBonus"`
	Damage          string `json:"damage"`
	DamageType      string `json:"damageType"`
}

func (s *Service) enemyCombatants(specs []EnemySpec) []rules.Combatant {
	out := make([]rules.Combatant, 0, len(specs))
	for i, e := range specs {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			name = "未命名敵人"
		}
		hp := e.HP
		if hp < 1 {
			hp = 1
		}
		damage := strings.TrimSpace(e.Damage)
		if damage == "" {
			damage = "1d6"
		}
		out = append(out, rules.Combatant{
			ID:              fmt.Sprintf("enemy-%d-%s", i+1, randomID()[:8]),
			Name:            clampStr(name, 50),
			Side:            "enemy",
			InitiativeBonus: e.InitiativeBonus,
			AC:              e.AC,
			HP:              hp,
			MaxHP:           hp,
			AttackBonus:     e.AttackBonus,
			Damage:          clampStr(damage, 30),
			DamageType:      clampStr(e.DamageType, 20),
		})
	}
	return out
}

// combatSnapshot is the party + combat state captured the moment combat
// starts, so a wiped party can retry the encounter from its opening state.
type combatSnapshot struct {
	Players []rules.Character `json:"players"`
	Combat  rules.CombatState `json:"combat"`
}

// startCombatLocked builds and persists a fresh combat state; the campaign
// lock must already be held.
func (s *Service) startCombatLocked(st *gameState, enemies []EnemySpec, firstTurn string) error {
	if st.combat != nil && st.combat.Active {
		return apperr.New(400, "戰鬥已在進行中。")
	}
	if len(enemies) == 0 {
		return apperr.New(400, "戰鬥需要至少一名敵人。")
	}
	if len(enemies) > 8 {
		enemies = enemies[:8]
	}
	combat := rules.StartCombat(append(rules.PartyCombatants(st.players), s.enemyCombatants(enemies)...), s.dice, firstTurn)
	st.combat = &combat
	if data, err := json.Marshal(combatSnapshot{Players: st.players, Combat: combat}); err == nil {
		// Best-effort: a missing snapshot only disables 戰鬥重來 for this fight.
		_ = s.store.SaveCombatSnapshot(st.row.ID, string(data), s.now().UnixMilli())
	}
	return nil
}

// RetryCombat restores the party and combat to the snapshot captured when the
// current combat started (same enemies, HP, resources, and initiative order).
func (s *Service) RetryCombat(id string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	if st.combat == nil || !st.combat.Active {
		return View{}, apperr.New(400, "目前沒有進行中的戰鬥。")
	}
	data, ok, err := s.store.CombatSnapshot(id)
	if err != nil {
		return View{}, err
	}
	if !ok {
		return View{}, apperr.New(400, "找不到本場戰鬥的開場紀錄，無法重來。")
	}
	var snap combatSnapshot
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		return View{}, fmt.Errorf("combat snapshot for %s is corrupt: %w", id, err)
	}
	st.players = snap.Players
	*st.combat = snap.Combat
	return s.persist(st, []string{fmt.Sprintf("戰鬥重來：隊伍與敵人回到本場戰鬥開始時的狀態。先攻順序：%s", initiativeOrder(*st.combat))})
}

func initiativeOrder(state rules.CombatState) string {
	parts := make([]string, 0, len(state.Combatants))
	for _, c := range state.Combatants {
		parts = append(parts, fmt.Sprintf("%s %d", c.Name, c.Initiative))
	}
	return strings.Join(parts, " → ")
}

// StartCombatManual is the encounter-builder path: the party plus hand-built
// enemies roll initiative server-side.
func (s *Service) StartCombatManual(id string, enemies []EnemySpec) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	if err := s.startCombatLocked(st, enemies, "initiative"); err != nil {
		return View{}, err
	}
	return s.persist(st, []string{fmt.Sprintf("戰鬥開始。先攻順序：%s", initiativeOrder(*st.combat))})
}

// applyCombatChange ports App.tsx changeCombat: sync combatant HP back onto
// the party and award victory XP the moment the last enemy drops.
func (s *Service) applyCombatChange(st *gameState, next rules.CombatState) []string {
	var logs []string
	previousAlive := false
	if st.combat != nil {
		for _, c := range st.combat.Combatants {
			if c.Side == "enemy" && !c.Defeated {
				previousAlive = true
				break
			}
		}
	}
	enemies := 0
	allDefeated := true
	totalEnemyHP := 0
	for _, c := range next.Combatants {
		if c.Side == "enemy" {
			enemies++
			totalEnemyHP += c.MaxHP
			if !c.Defeated {
				allDefeated = false
			}
		}
	}
	st.players = rules.SyncPlayersFromCombat(st.players, next)
	*st.combat = next
	if enemies > 0 && allDefeated && previousAlive {
		reward := int(math.Max(50, math.Ceil(float64(totalEnemyHP*10)/math.Max(1, float64(len(st.players))))))
		for i := range st.players {
			st.players[i] = rules.GrantExperience(st.players[i], reward)
		}
		logs = append(logs, fmt.Sprintf("戰鬥勝利：每位角色獲得 %d XP。", reward))
	}
	return logs
}

// AttackParams selects the attack and target for the current combatant.
type AttackParams struct {
	AttackID string `json:"attackId"`
	TargetID string `json:"targetId"`
}

// AttackResult is the view plus the dice outcome for the UI.
type AttackResult struct {
	View       View                   `json:"view"`
	Resolution rules.AttackResolution `json:"resolution"`
}

// Attack resolves the current combatant's weapon attack with server dice,
// spends the action, syncs players, and handles victory XP.
func (s *Service) Attack(id string, p AttackParams) (AttackResult, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return AttackResult{}, err
	}
	if st.combat == nil || !st.combat.Active {
		return AttackResult{}, apperr.New(400, "目前沒有進行中的戰鬥。")
	}
	if st.combat.TurnIndex < 0 || st.combat.TurnIndex >= len(st.combat.Combatants) {
		return AttackResult{}, apperr.New(400, "戰鬥狀態異常：找不到當前行動者。")
	}
	current := st.combat.Combatants[st.combat.TurnIndex]

	targetID := strings.TrimSpace(p.TargetID)
	if targetID == "" {
		for _, c := range st.combat.Combatants {
			if !c.Defeated && c.ID != current.ID && c.Side != current.Side {
				targetID = c.ID
				break
			}
		}
	}
	if targetID == "" {
		return AttackResult{}, apperr.New(400, "沒有可攻擊的目標。")
	}

	// A player combatant may pick one of their sheet attacks; the tracker
	// projects its numbers onto the combatant before rolling (CombatTracker.tsx).
	state := *st.combat
	strikes := 1
	if current.PlayerID != "" {
		for _, player := range st.players {
			if player.ID != current.PlayerID {
				continue
			}
			var chosen *rules.Attack
			for i := range player.Attacks {
				if player.Attacks[i].ID == p.AttackID {
					chosen = &player.Attacks[i]
					break
				}
			}
			if chosen == nil && len(player.Attacks) > 0 {
				chosen = &player.Attacks[0]
			}
			if chosen != nil {
				combatants := make([]rules.Combatant, len(state.Combatants))
				copy(combatants, state.Combatants)
				for i := range combatants {
					if combatants[i].ID == current.ID {
						combatants[i].AttackBonus = chosen.AttackBonus
						combatants[i].Damage = chosen.Damage
						combatants[i].DamageType = chosen.DamageType
					}
				}
				state.Combatants = combatants
				// Light weapons strike twice per action (house rule).
				strikes = chosen.AttacksPerAction
				if strikes < 1 {
					strikes = rules.WeaponAttacksPerAction(*chosen)
				}
			}
			break
		}
	}

	// One action may carry several strikes; stop early once the target drops.
	resolved := state
	var resolution rules.AttackResolution
	var strikeTexts []string
	for strike := 0; strike < strikes; strike++ {
		next, res, err := rules.ResolveAttack(resolved, current.ID, targetID, s.dice, "normal")
		if err != nil {
			return AttackResult{}, apperr.New(400, err.Error())
		}
		resolved = next
		resolution = res
		text := res.Text
		if strikes > 1 {
			text = fmt.Sprintf("第 %d 擊：%s", strike+1, text)
		}
		strikeTexts = append(strikeTexts, text)
		targetDown := false
		for _, c := range resolved.Combatants {
			if c.ID == targetID && c.Defeated {
				targetDown = true
				break
			}
		}
		if targetDown {
			break
		}
	}
	spent, err := rules.SpendCombatResource(resolved, current.ID, "action")
	if err != nil {
		return AttackResult{}, apperr.New(400, err.Error())
	}
	logs := append([]string{fmt.Sprintf("%s（已使用動作）", strings.Join(strikeTexts, " "))}, s.applyCombatChange(st, spent)...)
	view, err := s.persist(st, logs)
	if err != nil {
		return AttackResult{}, err
	}
	return AttackResult{View: view, Resolution: resolution}, nil
}

// EndTurnResult flags when the next actor is an enemy so the UI can trigger
// the enemy-turn endpoint.
type EndTurnResult struct {
	View             View `json:"view"`
	EnemyTurnPending bool `json:"enemyTurnPending"`
}

// EndTurn advances to the next non-defeated combatant.
func (s *Service) EndTurn(id string) (EndTurnResult, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return EndTurnResult{}, err
	}
	if st.combat == nil || !st.combat.Active {
		return EndTurnResult{}, apperr.New(400, "目前沒有進行中的戰鬥。")
	}
	next := rules.AdvanceTurn(*st.combat)
	logs := s.applyCombatChange(st, next)
	view, err := s.persist(st, logs)
	if err != nil {
		return EndTurnResult{}, err
	}
	return EndTurnResult{View: view, EnemyTurnPending: s.enemyTurnPending(st)}, nil
}

// enemyTurnPending reports whether combat is live and the current actor is an
// undefeated enemy.
func (s *Service) enemyTurnPending(st *gameState) bool {
	if st.combat == nil || !st.combat.Active {
		return false
	}
	if st.combat.TurnIndex < 0 || st.combat.TurnIndex >= len(st.combat.Combatants) {
		return false
	}
	current := st.combat.Combatants[st.combat.TurnIndex]
	return current.Side == "enemy" && !current.Defeated
}

// Conclusion mirrors the App.tsx endCombat outcome computation.
type Conclusion struct {
	Outcome string `json:"outcome"` // victory | defeat | withdrawal
	Summary string `json:"summary"`
}

// ConcludeResult carries the post-combat view plus the conclusion payload the
// DM turn narrates from.
type ConcludeResult struct {
	View       View       `json:"view"`
	Conclusion Conclusion `json:"conclusion"`
}

// Conclude deactivates combat, computes outcome and summary server-side, and
// logs the result. The DM narration turn consumes the returned conclusion.
func (s *Service) Conclude(id string) (ConcludeResult, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return ConcludeResult{}, err
	}
	if st.combat == nil || !st.combat.Active {
		return ConcludeResult{}, apperr.New(400, "目前沒有進行中的戰鬥。")
	}

	enemies, party := 0, 0
	enemiesDefeated, partyDefeated := true, true
	summaryParts := make([]string, 0, len(st.combat.Combatants))
	for _, c := range st.combat.Combatants {
		suffix := ""
		if c.Defeated {
			suffix = "（失去戰鬥能力）"
		}
		summaryParts = append(summaryParts, fmt.Sprintf("%s：%d/%d HP%s", c.Name, c.HP, c.MaxHP, suffix))
		switch c.Side {
		case "enemy":
			enemies++
			if !c.Defeated {
				enemiesDefeated = false
			}
		case "party":
			party++
			if !c.Defeated {
				partyDefeated = false
			}
		}
	}
	outcome := "withdrawal"
	switch {
	case enemies > 0 && enemiesDefeated:
		outcome = "victory"
	case party > 0 && partyDefeated:
		outcome = "defeat"
	}
	summary := strings.Join(summaryParts, "；")

	st.players = rules.SyncPlayersFromCombat(st.players, *st.combat)
	st.combat.Active = false
	_ = s.store.DeleteCombatSnapshot(id) // combat over; retry window closed

	resultLabel := map[string]string{"victory": "隊伍勝利", "defeat": "隊伍戰敗", "withdrawal": "戰鬥中止或撤退"}[outcome]
	view, err := s.persist(st, []string{fmt.Sprintf("戰鬥結束：%s。%s", resultLabel, summary)})
	if err != nil {
		return ConcludeResult{}, err
	}
	return ConcludeResult{View: view, Conclusion: Conclusion{Outcome: outcome, Summary: summary}}, nil
}
