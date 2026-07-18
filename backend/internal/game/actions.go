package game

import (
	"encoding/json"
	"fmt"
	"strings"

	"dndduet/internal/apperr"
	"dndduet/internal/rules"
	"dndduet/internal/store"
)

// gameState is the loaded mutable state for one campaign action.
type gameState struct {
	row     store.CampaignRow
	players []rules.Character
	combat  *rules.CombatState
	pending map[string]string
	check   *rules.RequiredCheck
	arc     *StoryArc
	script  *ScriptState
}

func (s *Service) loadState(id string) (*gameState, error) {
	row, err := s.mustCampaign(id)
	if err != nil {
		return nil, err
	}
	players, err := s.loadCharacters(id)
	if err != nil {
		return nil, err
	}
	st := &gameState{row: row, players: players, pending: map[string]string{}}
	if data, ok, err := s.store.Combat(id); err != nil {
		return nil, err
	} else if ok {
		st.combat = &rules.CombatState{}
		if err := json.Unmarshal([]byte(data), st.combat); err != nil {
			return nil, fmt.Errorf("combat document for %s is corrupt: %w", id, err)
		}
	}
	if row.Pending != "" {
		json.Unmarshal([]byte(row.Pending), &st.pending)
	}
	if row.RequiredCheck != "" {
		st.check = &rules.RequiredCheck{}
		if err := json.Unmarshal([]byte(row.RequiredCheck), st.check); err != nil {
			st.check = nil
		}
	}
	if data, ok, err := s.store.StoryArc(id); err != nil {
		return nil, err
	} else if ok {
		st.arc = &StoryArc{}
		if err := json.Unmarshal([]byte(data), st.arc); err != nil {
			st.arc = nil
		}
	}
	if data, ok, err := s.store.ScriptState(id); err != nil {
		return nil, err
	} else if ok {
		st.script = &ScriptState{}
		if err := json.Unmarshal([]byte(data), st.script); err != nil {
			st.script = nil
		}
	}
	// Campaigns without an arc (including pre-feature ones) get a fresh clock
	// starting at the current round; it persists on the next state write.
	if st.arc == nil {
		st.arc = defaultStoryArc(row.Round, row.Objective)
	}
	// Scripted campaigns keep the arc pinned to the module: goals, deadlines
	// and the current phase all derive from the node graph.
	if mod := scriptModuleFor(st.script); mod != nil {
		syncScriptArc(st.arc, mod, st.script, row.Round)
	}
	// Weapons bought before shop weapons granted attacks: backfill the attack
	// entries so carried catalog weapons are usable (persists on next write).
	for i := range st.players {
		ensureShopWeaponAttacks(&st.players[i])
		rules.EnsureDerivedDefaults(&st.players[i])
	}
	return st, nil
}

// persist writes back the mutated parts of the state and returns the fresh view.
func (s *Service) persist(st *gameState, logs []string) (View, error) {
	if err := s.saveCharacters(st.row.ID, st.players); err != nil {
		return View{}, err
	}
	if st.combat != nil {
		data, err := json.Marshal(st.combat)
		if err != nil {
			return View{}, err
		}
		if err := s.store.SaveCombat(st.row.ID, string(data), s.now().UnixMilli()); err != nil {
			return View{}, err
		}
	}
	if st.arc != nil {
		data, err := json.Marshal(st.arc)
		if err != nil {
			return View{}, err
		}
		if err := s.store.SaveStoryArc(st.row.ID, string(data), s.now().UnixMilli()); err != nil {
			return View{}, err
		}
	}
	if st.script != nil {
		data, err := json.Marshal(st.script)
		if err != nil {
			return View{}, err
		}
		if err := s.store.SaveScriptState(st.row.ID, string(data), s.now().UnixMilli()); err != nil {
			return View{}, err
		}
	}
	pending, err := json.Marshal(st.pending)
	if err != nil {
		return View{}, err
	}
	st.row.Pending = string(pending)
	if st.check != nil {
		check, err := json.Marshal(st.check)
		if err != nil {
			return View{}, err
		}
		st.row.RequiredCheck = string(check)
	} else {
		st.row.RequiredCheck = ""
	}
	if st.row, err = s.touch(st.row); err != nil {
		return View{}, err
	}
	if len(logs) > 0 {
		entries := make([]store.StoryRow, 0, len(logs))
		for _, text := range logs {
			entries = append(entries, store.StoryRow{Speaker: "system", Text: text})
		}
		if err := s.AppendStory(st.row.ID, entries); err != nil {
			return View{}, err
		}
	}
	return s.assembleView(st.row)
}

