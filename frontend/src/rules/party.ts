import type { PlayerCharacter, PlayerId } from '../types';

export function areAllActionsReady(players: PlayerCharacter[], pending: Partial<Record<PlayerId, string>>) {
  return players.length > 0 && players.every((player) => Boolean(pending[player.id]?.trim()));
}

export function createActionPayload(players: PlayerCharacter[], pending: Partial<Record<PlayerId, string>>) {
  return players.map((player) => ({ playerId: player.id, text: pending[player.id]?.trim() || '' }));
}
