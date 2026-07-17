package game

import (
	"fmt"
	"strings"

	"dndduet/internal/dm"
	"dndduet/internal/rules"
)

// BuildRulesDossier renders the per-story rules file for delta-mode turns:
// the complete DM ruleset plus each character's static sheet (species,
// background, features, full spell list, equipment) — everything the per-turn
// digest deliberately drops. Codex reads this file in its sandbox, so the
// per-turn prompt carries only a mini-preamble and the pointer.
func (s *Service) BuildRulesDossier(id string) (string, error) {
	players, err := s.loadCharacters(id)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("# DM 守則\n\n")
	b.WriteString(dm.FullPreambleText())
	b.WriteString("\n\n# 隊伍靜態資料\n\n以下為各角色的完整靜態檔案；即時數值（HP、法術位、資源、狀態）以每回合提示中的能力摘要為準。\n\n")
	for _, p := range players {
		writeDossierPlayer(&b, p)
	}
	return b.String(), nil
}

func writeDossierPlayer(b *strings.Builder, c rules.Character) {
	classLine := fmt.Sprintf("%s%d", c.ClassName, c.Level)
	if strings.TrimSpace(c.Subclass) != "" {
		classLine += "／" + c.Subclass
	}
	fmt.Fprintf(b, "## %s %s（%s）\n", c.ID, c.Name, classLine)
	fmt.Fprintf(b, "- 種族 %s；背景 %s；速度 %d；熟練 +%d\n",
		orText(c.Species, "未設定"), orText(c.Background, "未設定"), c.Speed, c.ProficiencyBonus)

	abilities := make([]string, 0, len(rules.AbilityKeys))
	for _, key := range rules.AbilityKeys {
		abilities = append(abilities, fmt.Sprintf("%s%d", rules.AbilityLabels[key], c.Abilities.Get(key)))
	}
	fmt.Fprintf(b, "- 能力值：%s\n", strings.Join(abilities, "、"))

	var skills []string
	for _, sk := range c.Skills {
		if sk.Proficient {
			skills = append(skills, fmt.Sprintf("%s%+d", sk.Name, sk.Bonus))
		}
	}
	if len(skills) > 0 {
		fmt.Fprintf(b, "- 熟練技能：%s\n", strings.Join(skills, "、"))
	}

	var attacks []string
	for _, a := range c.Attacks {
		attacks = append(attacks, fmt.Sprintf("%s 命中%+d／%s%s", a.Name, a.AttackBonus, a.Damage, a.DamageType))
	}
	if len(attacks) > 0 {
		fmt.Fprintf(b, "- 攻擊：%s\n", strings.Join(attacks, "；"))
	}

	var features []string
	for _, f := range c.Features {
		features = append(features, f.Name)
	}
	if len(features) > 0 {
		fmt.Fprintf(b, "- 職業能力：%s\n", strings.Join(features, "、"))
	}

	if c.Spellcasting != nil && len(c.Spellcasting.Spells) > 0 {
		var spells []string
		for _, sp := range c.Spellcasting.Spells {
			tag := ""
			switch {
			case sp.Level == 0:
				tag = "（戲法）"
			case sp.Prepared || sp.AlwaysPrepared:
				tag = fmt.Sprintf("（%d環·已準備）", sp.Level)
			default:
				tag = fmt.Sprintf("（%d環·未準備）", sp.Level)
			}
			spells = append(spells, sp.Name+tag)
		}
		fmt.Fprintf(b, "- 法術（攻擊%+d／豁免DC %d）：%s\n", c.Spellcasting.AttackBonus, c.Spellcasting.SaveDC, strings.Join(spells, "、"))
	}

	if len(c.Equipment) > 0 {
		fmt.Fprintf(b, "- 裝備：%s\n", strings.Join(c.Equipment, "、"))
	}
	b.WriteString("\n")
}

func orText(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