func (st *gameState) player(playerID string) (*rules.Character, error) {
	for i := range st.players {
		if st.players[i].ID == playerID {
			return &st.players[i], nil
		}
	}
	return nil, apperr.New(404, "找不到這名角色。")
}

// combatantName resolves a target id to a display name like the frontend
// castSpell target lookup.
func (st *gameState) combatantName(targetID string) string {
	if targetID == "scene" {
		return "目前場景"
	}
	for _, p := range st.players {
		if p.ID == targetID {
			return p.Name
		}
	}
	if st.combat != nil {
		for _, c := range st.combat.Combatants {
			if c.ID == targetID || c.PlayerID == targetID {
				return c.Name
			}
		}
	}
	if targetID == "" {
		return "自身"
	}
	return targetID
}

// CastParams is the cast-spell request body.
type CastParams struct {
	SpellID     string `json:"spellId"`
	AsRitual    bool   `json:"asRitual"`
	TargetID    string `json:"targetId"`
	AttackTotal *int   `json:"attackTotal"`
}

// CastResult carries either the updated view or a pending attack-roll request
// (nothing mutated yet) mirroring the frontend spellRoll dialog flow.
type CastResult struct {
	View            *View                `json:"view,omitempty"`
	NeedsAttackRoll *rules.RequiredCheck `json:"needsAttackRoll,omitempty"`
}

