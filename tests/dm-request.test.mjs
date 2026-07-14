import assert from 'node:assert/strict';
import test from 'node:test';
import { buildDmRequest } from '../dm-request.mjs';

function player(id, name, className) {
  return {
    id, name, className, subclass: '測試子職業', level: 3,
    hp: 20, maxHp: 24, ac: 15, speed: 30, proficiencyBonus: 2,
    abilities: { str: 10, dex: 14, con: 14, int: 17, wis: 12, cha: 8 },
    skills: [{ name: '奧秘', bonus: 5, proficient: true }],
    attacks: [{ name: '長棍', attackBonus: 2, damage: '1d6', damageType: '鈍擊' }],
    resources: [{ name: '奧術回復', current: 1, max: 1 }],
    features: [{ name: '儀式專家' }],
    spellcasting: {
      slots: [{ level: 1, current: 4, max: 4 }],
      spells: [{ name: '光亮術', level: 0, prepared: true }, { name: '魔法飛彈', level: 1, prepared: true }],
    },
  };
}

test('rejects a turn when one party member has not submitted', () => {
  assert.throws(
    () => buildDmRequest({
      players: [player('player1', '甲', '法師'), player('player2', '乙', '戰士')],
      actions: [{ playerId: 'player1', text: '施放光亮術' }],
    }),
    (error) => error.statusCode === 400 && /每位玩家/.test(error.message),
  );
});

test('includes complete rules state and all actions in the DM prompt', () => {
  const result = buildDmRequest({
    campaign: { title: '測試戰役', scene: '石門', round: 2 },
    players: [player('player1', '甲', '法師'), player('player2', '乙', '戰士')],
    actions: [{ playerId: 'player1', text: '施放光亮術' }, { playerId: 'player2', text: '推開石門' }],
    history: [{ speaker: 'dm', text: '門後傳來低語。' }],
  });
  assert.match(result.prompt, /SRD 5\.2\.1/);
  assert.match(result.prompt, /智力17/);
  assert.match(result.prompt, /奧術回復 1\/1/);
  assert.match(result.prompt, /魔法飛彈\(1環\)/);
  assert.match(result.prompt, /本輪宣告：推開石門/);
  assert.match(result.prompt, /必須在 actionIssues 駁回並給出具體規則理由/);
});

test('supports the legacy action object without calling the model', () => {
  const result = buildDmRequest({
    players: [player('player1', '甲', '法師')],
    actions: { player1: '檢查符文' },
  });
  assert.match(result.prompt, /本輪宣告：檢查符文/);
});

test('labels private history and includes active combat state', () => {
  const result = buildDmRequest({
    campaign: { title: '測試戰役', scene: '石門', round: 3 },
    players: [player('player1', '甲', '法師')],
    actions: [{ playerId: 'player1', text: '攻擊哥布林' }],
    history: [{ speaker: 'dm', audience: 'player1', text: '你看見暗號。' }],
    combat: { active: true, round: 2, combatants: [{ name: '哥布林', hp: 4, maxHp: 12, ac: 13, initiative: 15 }] },
  });
  assert.match(result.prompt, /僅 player1 可見/);
  assert.match(result.prompt, /戰鬥第 2 輪/);
  assert.match(result.prompt, /哥布林 HP 4\/12 AC 13 先攻 15/);
});

test('continues directly from a required check without inventing player actions', () => {
  const result = buildDmRequest({
    campaign: { title: '測試戰役', scene: '石門', round: 3 },
    players: [player('player1', '甲', '法師'), player('player2', '乙', '戰士')],
    actions: [],
    resolution: { character: '乙', ability: '力量', skill: '運動', reason: '推開卡死的石門', dc: 14, natural: 12, modifier: 3, total: 15, success: true },
  });
  assert.match(result.prompt, /不是新的玩家行動/);
  assert.match(result.prompt, /總值 15，DC 14，結果為成功/);
  assert.match(result.prompt, /不可插入、假設或要求任何新的玩家行動/);
  assert.doesNotMatch(result.prompt, /本輪宣告：/);
});

test('continues directly from combat conclusion without requiring player actions', () => {
  const result = buildDmRequest({
    campaign: { title: '測試戰役', scene: '石門', round: 4 },
    players: [player('player1', '甲', '法師'), player('player2', '乙', '戰士')],
    actions: [],
    combatConclusion: { outcome: 'victory', summary: '哥布林全數倒下；甲剩餘 8 HP，乙剩餘 17 HP。' },
  });
  assert.match(result.prompt, /這不是新的玩家行動/);
  assert.match(result.prompt, /戰鬥結果：隊伍勝利/);
  assert.match(result.prompt, /直接敘述戰鬥結束後的現場/);
  assert.doesNotMatch(result.prompt, /本輪宣告：/);
});
