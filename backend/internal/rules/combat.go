package rules

// Ported 1:1 from frontend/src/rules/combat.ts: initiative and turn order,
// per-turn action economy, attack resolution (advantage, criticals, temporary
// HP absorption), and syncing combat results back onto player characters.
// The die/rollExpression helpers from combat.ts live in dice.go.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// AttackResolution mirrors combat.ts AttackResolution.
type AttackResolution struct {
	AttackRoll int    `json:"attackRoll"`
	Total      int    `json:"total"`
	Hit        bool   `json:"hit"`
	Critical   bool   `json:"critical"`
	Damage     int    `json:"damage"`
	Text       string `json:"text"`
}

// PartyCombatants ports combat.ts partyCombatants: it projects each player
// character onto a party-side Combatant using their first attack (falling
// back to 1d4 鈍擊 with +0 to hit when no attack exists).
func PartyCombatants(players []Character) []Combatant {
	combatants := make([]Combatant, 0, len(players))
	for _, player := range players {
		var attack Attack // zero value plays the role of TS `attack?.` on an empty list
		if len(player.Attacks) > 0 {
			attack = player.Attacks[0]
		}
		damage := attack.Damage
		if damage == "" {
			damage = "1d4"
		}
		damageType := attack.DamageType
		if damageType == "" {
			damageType = "鈍擊"
		}
		combatants = append(combatants, Combatant{
			ID:              player.ID,
			PlayerID:        player.ID,
			Name:            player.Name,
			Side:            "party",
			InitiativeBonus: player.Initiative,
			Initiative:      0,
			AC:              player.AC,
			HP:              player.HP,
			TemporaryHP:     player.TemporaryHP,
			MaxHP:           player.MaxHP,
			AttackBonus:     attack.AttackBonus,
			Damage:          damage,
			DamageType:      damageType,
			CritThreshold:   player.CritThreshold,
			SneakAttackDice: player.SneakAttackDice,
			Defeated:        player.HP <= 0,
		})
	}
	return combatants
}

// StartCombat ports combat.ts startCombat. Every combatant rolls
// d20 + initiativeBonus, the order is sorted by initiative descending (ties:
// initiativeBonus descending, then name), and a fresh turn economy record is
// created for each combatant. firstTurn is "initiative" or "enemy"; "enemy"
// hands the opening turn to the first non-defeated enemy in initiative order
// (an ambush), falling back to index 0 when no such enemy exists.
func StartCombat(combatants []Combatant, random RandomSource, firstTurn string) CombatState {
	rolled := make([]Combatant, len(combatants))
	for i, combatant := range combatants {
		combatant.Initiative = Die(20, random) + combatant.InitiativeBonus
		combatant.Defeated = combatant.HP <= 0
		rolled[i] = combatant
	}
	sort.SliceStable(rolled, func(i, j int) bool {
		a, b := rolled[i], rolled[j]
		if a.Initiative != b.Initiative {
			return a.Initiative > b.Initiative
		}
		if a.InitiativeBonus != b.InitiativeBonus {
			return a.InitiativeBonus > b.InitiativeBonus
		}
		// TS breaks full ties with name.localeCompare(name, 'zh-TW');
		// strings.Compare uses UTF-8 byte order instead. The divergence is
		// cosmetic: it only reorders combatants whose initiative AND
		// initiative bonus are both tied.
		return strings.Compare(a.Name, b.Name) < 0
	})
	enemyIndex := -1
	if firstTurn == "enemy" {
		for i, entry := range rolled {
			if entry.Side == "enemy" && !entry.Defeated {
				enemyIndex = i
				break
			}
		}
	}
	turnIndex := 0
	if enemyIndex >= 0 {
		turnIndex = enemyIndex
	}
	economy := make(map[string]TurnEconomy, len(rolled))
	for _, entry := range rolled {
		economy[entry.ID] = TurnEconomy{}
	}
	return CombatState{Active: true, Round: 1, TurnIndex: turnIndex, Combatants: rolled, TurnEconomy: economy}
}

