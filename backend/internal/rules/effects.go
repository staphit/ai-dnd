package rules

// Ported 1:1 from frontend/src/rules/effects.ts: automatic settlement of
// healing, damage, short rests, spell effects, and DM-declared effects.
//
// All functions treat their inputs as immutable, matching the TS spread
// copies: slices are copied before any element is modified and callers'
// structs are never mutated.

import (
	"errors"
	"fmt"
	"math"
)

// ApplyHealing ports effects.ts applyHealing. Healing never exceeds maxHp,
// and a character whose condition was exactly 倒地 stands back up (正常) once
// hp is above zero.
func ApplyHealing(c Character, amount int) Character {
	hp := min(c.MaxHP, c.HP+max(0, amount))
	if hp > 0 && c.Condition == "倒地" {
		c.Condition = "正常"
	}
	c.HP = hp
	return c
}

// ApplyDamage ports effects.ts applyDamage. Temporary hp absorbs damage
// first; the combatant is marked defeated when hp reaches exactly zero.
func ApplyDamage(cb Combatant, amount int) Combatant {
	damage := max(0, amount)
	temporaryHp := max(0, cb.TemporaryHP)
	absorbed := min(temporaryHp, damage)
	hp := max(0, cb.HP-(damage-absorbed))
	cb.TemporaryHP = temporaryHp - absorbed
	cb.HP = hp
	cb.Defeated = hp == 0
	return cb
}

// ShortRestResult mirrors the anonymous return object of effects.ts
// resolveShortRest.
type ShortRestResult struct {
	Character Character
	Healed    int
	DiceSpent int
}

// ResolveShortRest ports effects.ts resolveShortRest: hit dice are spent
// automatically (floor(random()*hitDie)+1+conModifier, floored at 0 per die)
// until the character is at full hp or out of dice. A nil random defaults to
// DefaultRandom, mirroring the TS `random = Math.random` default parameter.
func ResolveShortRest(c Character, random RandomSource) ShortRestResult {
	if random == nil {
		random = DefaultRandom
	}
	hp := c.HP
	hitDice := c.HitDice
	diceSpent := 0
	constitution := AbilityModifier(c.Abilities.Con)
	for hp < c.MaxHP && hitDice > 0 {
		heal := int(math.Floor(random()*float64(c.HitDie))) + 1 + constitution
		hp = min(c.MaxHP, hp+max(0, heal))
		hitDice--
		diceSpent++
	}
	next := c
	next.HP = hp
	next.HitDice = hitDice
	if hp > 0 && next.Condition == "倒地" {
		next.Condition = "正常"
	}
	return ShortRestResult{Character: next, Healed: hp - c.HP, DiceSpent: diceSpent}
}

// amountFor ports effects.ts amountFor: dice roll + flat bonus + (optional)
// spellcasting ability modifier, floored at zero. The error propagates the
// throw from rollExpression on a malformed dice expression.
func amountFor(effect SpellEffect, caster Character, random RandomSource) (int, error) {
	dice := 0
	if effect.Dice != "" {
		rolled, err := RollExpression(effect.Dice, random, false)
		if err != nil {
			return 0, err
		}
		dice = rolled
	}
	modifier := 0
	if effect.AddAbilityModifier && caster.Spellcasting != nil {
		modifier = AbilityModifier(caster.Abilities.Get(caster.Spellcasting.Ability))
	}
	return max(0, dice+effect.Flat+modifier), nil
}

// ForcedRolls mirrors the optional forcedRolls parameter of effects.ts
// resolveSpellEffect. Nil pointers mean "not forced" (the TS ?? nullish
// check), so a forced total of 0 is honored.
type ForcedRolls struct {
	AttackTotal *int
	SaveTotal   *int
}

// SpellEffectResult mirrors the return object of effects.ts
// resolveSpellEffect.
type SpellEffectResult struct {
	Players []Character
	Combat  *CombatState
	Amount  int
	Text    string
}

