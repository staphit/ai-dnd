import type {
  Campaign,
  CampaignSettings,
  ForgeSettings,
  PlayerCharacter,
  PlayerId,
  StoryEntry,
} from '../types';
import type { CombatConclusion } from '../api';

// One DM continuation request. Actions come from the server pending lock;
// checkRoll carries only the local d20 and combatConclusion comes from the
// server-authoritative combat endpoint.
export interface AdvanceInput {
  actions?: Array<{ playerId: PlayerId; text: string }>;
  checkRoll?: { natural: number; success?: boolean };
  combatConclusion?: CombatConclusion;
}

export interface ContextualTip {
  id: string;
  title: string;
  text: string;
  page?: import('../types').Page;
}

export function timeLabel() {
  return new Intl.DateTimeFormat('zh-TW', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date());
}

export function errorMessage(caught: unknown) {
  return caught instanceof Error ? caught.message : String(caught);
}

export function settingsOf(campaign: Campaign): CampaignSettings {
  return (campaign.settings || {}) as CampaignSettings;
}

export function forgeRequest(settings?: ForgeSettings) {
  if (!settings?.Enabled) return undefined;
  return {
    enabled: true,
    positivePrompt: settings.PositivePrompt,
    negativePrompt: settings.NegativePrompt,
    steps: settings.Steps,
    cfgScale: settings.CFGScale,
    sampler: settings.Sampler,
    scheduler: settings.Scheduler,
    seed: settings.Seed,
    width: settings.Width,
    height: settings.Height,
  };
}

export function areAllActionsReady(view: Campaign) {
  return view.players.length > 0
    && view.players.every((player) => Boolean(view.pending[player.id]?.trim()));
}

export function actionsFrom(view: Campaign): Array<{ playerId: PlayerId; text: string }> {
  return view.players.map((player) => ({
    playerId: player.id,
    text: view.pending[player.id]?.trim() || '',
  }));
}

export function storySpeakerLabel(entry: StoryEntry, players: PlayerCharacter[]) {
  if (entry.speaker === 'dm') {
    return entry.audience && entry.audience !== 'public'
      ? `地城主私訊 ${players.find((player) => player.id === entry.audience)?.name || entry.audience}`
      : '地城主';
  }
  if (entry.speaker === 'system') return '紀錄';
  return players.find((player) => player.id === entry.speaker)?.name || '冒險者';
}