// CombatResourceForCastingTime ports combat.ts combatResourceForCastingTime:
// it maps a spell's casting-time text onto the turn-economy resource it
// consumes — "bonusAction" (附贈動作), "reaction" (反應), or "action".
func CombatResourceForCastingTime(castingTime string) string {
	if strings.Contains(castingTime, "附贈動作") {
		return "bonusAction"
	}
	if strings.Contains(castingTime, "反應") {
		return "reaction"
	}
	return "action"
}

// SpendCombatResource ports combat.ts spendCombatResource. combatantID may be
// either the combatant id or the owning player's id; resource is "action",
// "bonusAction", or "reaction". Actions and bonus actions may only be spent
// on the actor's own turn; reactions may be spent any time. Errors carry the
// exact TS throw messages.
func SpendCombatResource(state CombatState, combatantID, resource string) (CombatState, error) {
	actorIndex := -1
	for i, entry := range state.Combatants {
		if entry.ID == combatantID || entry.PlayerID == combatantID {
			actorIndex = i
			break
		}
	}
	if actorIndex < 0 {
		return CombatState{}, errors.New("找不到要消耗行動次數的戰鬥角色。")
	}
	actor := state.Combatants[actorIndex]
	// TS reads state.combatants[state.turnIndex] with optional chaining: an
	// out-of-range turnIndex behaves as "no current actor" (名稱 falls back
	// to 其他角色).
	currentID, currentName := "", ""
	hasCurrent := state.TurnIndex >= 0 && state.TurnIndex < len(state.Combatants)
	if hasCurrent {
		currentID = state.Combatants[state.TurnIndex].ID
		currentName = state.Combatants[state.TurnIndex].Name
	}
	if resource != "reaction" && (!hasCurrent || currentID != actor.ID) {
		name := currentName
		if name == "" {
			name = "其他角色"
		}
		return CombatState{}, fmt.Errorf("現在是 %s 的回合。", name)
	}
	// A missing map entry yields the zero TurnEconomy, matching the TS
	// `economy[actor.id] || { actionUsed: false, ... }` fallback.
	usage := state.TurnEconomy[actor.ID]
	var used bool
	var label string
	switch resource {
	case "action":
		used = usage.ActionUsed
		label = "動作"
	case "bonusAction":
		used = usage.BonusActionUsed
		label = "附贈動作"
	default:
		used = usage.ReactionUsed
		label = "反應"
	}
	if used {
		return CombatState{}, fmt.Errorf("%s本輪的%s已使用。", actor.Name, label)
	}
	switch resource {
	case "action":
		usage.ActionUsed = true
	case "bonusAction":
		usage.BonusActionUsed = true
	default:
		usage.ReactionUsed = true
	}
	economy := make(map[string]TurnEconomy, len(state.TurnEconomy)+1)
	for id, entry := range state.TurnEconomy {
		economy[id] = entry
	}
	economy[actor.ID] = usage
	next := state
	next.TurnEconomy = economy
	return next, nil
}

// AdvanceTurn ports combat.ts advanceTurn: it moves to the next non-defeated
// combatant, increments the round whenever the order wraps (next index <=
// current index), and resets the turn economy of ONLY the incoming actor.
func AdvanceTurn(state CombatState) CombatState {
	if !state.Active || len(state.Combatants) == 0 {
		return state
	}
	next := state.TurnIndex
	attempts := 0
	for {
		next = (next + 1) % len(state.Combatants)
		attempts++
		if !state.Combatants[next].Defeated || attempts >= len(state.Combatants) {
			break
		}
	}
	wrapped := next <= state.TurnIndex
	nextActor := state.Combatants[next]
	economy := make(map[string]TurnEconomy, len(state.TurnEconomy)+1)
	for id, entry := range state.TurnEconomy {
		economy[id] = entry
	}
	economy[nextActor.ID] = TurnEconomy{}
	out := state
	out.TurnIndex = next
	if wrapped {
		out.Round = state.Round + 1
	}
	out.TurnEconomy = economy
	return out
}

