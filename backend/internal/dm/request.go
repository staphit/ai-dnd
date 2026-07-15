package dm

import (
	"strings"

	"dndduet/internal/apperr"
)

// abilityOrder preserves the insertion order of the original abilityLabels
// object so the ability summary reads 力量、敏捷、體質、智力、感知、魅力.
var abilityOrder = []struct{ Key, Label string }{
	{"str", "力量"},
	{"dex", "敏捷"},
	{"con", "體質"},
	{"int", "智力"},
	{"wis", "感知"},
	{"cha", "魅力"},
}

// SanitizedPlayer is the condensed, DM-facing view of a character.
type SanitizedPlayer struct {
	ID        string
	Name      string
	ClassName string
	Subclass  string
	Summary   string
}

func arrTake(arr []any, n int) []any {
	if n < len(arr) {
		return arr[:n]
	}
	return arr
}

func arrTakeLast(arr []any, n int) []any {
	if n < len(arr) {
		return arr[len(arr)-n:]
	}
	return arr
}

func sanitizePlayer(player any, index int) SanitizedPlayer {
	abilityParts := make([]string, 0, len(abilityOrder))
	for _, a := range abilityOrder {
		abilityParts = append(abilityParts, a.Label+numStr(get(player, "abilities", a.Key), 10))
	}
	abilities := strings.Join(abilityParts, "、")

	skills := ""
	if arr, ok := asSlice(get(player, "skills")); ok {
		var parts []string
		for _, s := range arr {
			if !truthy(get(s, "proficient")) {
				continue
			}
			bonus := toNum(get(s, "bonus"))
			sign := ""
			if bonus >= 0 {
				sign = "+"
			}
			parts = append(parts, jsSlice(strOf(get(s, "name")), 40)+sign+numToStr(bonus))
		}
		skills = strings.Join(parts, "、")
	}

	attacks := ""
	if arr, ok := asSlice(get(player, "attacks")); ok {
		var parts []string
		for _, a := range arrTake(arr, 8) {
			bonus := toNum(get(a, "attackBonus"))
			sign := ""
			if bonus >= 0 {
				sign = "+"
			}
			parts = append(parts, jsSlice(strOf(get(a, "name")), 60)+" 命中"+sign+numToStr(bonus)+"／"+jsSlice(strOf(get(a, "damage")), 30)+jsSlice(strOr(get(a, "damageType"), ""), 20))
		}
		attacks = strings.Join(parts, "；")
	}

	resources := ""
	if arr, ok := asSlice(get(player, "resources")); ok {
		var parts []string
		for _, r := range arrTake(arr, 10) {
			parts = append(parts, jsSlice(strOf(get(r, "name")), 60)+" "+numPlain(get(r, "current"))+"/"+numPlain(get(r, "max")))
		}
		resources = strings.Join(parts, "、")
	}

	slots := ""
	if arr, ok := asSlice(get(player, "spellcasting", "slots")); ok {
		var parts []string
		for _, sl := range arr {
			parts = append(parts, numPlain(get(sl, "level"))+"環 "+numPlain(get(sl, "current"))+"/"+numPlain(get(sl, "max")))
		}
		slots = strings.Join(parts, "、")
	}

	spells := ""
	if arr, ok := asSlice(get(player, "spellcasting", "spells")); ok {
		filtered := make([]any, 0, len(arr))
		for _, sp := range arr {
			if toNum(get(sp, "level")) == 0 || truthy(get(sp, "prepared")) || truthy(get(sp, "alwaysPrepared")) {
				filtered = append(filtered, sp)
			}
		}
		var parts []string
		for _, sp := range arrTake(filtered, 30) {
			suffix := "(戲法)"
			if toNum(get(sp, "level")) != 0 {
				suffix = "(" + numPlain(get(sp, "level")) + "環)"
			}
			parts = append(parts, jsSlice(strOf(get(sp, "name")), 60)+suffix)
		}
		spells = strings.Join(parts, "、")
	}

	features := ""
	if arr, ok := asSlice(get(player, "features")); ok {
		var parts []string
		for _, f := range arrTake(arr, 12) {
			parts = append(parts, jsSlice(strOf(get(f, "name")), 60))
		}
		features = strings.Join(parts, "、")
	}

	classLevels := ""
	if arr, ok := asSlice(get(player, "classLevels")); ok {
		var parts []string
		for _, c := range arr {
			parts = append(parts, jsSlice(strOf(get(c, "className")), 40)+numPlain(get(c, "level")))
		}
		classLevels = strings.Join(parts, "／")
	}

	classLevelSuffix := ""
	if classLevels != "" {
		classLevelSuffix = "（" + classLevels + "）"
	}

	lines := []string{
		"等級 " + numStr(get(player, "level"), 3) + classLevelSuffix + "；種族 " + jsSlice(strOr(get(player, "species"), "未設定"), 60) + "；背景 " + jsSlice(strOr(get(player, "background"), "未設定"), 60) + "；" + abilities,
		"HP " + numStr(get(player, "hp"), 0) + "/" + numStr(get(player, "maxHp"), 0) + "；AC " + numStr(get(player, "ac"), 10) + "；速度 " + numStr(get(player, "speed"), 30) + "；熟練 +" + numStr(get(player, "proficiencyBonus"), 2),
	}
	if skills != "" {
		lines = append(lines, "熟練技能："+skills)
	}
	if attacks != "" {
		lines = append(lines, "攻擊："+attacks)
	}
	if resources != "" {
		lines = append(lines, "職業資源："+resources)
	}
	if features != "" {
		lines = append(lines, "職業能力："+features)
	}
	if slots != "" {
		lines = append(lines, "法術位："+slots)
	}
	if spells != "" {
		lines = append(lines, "可施放法術："+spells)
	}
	if truthy(get(player, "concentration")) {
		lines = append(lines, "目前專注："+jsSlice(strOf(get(player, "concentration")), 80))
	}

	return SanitizedPlayer{
		ID:        "player" + itoa(index+1),
		Name:      jsSlice(strings.TrimSpace(strOr(get(player, "name"), "玩家 "+itoa(index+1))), 100),
		ClassName: jsSlice(strings.TrimSpace(strOr(get(player, "className"), "冒險者")), 100),
		Subclass:  jsSlice(strings.TrimSpace(strOr(get(player, "subclass"), "")), 100),
		Summary:   strings.Join(lines, "\n"),
	}
}

