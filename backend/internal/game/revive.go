package game

import (
	"fmt"

	"dndduet/internal/apperr"
	"dndduet/internal/rules"
)

// Revive lets a standing ally bring a downed (0 HP) character back up by
// spending resources: the downed character's hit dice fuel the healing, and
// the rescue costs the rescuer their combat action (in combat) or one point
// of exploration time (out of combat).
func (s *Service) Revive(id, rescuerID, targetID string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	if rescuerID == targetID {
		return View{}, apperr.New(400, "不能對自己進行救援。")
	}
	rescuer, err := st.player(rescuerID)
	if err != nil {
		return View{}, err
	}
	target, err := st.player(targetID)
	if err != nil {
		return View{}, err
	}
	if rescuer.HP <= 0 {
		return View{}, apperr.New(400, rescuer.Name+"自身已倒地，無法救援隊友。")
	}
	if target.HP > 0 {
		return View{}, apperr.New(400, target.Name+"尚未倒地，不需要救援。")
	}
	if st.check != nil {
		return View{}, apperr.New(400, "請先完成目前的必要擲骰，再進行救援。")
	}

	combatActive := st.combat != nil && st.combat.Active

	// Cost: combat action on the rescuer's turn, or 1 exploration action point.
	costLabel := ""
	if combatActive {
		next, err := rules.SpendCombatResource(*st.combat, rescuerID, "action")
		if err != nil {
			return View{}, apperr.New(400, err.Error())
		}
		*st.combat = next
		costLabel = "消耗本回合動作"
	} else {
		if st.pending[rescuerID] != "" {
			return View{}, apperr.New(400, rescuer.Name+"本回合已鎖定行動，請先解鎖才能改為救援。")
		}
		st.row.Round++
		costLabel = "消耗 1 點探索行動時間"
	}

	// Healing: the downed character spends hit dice (1 in combat, up to 2 out
	// of combat), each healing die + CON modifier; no dice left still revives
	// at 1 HP — the rescue itself is what brings them back.
	maxDice := 1
	if !combatActive {
		maxDice = 2
	}
	conMod := rules.AbilityModifier(target.Abilities.Con)
	healed := 0
	diceSpent := 0
	for diceSpent < maxDice && target.HitDice > 0 {
		roll := rules.Die(target.HitDie, s.dice) + conMod
		if roll < 1 {
			roll = 1
		}
		healed += roll
		target.HitDice--
		diceSpent++
	}
	if healed < 1 {
		healed = 1
	}
	if healed > target.MaxHP {
		healed = target.MaxHP
	}
	target.HP = healed
	target.Condition = "正常"

	// Combat sync: the revived combatant stands back up.
	if combatActive {
		for i := range st.combat.Combatants {
			c := &st.combat.Combatants[i]
			if c.PlayerID == targetID {
				c.HP = target.HP
				c.Defeated = false
				break
			}
		}
	}

	diceNote := ""
	if diceSpent > 0 {
		diceNote = fmt.Sprintf("，花費 %d 顆 d%d 生命骰", diceSpent, target.HitDie)
	} else {
		diceNote = "（生命骰已用盡，僅以 1 HP 甦醒）"
	}
	log := fmt.Sprintf("%s救援倒地的%s（%s）%s，%s以 %d HP 重新站起。", rescuer.Name, target.Name, costLabel, diceNote, target.Name, target.HP)
	return s.persist(st, []string{log})
}
