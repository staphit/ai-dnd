package rules

// Ported 1:1 from frontend/src/rules/party.test.ts ("dynamic party turns"
// suite); every vitest assertion is preserved.

import (
	"reflect"
	"testing"
)

// partyTestPlayers rebuilds the shared describe-level fixture for each test.
func partyTestPlayers() []Character {
	return []Character{
		CreateLevel3Character("player1", "甲", "戰士"),
		CreateLevel3Character("player2", "乙", "法師"),
		CreateLevel3Character("player3", "丙", "聖武士"),
	}
}

// TestAreAllActionsReady ports "waits until every party member has submitted a
// non-empty action".
func TestAreAllActionsReady(t *testing.T) {
	players := partyTestPlayers()
	if AreAllActionsReady(players, map[string]string{"player1": "守住門口", "player2": "施放光亮術"}) {
		t.Error("two of three actions submitted: got true, want false")
	}
	if !AreAllActionsReady(players, map[string]string{"player1": "守住門口", "player2": "施放光亮術", "player3": "搜索陷阱"}) {
		t.Error("all actions submitted: got false, want true")
	}
	if AreAllActionsReady(players, map[string]string{"player1": "守住門口", "player2": " ", "player3": "搜索陷阱"}) {
		t.Error("whitespace-only action submitted: got true, want false")
	}
}

// TestCreateActionPayload ports "creates a stable action payload in party
// order".
func TestCreateActionPayload(t *testing.T) {
	players := partyTestPlayers()
	payload := CreateActionPayload(players, map[string]string{"player3": "搜索陷阱", "player1": "守住門口", "player2": "施放光亮術"})
	want := []Action{
		{PlayerID: "player1", Text: "守住門口"},
		{PlayerID: "player2", Text: "施放光亮術"},
		{PlayerID: "player3", Text: "搜索陷阱"},
	}
	if !reflect.DeepEqual(payload, want) {
		t.Errorf("payload = %+v, want %+v", payload, want)
	}
}
