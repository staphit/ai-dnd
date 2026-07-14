import { describe, expect, it } from 'vitest';
import { createLevel3Character } from './characters';
import { areAllActionsReady, createActionPayload } from './party';

describe('dynamic party turns', () => {
  const players = [
    createLevel3Character('player1', '甲', '戰士'),
    createLevel3Character('player2', '乙', '法師'),
    createLevel3Character('player3', '丙', '盜賊'),
  ];

  it('waits until every party member has submitted a non-empty action', () => {
    expect(areAllActionsReady(players, { player1: '守住門口', player2: '施放光亮術' })).toBe(false);
    expect(areAllActionsReady(players, { player1: '守住門口', player2: '施放光亮術', player3: '搜索陷阱' })).toBe(true);
    expect(areAllActionsReady(players, { player1: '守住門口', player2: ' ', player3: '搜索陷阱' })).toBe(false);
  });

  it('creates a stable action payload in party order', () => {
    expect(createActionPayload(players, { player3: '搜索陷阱', player1: '守住門口', player2: '施放光亮術' })).toEqual([
      { playerId: 'player1', text: '守住門口' },
      { playerId: 'player2', text: '施放光亮術' },
      { playerId: 'player3', text: '搜索陷阱' },
    ]);
  });
});