// CastSpell ports App.tsx castSpell + applySpellCast: validates, spends the
// slot/resource, resolves the structured effect with server dice, spends the
// combat action, logs, and (out of combat) locks the declared action.
func (s *Service) CastSpell(id, playerID string, p CastParams) (CastResult, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return CastResult{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return CastResult{}, err
	}
	if st.check != nil {
		return CastResult{}, apperr.New(400, "請先完成目前畫面上的必要擲骰，再進行施法。")
	}
	if st.pending[playerID] != "" {
		return CastResult{}, apperr.New(400, player.Name+"本回合已鎖定行動，請先解鎖才能改為施法。")
	}
	if player.Spellcasting == nil {
		return CastResult{}, apperr.New(400, player.Name+" 沒有施法能力。")
	}
	var spell *rules.Spell
	for i := range player.Spellcasting.Spells {
		if player.Spellcasting.Spells[i].ID == p.SpellID {
			spell = &player.Spellcasting.Spells[i]
			break
		}
	}
	if spell == nil {
		return CastResult{}, apperr.New(404, "找不到這個法術。")
	}
	combatActive := st.combat != nil && st.combat.Active
	if spell.Effect != nil && spell.Effect.Kind == "damage" && !combatActive {
		return CastResult{}, apperr.New(400, "傷害法術必須在戰鬥追蹤器有有效目標時結算。")
	}
	if p.TargetID == "" {
		return CastResult{}, apperr.New(400, spell.Name+" 必須先指定目標。")
	}
	if combatActive {
		// Dry-run the action-economy spend so an illegal cast fails before any
		// slot is consumed (mirrors the frontend pre-check).
		if _, err := rules.SpendCombatResource(*st.combat, playerID, rules.CombatResourceForCastingTime(spell.CastingTime)); err != nil {
			return CastResult{}, apperr.New(400, err.Error())
		}
	}

	usedFree := false
	if spell.FreeUseResourceID != "" {
		for _, r := range player.Resources {
			if r.ID == spell.FreeUseResourceID && r.Current > 0 {
				usedFree = true
				break
			}
		}
	}

	// Attack-roll spells need a d20 from the player first: return the roll
	// request without mutating anything (frontend DiceTray flow).
	if spell.Effect != nil && spell.Effect.AttackRoll && p.AttackTotal == nil {
		var target *rules.Combatant
		if st.combat != nil {
			for i := range st.combat.Combatants {
				c := &st.combat.Combatants[i]
				if c.ID == p.TargetID || c.PlayerID == p.TargetID {
					target = c
					break
				}
			}
		}
		if target == nil {
			return CastResult{}, apperr.New(400, spell.Name+" 找不到可供法術攻擊的目標。")
		}
		ability := "int"
		if player.Spellcasting.Ability != "" {
			ability = player.Spellcasting.Ability
		}
		return CastResult{NeedsAttackRoll: &rules.RequiredCheck{
			Character: player.Name,
			Ability:   rules.AbilityLabels[ability],
			Skill:     spell.Name + "法術攻擊",
			DC:        target.AC,
			Modifier:  player.Spellcasting.AttackBonus,
			Reason:    fmt.Sprintf("以法術攻擊命中 %s（AC %d）後才會結算效果。", target.Name, target.AC),
			PlayerID:  playerID,
		}}, nil
	}

	spent, ok := rules.SpendSpellSlot(*player, *spell, p.AsRitual)
	if !ok {
		return CastResult{}, apperr.New(400, fmt.Sprintf("%s 沒有可用的 %d 環法術位。", player.Name, spell.Level))
	}
	*player = spent

	mode := ""
	switch {
	case p.AsRitual:
		mode = "以儀式"
	case spell.Level == 0:
		mode = "施展戲法"
	case usedFree:
		mode = "使用免費施法能力施放"
	default:
		mode = fmt.Sprintf("消耗 %d 環以上法術位施放", spell.Level)
	}

	detail := ""
	var combatLogs []string
	if spell.Effect != nil {
		var forced *rules.ForcedRolls
		if p.AttackTotal != nil {
			forced = &rules.ForcedRolls{AttackTotal: p.AttackTotal}
		}
		result, err := rules.ResolveSpellEffect(st.players, st.combat, playerID, p.TargetID, *spell.Effect, nil, forced)
		if err != nil {
			return CastResult{}, apperr.New(400, err.Error())
		}
		st.players = result.Players
		if result.Combat != nil {
			if st.combat != nil && st.combat.Active {
				// Route through applyCombatChange so a spell dropping the last
				// enemy pays victory XP exactly like a weapon kill.
				priorDown := make(map[string]bool, len(st.combat.Combatants))
				for _, c := range st.combat.Combatants {
					priorDown[c.ID] = c.Defeated
				}
				newlyDown := false
				for _, c := range result.Combat.Combatants {
					if c.Side == "enemy" && c.Defeated && !priorDown[c.ID] {
						newlyDown = true
						break
					}
				}
				combatLogs = append(combatLogs, s.applyCombatChange(st, *result.Combat)...)
				// 魔契師 黑暗者賜福: spell kills (eldritch blast included) grant
				// temp HP just like weapon kills.
				if newlyDown {
					if caster, err := st.player(playerID); err == nil && rules.HasClass(*caster, "魔契師") {
						grant := rules.AbilityModifier(caster.Abilities.Cha) + classLevelOf(*caster, "魔契師")
						if grant > 0 && caster.TemporaryHP < grant {
							caster.TemporaryHP = grant
							syncCombatFromPlayers(st)
							combatLogs = append(combatLogs, fmt.Sprintf("%s的「黑暗者賜福」發動：獲得 %d 點暫時生命。", caster.Name, grant))
						}
					}
				}
			} else {
				st.combat = result.Combat
			}
		}
		detail = " " + result.Text
	}

	if st.combat != nil && st.combat.Active {
		next, err := rules.SpendCombatResource(*st.combat, playerID, rules.CombatResourceForCastingTime(spell.CastingTime))
		if err != nil {
			return CastResult{}, apperr.New(400, err.Error())
		}
		*st.combat = next
	}

	if !combatActive {
		st.pending[playerID] = fmt.Sprintf("對%s施放「%s」", st.combatantName(p.TargetID), spell.Name)
	}

	view, err := s.persist(st, append([]string{fmt.Sprintf("%s%s「%s」。%s", player.Name, mode, spell.Name, detail)}, combatLogs...))
	if err != nil {
		return CastResult{}, err
	}
	return CastResult{View: &view}, nil
}