// ResolveSpellEffect ports effects.ts resolveSpellEffect. The target may be a
// player, a combatant, or both (a party combatant matched by playerId). The
// attack roll is checked first (a miss zeroes the amount), then the saving
// throw (halfOnSave floors amount/2, otherwise zero). Player and combat state
// are kept in sync in both directions. A nil random defaults to
// DefaultRandom, mirroring the TS `random = Math.random` default parameter.
// Errors carry the exact TS throw messages 找不到施法者 / 找不到法術目標.
func ResolveSpellEffect(players []Character, combat *CombatState, casterID, targetID string, effect SpellEffect, random RandomSource, forced *ForcedRolls) (SpellEffectResult, error) {
	if random == nil {
		random = DefaultRandom
	}
	var caster *Character
	for i := range players {
		if players[i].ID == casterID {
			caster = &players[i]
			break
		}
	}
	var targetPlayer *Character
	for i := range players {
		if players[i].ID == targetID {
			targetPlayer = &players[i]
			break
		}
	}
	var targetCombatant *Combatant
	if combat != nil {
		for i := range combat.Combatants {
			entry := &combat.Combatants[i]
			if entry.ID == targetID || entry.PlayerID == targetID {
				targetCombatant = entry
				break
			}
		}
	}
	if caster == nil {
		return SpellEffectResult{}, errors.New("找不到施法者")
	}
	if targetPlayer == nil && targetCombatant == nil {
		return SpellEffectResult{}, errors.New("找不到法術目標")
	}
	amount, err := amountFor(effect, *caster, random)
	if err != nil {
		return SpellEffectResult{}, err
	}
	outcome := ""
	if effect.AttackRoll && targetCombatant != nil {
		var attack int
		if forced != nil && forced.AttackTotal != nil {
			attack = *forced.AttackTotal
		} else {
			bonus := 0
			if caster.Spellcasting != nil {
				bonus = caster.Spellcasting.AttackBonus
			}
			attack = int(math.Floor(random()*20)) + 1 + bonus
		}
		if attack < targetCombatant.AC {
			amount = 0
			outcome = "法術攻擊未命中"
		}
	}
	if effect.SaveAbility != "" && targetCombatant != nil && outcome == "" {
		var save int
		if forced != nil && forced.SaveTotal != nil {
			save = *forced.SaveTotal
		} else {
			// Nil-map lookups return 0, matching savingThrows?.[ability] || 0.
			save = int(math.Floor(random()*20)) + 1 + targetCombatant.SavingThrows[effect.SaveAbility]
		}
		dc := 10 // caster.spellcasting?.saveDc || 10
		if caster.Spellcasting != nil && caster.Spellcasting.SaveDC != 0 {
			dc = caster.Spellcasting.SaveDC
		}
		if save >= dc {
			if effect.HalfOnSave {
				amount = amount / 2 // amount >= 0, so truncation == Math.floor
			} else {
				amount = 0
			}
			if amount != 0 {
				outcome = fmt.Sprintf("豁免成功，承受 %d 點", amount)
			} else {
				outcome = "豁免成功，未受影響"
			}
		}
	}
	nextPlayers := players
	nextCombat := combat
	if effect.Kind == "healing" && targetPlayer != nil {
		out := make([]Character, len(players))
		for i, entry := range players {
			if entry.ID == targetPlayer.ID {
				out[i] = ApplyHealing(entry, amount)
			} else {
				out[i] = entry
			}
		}
		nextPlayers = out
	}
	if effect.Kind == "temporaryHp" && targetPlayer != nil {
		out := make([]Character, len(players))
		copy(out, players)
		for i := range out {
			if out[i].ID == targetPlayer.ID {
				out[i].TemporaryHP = max(out[i].TemporaryHP, amount)
			}
		}
		nextPlayers = out
	}
	if effect.Kind == "condition" && targetPlayer != nil {
		condition := effect.Condition
		if condition == "" {
			condition = "正常"
		}
		out := make([]Character, len(players))
		copy(out, players)
		for i := range out {
			if out[i].ID == targetPlayer.ID {
				out[i].Condition = condition
			}
		}
		nextPlayers = out
	}
	if effect.Kind == "damage" && targetCombatant != nil && combat != nil {
		combatants := make([]Combatant, len(combat.Combatants))
		for i, entry := range combat.Combatants {
			if entry.ID == targetCombatant.ID {
				combatants[i] = ApplyDamage(entry, amount)
			} else {
				combatants[i] = entry
			}
		}
		nc := *combat
		nc.Combatants = combatants
		nextCombat = &nc
		var updated Combatant
		for _, entry := range combatants {
			if entry.ID == targetCombatant.ID {
				updated = entry
				break
			}
		}
		if targetCombatant.PlayerID != "" {
			out := make([]Character, len(players))
			copy(out, players)
			for i := range out {
				if out[i].ID == targetCombatant.PlayerID {
					out[i].HP = updated.HP
					out[i].TemporaryHP = updated.TemporaryHP
					if updated.HP == 0 {
						out[i].Condition = "倒地"
					}
				}
			}
			nextPlayers = out
		}
	}
	if targetPlayer != nil && nextCombat != nil && effect.Kind != "damage" {
		var updatedPlayer Character
		for _, entry := range nextPlayers {
			if entry.ID == targetPlayer.ID {
				updatedPlayer = entry
				break
			}
		}
		combatants := make([]Combatant, len(nextCombat.Combatants))
		copy(combatants, nextCombat.Combatants)
		for i := range combatants {
			if combatants[i].PlayerID == targetPlayer.ID {
				combatants[i].HP = updatedPlayer.HP
				combatants[i].TemporaryHP = updatedPlayer.TemporaryHP
				combatants[i].Defeated = updatedPlayer.HP == 0
			}
		}
		nc := *nextCombat
		nc.Combatants = combatants
		nextCombat = &nc
	}
	targetName := ""
	if targetCombatant != nil {
		targetName = targetCombatant.Name
	}
	if targetName == "" && targetPlayer != nil {
		targetName = targetPlayer.Name
	}
	if targetName == "" {
		targetName = "目標"
	}
	if outcome == "" {
		switch effect.Kind {
		case "damage":
			outcome = fmt.Sprintf("受到 %d 點%s傷害", amount, effect.DamageType)
		case "healing":
			outcome = fmt.Sprintf("恢復 %d 點生命", amount)
		case "temporaryHp":
			outcome = fmt.Sprintf("獲得 %d 點暫時生命", amount)
		default:
			outcome = fmt.Sprintf("狀態變為「%s」", effect.Condition)
		}
	}
	return SpellEffectResult{Players: nextPlayers, Combat: nextCombat, Amount: amount, Text: targetName + outcome + "。"}, nil
}

