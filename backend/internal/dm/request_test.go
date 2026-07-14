package dm

import (
	"encoding/json"
	"strings"
	"testing"

	"dndduet/internal/apperr"
)

// decodeBody marshals a Go value and decodes it back into a map[string]any,
// reproducing the float64/string typing the HTTP layer feeds BuildDMRequest.
func decodeBody(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func player(id, name, className string) map[string]any {
	return map[string]any{
		"id": id, "name": name, "className": className, "subclass": "測試子職業", "level": 3,
		"hp": 20, "maxHp": 24, "ac": 15, "speed": 30, "proficiencyBonus": 2,
		"abilities": map[string]any{"str": 10, "dex": 14, "con": 14, "int": 17, "wis": 12, "cha": 8},
		"skills":    []any{map[string]any{"name": "奧秘", "bonus": 5, "proficient": true}},
		"attacks":   []any{map[string]any{"name": "長棍", "attackBonus": 2, "damage": "1d6", "damageType": "鈍擊"}},
		"resources": []any{map[string]any{"name": "奧術回復", "current": 1, "max": 1}},
		"features":  []any{map[string]any{"name": "儀式專家"}},
		"spellcasting": map[string]any{
			"slots":  []any{map[string]any{"level": 1, "current": 4, "max": 4}},
			"spells": []any{map[string]any{"name": "光亮術", "level": 0, "prepared": true}, map[string]any{"name": "魔法飛彈", "level": 1, "prepared": true}},
		},
	}
}

func build(t *testing.T, body any) (string, error) {
	t.Helper()
	prompt, _, err := BuildDMRequest(decodeBody(t, body))
	return prompt, err
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected prompt to contain %q", sub)
	}
}

func mustNotContain(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("expected prompt NOT to contain %q", sub)
	}
}

func TestRejectsTurnWhenMemberHasNotSubmitted(t *testing.T) {
	_, err := build(t, map[string]any{
		"players": []any{player("player1", "甲", "法師"), player("player2", "乙", "戰士")},
		"actions": []any{map[string]any{"playerId": "player1", "text": "施放光亮術"}},
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := apperr.StatusOf(err, 0); got != 400 {
		t.Errorf("status = %d, want 400", got)
	}
	if !strings.Contains(err.Error(), "每位玩家") {
		t.Errorf("message = %q, want it to mention 每位玩家", err.Error())
	}
}

func TestIncludesCompleteRulesStateAndActions(t *testing.T) {
	prompt, err := build(t, map[string]any{
		"campaign": map[string]any{"title": "測試戰役", "scene": "石門", "round": 2},
		"players":  []any{player("player1", "甲", "法師"), player("player2", "乙", "戰士")},
		"actions":  []any{map[string]any{"playerId": "player1", "text": "施放光亮術"}, map[string]any{"playerId": "player2", "text": "推開石門"}},
		"history":  []any{map[string]any{"speaker": "dm", "text": "門後傳來低語。"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, prompt, "SRD 5.2.1")
	mustContain(t, prompt, "智力17")
	mustContain(t, prompt, "奧術回復 1/1")
	mustContain(t, prompt, "魔法飛彈(1環)")
	mustContain(t, prompt, "本輪宣告：推開石門")
	mustContain(t, prompt, "必須在 actionIssues 駁回並給出具體規則理由")
}

func TestSupportsLegacyActionObject(t *testing.T) {
	prompt, err := build(t, map[string]any{
		"players": []any{player("player1", "甲", "法師")},
		"actions": map[string]any{"player1": "檢查符文"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, prompt, "本輪宣告：檢查符文")
}

func TestLabelsPrivateHistoryAndActiveCombat(t *testing.T) {
	prompt, err := build(t, map[string]any{
		"campaign": map[string]any{"title": "測試戰役", "scene": "石門", "round": 3},
		"players":  []any{player("player1", "甲", "法師")},
		"actions":  []any{map[string]any{"playerId": "player1", "text": "攻擊哥布林"}},
		"history":  []any{map[string]any{"speaker": "dm", "audience": "player1", "text": "你看見暗號。"}},
		"combat":   map[string]any{"active": true, "round": 2, "combatants": []any{map[string]any{"name": "哥布林", "hp": 4, "maxHp": 12, "ac": 13, "initiative": 15}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, prompt, "僅 player1 可見")
	mustContain(t, prompt, "戰鬥第 2 輪")
	mustContain(t, prompt, "哥布林 HP 4/12 AC 13 先攻 15")
}

func TestContinuesFromRequiredCheck(t *testing.T) {
	prompt, err := build(t, map[string]any{
		"campaign":   map[string]any{"title": "測試戰役", "scene": "石門", "round": 3},
		"players":    []any{player("player1", "甲", "法師"), player("player2", "乙", "戰士")},
		"actions":    []any{},
		"resolution": map[string]any{"character": "乙", "ability": "力量", "skill": "運動", "reason": "推開卡死的石門", "dc": 14, "natural": 12, "modifier": 3, "total": 15, "success": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, prompt, "不是新的玩家行動")
	mustContain(t, prompt, "總值 15，DC 14，結果為成功")
	mustContain(t, prompt, "不可插入、假設或要求任何新的玩家行動")
	mustNotContain(t, prompt, "本輪宣告：")
}

func TestCombatConclusionOutcomeBranches(t *testing.T) {
	cases := []struct{ outcome, want string }{
		{"victory", "戰鬥結果：隊伍勝利"},
		{"defeat", "戰鬥結果：隊伍戰敗"},
		{"withdrawal", "戰鬥結果：戰鬥中止或撤退"},
		{"nonsense", "戰鬥結果：戰鬥中止或撤退"}, // invalid outcome falls back to withdrawal
	}
	for _, c := range cases {
		prompt, err := build(t, map[string]any{
			"players":          []any{player("player1", "甲", "法師")},
			"actions":          []any{},
			"combatConclusion": map[string]any{"outcome": c.outcome, "summary": "殘局"},
		})
		if err != nil {
			t.Fatalf("%s: %v", c.outcome, err)
		}
		mustContain(t, prompt, c.want)
	}
}

func TestContinuesFromCombatConclusion(t *testing.T) {
	prompt, err := build(t, map[string]any{
		"campaign":         map[string]any{"title": "測試戰役", "scene": "石門", "round": 4},
		"players":          []any{player("player1", "甲", "法師"), player("player2", "乙", "戰士")},
		"actions":          []any{},
		"combatConclusion": map[string]any{"outcome": "victory", "summary": "哥布林全數倒下；甲剩餘 8 HP，乙剩餘 17 HP。"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, prompt, "這不是新的玩家行動")
	mustContain(t, prompt, "戰鬥結果：隊伍勝利")
	mustContain(t, prompt, "直接敘述戰鬥結束後的現場")
	mustNotContain(t, prompt, "本輪宣告：")
}