// Rest ports App.tsx rest(): short rest auto-spends hit dice, long rest
// restores everything; both advance the exploration-time round counter.
func (s *Service) Rest(id, playerID, restType string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	if st.combat != nil && st.combat.Active {
		return View{}, apperr.New(400, "戰鬥進行中不能休息。")
	}
	if st.check != nil {
		return View{}, apperr.New(400, "請先完成目前的必要擲骰，再決定是否休息。")
	}
	if len(st.pending) > 0 {
		return View{}, apperr.New(400, "隊伍已有待裁定行動；請先完成或解鎖本輪行動，再開始休息。")
	}
	if restType != "short" && restType != "long" {
		return View{}, apperr.New(400, "休息類型必須是 short 或 long。")
	}

	recovered := rules.RestCharacter(*player, restType)
	label := "長休"
	actionCost := 4
	detail := "，生命、生命骰、法術位與職業資源已完全恢復，專注已結束"
	if restType == "short" {
		label = "短休"
		actionCost = 1
		shortRest := rules.ResolveShortRest(recovered, nil)
		detail = fmt.Sprintf("，消耗 %d 顆 d%d 生命骰、恢復 %d HP；短休資源已恢復，一般法術位不恢復", shortRest.DiceSpent, player.HitDie, shortRest.Healed)
		recovered = shortRest.Character
	}
	*player = recovered
	st.row.Round += actionCost

	return s.persist(st, []string{fmt.Sprintf("%s完成%s（消耗 %d 點探索行動時間）%s。", player.Name, label, actionCost, detail)})
}

// LevelUp applies one class level (validating XP thresholds in rules).
func (s *Service) LevelUp(id, playerID, className string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	next, err := rules.LevelUpCharacter(*player, className)
	if err != nil {
		return View{}, apperr.New(400, err.Error())
	}
	*player = next
	return s.persist(st, []string{fmt.Sprintf("%s升級：%s 現在是 %d 級。", player.Name, player.ClassName, player.Level)})
}

// SpendAbilityPointAction spends one banked ability point.
func (s *Service) SpendAbilityPointAction(id, playerID, ability string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	next, err := rules.SpendAbilityPoint(*player, ability)
	if err != nil {
		return View{}, apperr.New(400, err.Error())
	}
	*player = next
	return s.persist(st, nil)
}

// SetPrepared replaces the prepared-spell selection.
func (s *Service) SetPrepared(id, playerID string, spellIDs []string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	*player = rules.SetPreparedSpells(*player, spellIDs)
	return s.persist(st, nil)
}