// DMEffect mirrors effects.ts DmEffect: an effect declared by the DM in a
// structured response. Kind is damage | healing | condition.
type DMEffect struct {
	TargetID  string `json:"targetId"`
	Kind      string `json:"kind"`
	Amount    int    `json:"amount"`
	Condition string `json:"condition"`
	Reason    string `json:"reason"`
}

// dmCondition mirrors String(effect.condition || '正常').slice(0, 40). JS
// slices UTF-16 code units; runes are equivalent for the BMP (all Chinese)
// strings this handles.
func dmCondition(condition string) string {
	if condition == "" {
		condition = "正常"
	}
	runes := []rune(condition)
	if len(runes) > 40 {
		runes = runes[:40]
	}
	return string(runes)
}

// ApplyDmEffects ports effects.ts applyDmEffects: at most the first 8 effects
// apply, amounts clamp to 0..500, effects for unknown targets are skipped,
// and every applied effect appends a Chinese log line.
func ApplyDmEffects(players []Character, effects []DMEffect) ([]Character, []string) {
	next := players
	logs := []string{}
	if len(effects) > 8 {
		effects = effects[:8]
	}
	for _, effect := range effects {
		var target *Character
		for i := range next {
			if next[i].ID == effect.TargetID {
				target = &next[i]
				break
			}
		}
		if target == nil {
			continue
		}
		targetID := target.ID
		targetName := target.Name
		amount := max(0, min(500, effect.Amount))
		if effect.Kind == "damage" {
			out := make([]Character, len(next))
			copy(out, next)
			for i := range out {
				if out[i].ID != targetID {
					continue
				}
				temporaryHp := max(0, out[i].TemporaryHP)
				absorbed := min(temporaryHp, amount)
				hp := max(0, out[i].HP-(amount-absorbed))
				out[i].TemporaryHP = temporaryHp - absorbed
				out[i].HP = hp
				if hp == 0 {
					out[i].Condition = "倒地"
				}
			}
			next = out
		}
		if effect.Kind == "healing" {
			out := make([]Character, len(next))
			for i, entry := range next {
				if entry.ID == targetID {
					out[i] = ApplyHealing(entry, amount)
				} else {
					out[i] = entry
				}
			}
			next = out
		}
		if effect.Kind == "condition" {
			out := make([]Character, len(next))
			copy(out, next)
			for i := range out {
				if out[i].ID == targetID {
					out[i].Condition = dmCondition(effect.Condition)
				}
			}
			next = out
		}
		var detail string
		switch effect.Kind {
		case "damage":
			detail = fmt.Sprintf("-%d HP", amount)
		case "healing":
			detail = fmt.Sprintf("+%d HP", amount)
		default:
			detail = fmt.Sprintf("狀態：%s", dmCondition(effect.Condition))
		}
		logs = append(logs, fmt.Sprintf("%s：%s（%s）", targetName, effect.Reason, detail))
	}
	return next, logs
}