// ResolveAttack ports combat.ts resolveAttack. advantage is "normal",
// "advantage", or "disadvantage" (rolling two d20 and keeping the higher or
// lower). A natural roll at or above the attacker's crit threshold (default
// 20; 19 for a 戰士 with 精通重擊) crits and doubles damage dice, a natural 1
// always misses, temporary HP absorbs damage first, and the target's
// defeated flag is set when its HP reaches 0. Errors carry the exact TS
// throw messages.
func ResolveAttack(state CombatState, attackerID, targetID string, random RandomSource, advantage string) (CombatState, AttackResolution, error) {
	attackerIndex, targetIndex := -1, -1
	for i, entry := range state.Combatants {
		if attackerIndex < 0 && entry.ID == attackerID {
			attackerIndex = i
		}
		if targetIndex < 0 && entry.ID == targetID {
			targetIndex = i
		}
	}
	if attackerIndex < 0 || targetIndex < 0 {
		return CombatState{}, AttackResolution{}, errors.New("找不到攻擊者或目標")
	}
	attacker := state.Combatants[attackerIndex]
	target := state.Combatants[targetIndex]
	if attacker.Defeated {
		return CombatState{}, AttackResolution{}, fmt.Errorf("%s 已失去戰鬥能力", attacker.Name)
	}
	first := Die(20, random)
	second := first
	if advantage != "normal" {
		second = Die(20, random)
	}
	attackRoll := first
	switch advantage {
	case "advantage":
		attackRoll = max(first, second)
	case "disadvantage":
		attackRoll = min(first, second)
	}
	threshold := attacker.CritThreshold
	if threshold <= 0 || threshold > 20 {
		threshold = 20
	}
	critical := attackRoll >= threshold
	total := attackRoll + attacker.AttackBonus
	hit := attackRoll != 1 && (critical || total >= target.AC)
	damage := 0
	sneak := 0
	if hit {
		var err error
		damage, err = RollExpression(attacker.Damage, random, critical)
		if err != nil {
			return CombatState{}, AttackResolution{}, err
		}
		// 盜賊 偷襲: rider fires while any other ally on the attacker's side
		// still stands (the flanking assumption); crits double the dice.
		if attacker.SneakAttackDice > 0 {
			allyUp := false
			for _, entry := range state.Combatants {
				if entry.ID != attacker.ID && entry.Side == attacker.Side && !entry.Defeated {
					allyUp = true
					break
				}
			}
			if allyUp {
				diceCount := attacker.SneakAttackDice
				if critical {
					diceCount *= 2
				}
				for i := 0; i < diceCount; i++ {
					sneak += Die(6, random)
				}
				damage += sneak
			}
		}
	}
	temporaryHP := max(0, target.TemporaryHP)
	absorbed := min(temporaryHP, damage)
	nextHP := max(0, target.HP-(damage-absorbed))
	combatants := make([]Combatant, len(state.Combatants))
	for i, entry := range state.Combatants {
		if entry.ID == targetID {
			entry.TemporaryHP = temporaryHP - absorbed
			entry.HP = nextHP
			entry.Defeated = nextHP == 0
		}
		combatants[i] = entry
	}
	var text string
	if hit {
		criticalText := ""
		if critical {
			criticalText = "並造成重擊"
		}
		sneakText := ""
		if sneak > 0 {
			sneakText = fmt.Sprintf("（含偷襲 +%d）", sneak)
		}
		text = fmt.Sprintf("%s以 %d 命中%s，對 %s 造成 %d 點%s傷害%s。", attacker.Name, total, criticalText, target.Name, damage, attacker.DamageType, sneakText)
	} else {
		text = fmt.Sprintf("%s的攻擊結果為 %d，未命中 %s（AC %d）。", attacker.Name, total, target.Name, target.AC)
	}
	next := state
	next.Combatants = combatants
	return next, AttackResolution{AttackRoll: attackRoll, Total: total, Hit: hit, Critical: critical, Damage: damage, Text: text}, nil
}

// SyncPlayersFromCombat ports combat.ts syncPlayersFromCombat: it copies each
// matching combatant's hp/temporaryHp back onto the player and marks the
// player 倒地 when the combatant is at 0 HP. Players without a matching
// combatant are returned unchanged.
func SyncPlayersFromCombat(players []Character, state CombatState) []Character {
	out := make([]Character, len(players))
	for i, player := range players {
		for _, combatant := range state.Combatants {
			if combatant.PlayerID == player.ID {
				player.HP = combatant.HP
				player.TemporaryHP = combatant.TemporaryHP
				if combatant.HP == 0 {
					player.Condition = "倒地"
				}
				break
			}
		}
		out[i] = player
	}
	return out
}