// ChangeResourceAction adjusts a class resource by delta (clamped in rules).
// Spending (delta < 0) also applies the mechanical effect of resources the
// server understands: 動作如潮 restores the current turn's action, 回氣 heals
// 1d10 + level as a bonus action, 聖療 heals the most injured party member,
// 奧術回復 and 魔法巧思 restore expended spell slots, 引導神力 (牧師) runs
// 保存生機, and 吟遊激勵 queues a d6 for the next required check.
func (s *Service) ChangeResourceAction(id, playerID, resourceID string, delta int) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	combatActive := st.combat != nil && st.combat.Active

	resourceAt := func(c rules.Character) (current int, name string) {
		for _, r := range c.Resources {
			if r.ID == resourceID {
				return r.Current, r.Name
			}
		}
		return 0, resourceID
	}
	prev, name := resourceAt(*player)

	// Pre-validate effects that have combat requirements, before spending.
	spending := delta < 0
	if spending && prev <= 0 {
		return View{}, apperr.New(400, fmt.Sprintf("%s的「%s」已用盡。", player.Name, name))
	}
	if spending && resourceID == "action_surge" {
		if !combatActive {
			return View{}, apperr.New(400, "動作如潮只能在戰鬥中自己的回合使用。")
		}
		if st.combat.TurnIndex < 0 || st.combat.TurnIndex >= len(st.combat.Combatants) ||
			st.combat.Combatants[st.combat.TurnIndex].PlayerID != playerID {
			return View{}, apperr.New(400, "現在不是"+player.Name+"的回合，無法發動動作如潮。")
		}
	}
	if spending && resourceID == "second_wind" && player.HP == 0 {
		return View{}, apperr.New(400, player.Name+"已倒地失去意識，無法使用回氣；請隊友先救援。")
	}
	if spending && resourceID == "second_wind" && combatActive {
		// Bonus action cost; dry-run so an illegal use fails before the spend.
		if _, err := rules.SpendCombatResource(*st.combat, playerID, "bonusAction"); err != nil {
			return View{}, apperr.New(400, err.Error())
		}
	}
	if spending && resourceID == "arcane_recovery" {
		if combatActive {
			return View{}, apperr.New(400, "奧術回復需要在戰鬥外進行。")
		}
		if len(restorableSlotLevels(*player, 2)) == 0 {
			return View{}, apperr.New(400, player.Name+"沒有可用奧術回復恢復的已消耗法術位。")
		}
	}
	if spending && resourceID == "magical_cunning" && expendedPactSlots(*player) == 0 {
		return View{}, apperr.New(400, player.Name+"沒有已消耗的契約法術位可恢復。")
	}
	if spending && resourceID == "channel_divinity" && rules.HasClass(*player, "牧師") && preserveLifeTargetIndex(st.players) < 0 {
		return View{}, apperr.New(400, "隊伍中沒有生命低於一半上限的成員，保存生機無法分配治療。")
	}
	if spending && resourceID == "lay_on_hands" {
		if idx := mostInjuredIndex(st.players); idx < 0 || st.players[idx].HP >= st.players[idx].MaxHP {
			return View{}, apperr.New(400, "隊伍成員都是滿血狀態，不需要聖療。")
		}
	}
	if spending && resourceID == "bardic_inspiration" && player.PendingInspiration > 0 {
		return View{}, apperr.New(400, "已有一顆吟遊激勵骰待用，會在下一次必要檢定自動加入。")
	}

	*player = rules.ChangeResource(*player, resourceID, delta)
	next, _ := resourceAt(*player)

	var logs []string
	if spending && next < prev {
		switch resourceID {
		case "action_surge":
			actor := st.combat.Combatants[st.combat.TurnIndex]
			economy := make(map[string]rules.TurnEconomy, len(st.combat.TurnEconomy)+1)
			for k, v := range st.combat.TurnEconomy {
				economy[k] = v
			}
			usage := economy[actor.ID]
			usage.ActionUsed = false
			economy[actor.ID] = usage
			st.combat.TurnEconomy = economy
			logs = append(logs, fmt.Sprintf("%s發動「動作如潮」：本回合可以再進行一個動作。", player.Name))
		case "second_wind":
			heal := rules.Die(10, s.dice) + player.Level
			healed := min(player.MaxHP-player.HP, heal)
			if healed < 0 {
				healed = 0
			}
			player.HP += healed
			if combatActive {
				spent, err := rules.SpendCombatResource(*st.combat, playerID, "bonusAction")
				if err == nil {
					*st.combat = spent
				}
				for i := range st.combat.Combatants {
					if st.combat.Combatants[i].PlayerID == playerID {
						st.combat.Combatants[i].HP = player.HP
						break
					}
				}
			}
			suffix := ""
			if combatActive {
				suffix = "（已使用附贈動作）"
			}
			logs = append(logs, fmt.Sprintf("%s使用「回氣」恢復 %d 生命，現在 HP %d/%d%s。", player.Name, healed, player.HP, player.MaxHP, suffix))
		case "arcane_recovery":
			restored := restoreStandardSlots(player, 2)
			logs = append(logs, fmt.Sprintf("%s使用「奧術回復」：恢復%s。", player.Name, describeSlotLevels(restored)))
		case "magical_cunning":
			restored := restorePactSlots(player)
			logs = append(logs, fmt.Sprintf("%s使用「魔法巧思」：恢復 %d 個契約法術位。", player.Name, restored))
		case "channel_divinity":
			if !rules.HasClass(*player, "牧師") {
				// Legacy non-cleric holders just track the charge.
				logs = append(logs, fmt.Sprintf("%s使用「%s」（剩 %d/%d）。", player.Name, name, next, resourceMax(*player, resourceID)))
				break
			}
			pool := 5 * classLevelOf(*player, "牧師")
			details := preserveLife(st, pool)
			syncCombatFromPlayers(st)
			logs = append(logs, fmt.Sprintf("%s引導神力發動「保存生機」（治療池 %d）：%s。", player.Name, pool, strings.Join(details, "、")))
		case "lay_on_hands":
			points := prev - next
			idx := mostInjuredIndex(st.players)
			target := &st.players[idx]
			before := target.HP
			*target = rules.ApplyHealing(*target, points)
			syncCombatFromPlayers(st)
			logs = append(logs, fmt.Sprintf("%s以「聖療」治療%s %d 點生命（HP %d/%d，治療池剩 %d/%d）。",
				player.Name, target.Name, target.HP-before, target.HP, target.MaxHP, next, resourceMax(*player, resourceID)))
		case "bardic_inspiration":
			player.PendingInspiration = 1
			logs = append(logs, fmt.Sprintf("%s奏響「吟遊激勵」：下一次任何隊員的必要檢定將額外加骰 1d6。", player.Name))
		default:
			logs = append(logs, fmt.Sprintf("%s使用「%s」（剩 %d/%d）。", player.Name, name, next, resourceMax(*player, resourceID)))
		}
	}
	return s.persist(st, logs)
}

