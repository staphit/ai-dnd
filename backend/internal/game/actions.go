package game

import (
	"encoding/json"
	"fmt"

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
	// Campaigns without an arc (including pre-feature ones) get a fresh clock
	// starting at the current round; it persists on the next state write.
	if st.arc == nil {
		st.arc = defaultStoryArc(row.Round, row.Objective)
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
			st.combat = result.Combat
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

	view, err := s.persist(st, []string{fmt.Sprintf("%s%s「%s」。%s", player.Name, mode, spell.Name, detail)})
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
	*player = rules.ChangeResource(*player, resourceID, delta)
	return s.persist(st, nil)
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