func itoa(n int) string {
	// small non-negative integers only (player indices / counts)
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// BuildDMRequest builds the DM prompt from the untrusted request body, matching
// dm-request.mjs. It returns an apperr.Error with status 400 when the party is
// empty or (outside a continuation) a player has not submitted an action.
func BuildDMRequest(body map[string]any) (string, []SanitizedPlayer, error) {
	var players []SanitizedPlayer
	if arr, ok := asSlice(get(body, "players")); ok {
		for i, p := range arrTake(arr, 4) {
			players = append(players, sanitizePlayer(p, i))
		}
	}

	var resolution *resolutionData
	if m := asMap(get(body, "resolution")); m != nil {
		resolution = &resolutionData{
			character: jsSlice(strOr(m["character"], ""), 100),
			ability:   jsSlice(strOr(m["ability"], ""), 100),
			skill:     jsSlice(strOr(m["skill"], ""), 100),
			reason:    jsSlice(strOr(m["reason"], ""), 500),
			dc:        clampNum(numOr(m["dc"], 10), 5, 30),
			natural:   clampNum(numOr(m["natural"], 1), 1, 20),
			modifier:  clampNum(numOr(m["modifier"], 0), -20, 30),
			total:     clampNum(numOr(m["total"], 0), -19, 50),
			success:   m["success"] == true,
		}
	}

	var combatConclusion *combatConclusionData
	if m := asMap(get(body, "combatConclusion")); m != nil {
		outcome := "withdrawal"
		if o, ok := m["outcome"].(string); ok && (o == "victory" || o == "defeat" || o == "withdrawal") {
			outcome = o
		}
		combatConclusion = &combatConclusionData{
			outcome: outcome,
			summary: jsSlice(strings.TrimSpace(strOr(m["summary"], "")), 3000),
		}
	}

	isContinuation := resolution != nil || combatConclusion != nil

	// Build the action map (last write wins, like `new Map(entries)`).
	actions := map[string]string{}
	if arr, ok := asSlice(get(body, "actions")); ok {
		for _, a := range arrTake(arr, 4) {
			pid := strOr(get(a, "playerId"), "")
			text := jsSlice(strings.TrimSpace(strOr(get(a, "text"), "")), 2000)
			actions[pid] = text
		}
	} else if m := asMap(get(body, "actions")); m != nil {
		for pid, text := range m {
			actions[pid] = jsSlice(strings.TrimSpace(strOr(text, "")), 2000)
		}
	}

	missingAction := false
	for _, p := range players {
		if actions[p.ID] == "" {
			missingAction = true
			break
		}
	}
	if len(players) < 1 || (!isContinuation && missingAction) {
		return "", nil, apperr.New(400, "需要隊伍中每位玩家的行動才能進行裁定。")
	}

	recent := ""
	if arr, ok := asSlice(get(body, "history")); ok {
		var parts []string
		for _, entry := range arrTakeLast(arr, 16) {
			audience := ""
			aud := get(entry, "audience")
			if truthy(aud) && aud != "public" {
				audience = "（僅 " + strOf(aud) + " 可見）"
			}
			parts = append(parts, strOf(get(entry, "speaker"))+audience+": "+jsSlice(strOf(get(entry, "text")), 1400))
		}
		recent = strings.Join(parts, "\n")
	}

	var playerBlocks []string
	for i, p := range players {
		subclass := p.Subclass
		if subclass == "" {
			subclass = "未選子職業"
		}
		playerBlocks = append(playerBlocks, "玩家 "+itoa(i+1)+"「"+p.Name+"」（"+p.ClassName+"／"+subclass+"）", p.Summary)
		if !isContinuation {
			playerBlocks = append(playerBlocks, "本輪宣告："+actions[p.ID])
		}
		playerBlocks = append(playerBlocks, "")
	}

	combat := "目前沒有進行中的戰鬥。"
	if truthy(get(body, "combat", "active")) {
		if arr, ok := asSlice(get(body, "combat", "combatants")); ok {
			var parts []string
			for _, e := range arr {
				parts = append(parts, jsSlice(strOf(get(e, "name")), 50)+" HP "+numPlain(get(e, "hp"))+"/"+numPlain(get(e, "maxHp"))+" AC "+numPlain(get(e, "ac"))+" 先攻 "+numPlain(get(e, "initiative")))
			}
			combat = "戰鬥第 " + numStr(get(body, "combat", "round"), 1) + " 輪：" + strings.Join(parts, "；")
		}
	}

	prevChoices := ""
	if raw, ok := asSlice(get(body, "campaign", "choices")); ok {
		var cs []string
		for _, c := range raw {
			if s, ok := c.(string); ok && strings.TrimSpace(s) != "" {
				cs = append(cs, strings.TrimSpace(s))
			}
		}
		prevChoices = strings.Join(cs, "；")
	}

	lines := []string{
		"規則版本：2024 第五版／SRD 5.2.1。角色卡快照與戰鬥追蹤器是本輪裁定的事實來源。",
		"戰役：" + jsSlice(strOr(get(body, "campaign", "title"), "灰燼王冠"), 180),
		"場景：" + jsSlice(strOr(get(body, "campaign", "scene"), "未知地點"), 240),
		"目前目標：" + jsSlice(strOr(get(body, "campaign", "objective"), "尚未確定"), 240),
		"任務背景：" + jsSlice(strOr(get(body, "campaign", "objectiveContext"), "尚未確定"), 600),
		"風險：" + jsSlice(strOr(get(body, "campaign", "stakes"), "尚未確定"), 300),
		"回合：" + numStr(get(body, "campaign", "round"), 1),
		"上一回合你提供的選項（玩家若照著宣告就必須接受，不可再以能力或資源理由駁回）：" + firstNonEmpty(jsSlice(prevChoices, 400), "（無）"),
		combat,
		"",
		"最近紀錄：",
		firstNonEmpty(recent, "這是冒險的開始。"),
		"",
		continuationHeader(isContinuation),
	}
	lines = append(lines, playerBlocks...)

	switch {
	case resolution != nil:
		naturalSign := ""
		if resolution.modifier >= 0 {
			naturalSign = "+"
		}
		successText := "失敗"
		if resolution.success {
			successText = "成功"
		}
		lines = append(lines,
			"這是上一輪玩家行動所觸發的必要檢定結果，不是新的玩家行動：",
			resolution.character+"進行"+resolution.ability+"（"+resolution.skill+"）檢定，原因："+resolution.reason+"。d20 骰面 "+numToStr(resolution.natural)+naturalSign+numToStr(resolution.modifier)+"，總值 "+numToStr(resolution.total)+"，DC "+numToStr(resolution.dc)+"，結果為"+successText+"。",
			"請直接接續敘述此成功或失敗造成的具體後果並推進場景。不可插入、假設或要求任何新的玩家行動；actionIssues 必須為空陣列。若後果又立即產生另一個有風險且不確定的檢定，才可建立新的結構化 check。",
		)
	case combatConclusion != nil:
		outcomeText := "戰鬥中止或撤退"
		switch combatConclusion.outcome {
		case "victory":
			outcomeText = "隊伍勝利"
		case "defeat":
			outcomeText = "隊伍戰敗"
		}
		lines = append(lines,
			"戰鬥追蹤器剛剛完成戰鬥，這不是新的玩家行動：",
			"戰鬥結果："+outcomeText+"。"+combatConclusion.summary,
			"請直接敘述戰鬥結束後的現場、存活者反應、立即後果與新局勢，並更新目標背景和風險。不可插入、假設或要求新的玩家行動；不可再次結算傷害或 XP；combat.starts 必須為 false、actionIssues 必須為空陣列。",
		)
	default:
		lines = append(lines, "請公平處理全隊 "+itoa(len(players))+" 個行動並推進場景。")
	}

	lines = append(lines,
		"若宣告需要已耗盡的資源、未準備的法術、缺少必要目標、超過行動次數，或角色不具備的能力，必須在 actionIssues 駁回並給出具體規則理由與修改方向；不可自行補回資源、改寫行動或讓故事先推進。",
		"若行動合理，讓每位角色的選擇產生可見回應。只有結果具有風險且不確定時才要求檢定。",
	)

	return strings.Join(lines, "\n"), players, nil
}

type resolutionData struct {
	character, ability, skill, reason string
	dc, natural, modifier, total      float64
	success                           bool
}

type combatConclusionData struct {
	outcome string
	summary string
}

func continuationHeader(isContinuation bool) string {
	if isContinuation {
		return "角色狀態："
	}
	return "角色狀態與本輪行動："
}

func firstNonEmpty(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