// restorableSlotLevels reports which standard (non-pact) expended slot levels
// could be restored within the given total-slot-level budget, lowest first.
func restorableSlotLevels(c rules.Character, budget int) []int {
	if c.Spellcasting == nil || c.Spellcasting.Mode == "pact" {
		return nil
	}
	var restorable []int
	for _, slot := range c.Spellcasting.Slots { // stored in ascending level order
		for missing := slot.Max - slot.Current; missing > 0 && budget >= slot.Level; missing-- {
			restorable = append(restorable, slot.Level)
			budget -= slot.Level
		}
	}
	return restorable
}

// restoreStandardSlots applies 奧術回復: it restores expended standard spell
// slots, lowest level first, until the slot-level budget runs out, returning
// the restored slot levels.
func restoreStandardSlots(c *rules.Character, budget int) []int {
	restored := restorableSlotLevels(*c, budget)
	if len(restored) == 0 {
		return nil
	}
	casting := *c.Spellcasting
	slots := make([]rules.SlotPool, len(casting.Slots))
	copy(slots, casting.Slots)
	for _, level := range restored {
		for i := range slots {
			if slots[i].Level == level && slots[i].Current < slots[i].Max {
				slots[i].Current++
				break
			}
		}
	}
	casting.Slots = slots
	c.Spellcasting = &casting
	return restored
}

// describeSlotLevels renders restored slot levels like "1 環法術位 ×2".
func describeSlotLevels(levels []int) string {
	counts := map[int]int{}
	var order []int
	for _, level := range levels {
		if counts[level] == 0 {
			order = append(order, level)
		}
		counts[level]++
	}
	parts := make([]string, 0, len(order))
	for _, level := range order {
		parts = append(parts, fmt.Sprintf("%d 環法術位 ×%d", level, counts[level]))
	}
	return strings.Join(parts, "、")
}

// expendedPactSlots counts the warlock's missing pact slots.
func expendedPactSlots(c rules.Character) int {
	if c.Spellcasting == nil || c.Spellcasting.Mode != "pact" || len(c.Spellcasting.Slots) == 0 {
		return 0
	}
	pool := c.Spellcasting.Slots[0]
	return max(0, pool.Max-pool.Current)
}

// restorePactSlots applies 魔法巧思: it restores up to ceil(max/2) expended
// pact slots and returns how many came back.
func restorePactSlots(c *rules.Character) int {
	restore := min((c.Spellcasting.Slots[0].Max+1)/2, expendedPactSlots(*c))
	if restore <= 0 {
		return 0
	}
	casting := *c.Spellcasting
	slots := make([]rules.SlotPool, len(casting.Slots))
	copy(slots, casting.Slots)
	slots[0].Current += restore
	casting.Slots = slots
	c.Spellcasting = &casting
	return restore
}

// preserveLifeTargetIndex finds the most injured party member still below
// half max HP (-1 when nobody qualifies).
func preserveLifeTargetIndex(players []rules.Character) int {
	idx := -1
	for i := range players {
		if players[i].HP >= players[i].MaxHP/2 {
			continue
		}
		if idx < 0 || players[i].HP < players[idx].HP {
			idx = i
		}
	}
	return idx
}

// preserveLife distributes the 保存生機 healing pool: the most injured
// eligible member is topped up first, and nobody is healed above half their
// max HP. Returns per-member healing details for the log.
func preserveLife(st *gameState, pool int) []string {
	var details []string
	for pool > 0 {
		idx := preserveLifeTargetIndex(st.players)
		if idx < 0 {
			break
		}
		target := &st.players[idx]
		heal := min(pool, target.MaxHP/2-target.HP)
		*target = rules.ApplyHealing(*target, heal)
		pool -= heal
		details = append(details, fmt.Sprintf("%s +%d HP", target.Name, heal))
	}
	return details
}

// mostInjuredIndex finds the party member with the lowest current HP.
func mostInjuredIndex(players []rules.Character) int {
	idx := -1
	for i := range players {
		if idx < 0 || players[i].HP < players[idx].HP {
			idx = i
		}
	}
	return idx
}

// classLevelOf reads the character's level in one class (falling back to the
// total level for pre-multiclass documents).
func classLevelOf(c rules.Character, className string) int {
	for _, entry := range rules.GetCharacterClasses(c) {
		if entry.ClassName == className {
			return entry.Level
		}
	}
	return c.Level
}

// syncCombatFromPlayers pushes updated player HP / temporary HP back onto
// their party combatants (the inverse of rules.SyncPlayersFromCombat).
func syncCombatFromPlayers(st *gameState) {
	if st.combat == nil {
		return
	}
	for i := range st.combat.Combatants {
		combatant := &st.combat.Combatants[i]
		if combatant.PlayerID == "" {
			continue
		}
		for _, p := range st.players {
			if p.ID == combatant.PlayerID {
				combatant.HP = p.HP
				combatant.TemporaryHP = p.TemporaryHP
				combatant.Defeated = p.HP == 0
				break
			}
		}
	}
}

func resourceMax(c rules.Character, resourceID string) int {
	for _, r := range c.Resources {
		if r.ID == resourceID {
			return r.Max
		}
	}
	return 0
}

// CharacterPatch carries cosmetic/customize edits.
type CharacterPatch struct {
	Species     *string              `json:"species"`
	Background  *string              `json:"background"`
	Abilities   *rules.AbilityScores `json:"abilities"`
	Appearance  *string              `json:"appearance"`
	PortraitURL *string              `json:"portraitUrl"`
}

// UpdateCharacter ports CharacterManager customize (species/background/
// abilities re-derive stats) plus plain cosmetic fields.
func (s *Service) UpdateCharacter(id, playerID string, patch CharacterPatch) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	if patch.Species != nil || patch.Background != nil || patch.Abilities != nil {
		species, background := "", ""
		if patch.Species != nil {
			species = *patch.Species
		}
		if patch.Background != nil {
			background = *patch.Background
		}
		*player = rules.CustomizeCharacter(*player, species, background, patch.Abilities)
	}
	if patch.Appearance != nil {
		player.Appearance = clampStr(*patch.Appearance, 1200)
	}
	if patch.PortraitURL != nil {
		player.PortraitURL = clampStr(*patch.PortraitURL, 500)
	}
	return s.persist(st, nil)
}

// SubmitAction locks one player's declared action for the round.
func (s *Service) SubmitAction(id, playerID, text string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	if _, err := st.player(playerID); err != nil {
		return View{}, err
	}
	if st.pending[playerID] != "" {
		return s.persist(st, nil) // already locked; no-op like the frontend guard
	}
	st.pending[playerID] = clampStr(text, 2000)
	return s.persist(st, nil)
}

// UnlockAction releases one player's declared action.
func (s *Service) UnlockAction(id, playerID string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	delete(st.pending, playerID)
	return s.persist(st, nil)
}
