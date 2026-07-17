// Typed fetch client for the Go backend. Every mutating endpoint returns the
// full server View (typed as Campaign) so callers can setCampaign(view)
// wholesale; the client never recomputes rules.
import type {
  AbilityKey,
  AbilityScores,
  Campaign,
  CampaignSummary,
  CharacterSpell,
  Choice,
  PlayerId,
  RequiredCheck,
  RestType,
} from './types';

// ApiError carries the HTTP status and the parsed error body so callers can
// branch on 404 (missing campaign), 409 (conflict / needsConsent) and 422
// (mechanical action rejection with actionIssues).
export class ApiError extends Error {
  status: number;
  data: Record<string, unknown>;

  constructor(message: string, status: number, data: Record<string, unknown>) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.data = data;
  }
}

async function apiFetch<T>(url: string, init: RequestInit = {}): Promise<T> {
  const headers = init.body != null
    ? { 'content-type': 'application/json', ...(init.headers as Record<string, string> | undefined) }
    : init.headers;
  const response = await fetch(url, { ...init, headers });
  const data: Record<string, unknown> = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message = typeof data.error === 'string' && data.error ? data.error : `伺服器錯誤（HTTP ${response.status}）`;
    throw new ApiError(message, response.status, data);
  }
  return data as T;
}

function campaignPath(id: string, suffix = '') {
  return `/api/campaign/${encodeURIComponent(id)}${suffix}`;
}

function playerPath(id: string, playerId: PlayerId | string, suffix = '') {
  return campaignPath(id, `/players/${encodeURIComponent(playerId)}${suffix}`);
}

// ---------------------------------------------------------------------------
// Campaign lifecycle

export interface PlayerSeed {
  name: string;
  className: string;
  level?: number;
  species?: string;
  background?: string;
  abilities?: AbilityScores;
}

export interface CreateCampaignParams {
  id?: string;
  storyId: string;
  title: string;
  chapter: string;
  scene: string;
  objective: string;
  objectiveContext: string;
  stakes: string;
  opening: string;
  players: PlayerSeed[];
  settings?: Record<string, unknown>;
}

export function listCampaigns(): Promise<{ campaigns: CampaignSummary[] }> {
  return apiFetch('/api/campaigns');
}

export function createCampaign(params: CreateCampaignParams): Promise<Campaign> {
  return apiFetch('/api/campaigns', { method: 'POST', body: JSON.stringify(params) });
}

// raw is the export JSON text ({format,version,campaign} or a bare campaign).
export function importCampaign(raw: string, overwrite: boolean): Promise<Campaign> {
  return apiFetch(`/api/campaigns/import${overwrite ? '?overwrite=true' : ''}`, { method: 'POST', body: raw });
}

export function getCampaign(id: string): Promise<Campaign> {
  return apiFetch(campaignPath(id));
}

export function deleteCampaign(id: string): Promise<{ ok: boolean }> {
  return apiFetch(campaignPath(id), { method: 'DELETE' });
}

// URL for an <a download> — the server answers with content-disposition.
export function exportUrl(id: string): string {
  return campaignPath(id, '/export');
}

export function patchSettings(id: string, patch: Record<string, unknown>): Promise<Campaign> {
  return apiFetch(campaignPath(id, '/settings'), { method: 'PATCH', body: JSON.stringify(patch) });
}

// ---------------------------------------------------------------------------
// Rules catalog (static per server build; cached in a module variable)

export interface RulesCatalog {
  classNames: string[];
  abilityLabels: Record<AbilityKey, string>;
  spells: CharacterSpell[];
}

let catalogPromise: Promise<RulesCatalog> | null = null;

export function getCatalog(): Promise<RulesCatalog> {
  if (!catalogPromise) {
    catalogPromise = apiFetch<RulesCatalog>('/api/rules/catalog').catch((caught) => {
      catalogPromise = null; // allow a retry after a failed fetch
      throw caught;
    });
  }
  return catalogPromise;
}

// ---------------------------------------------------------------------------
// Character actions

export interface CastSpellParams {
  spellId: string;
  asRitual: boolean;
  targetId: string;
  attackTotal?: number;
}

// Either the updated view, or a pending attack-roll request (nothing mutated
// yet): the caller shows the DiceTray and resubmits with attackTotal.
export interface CastResult {
  view?: Campaign;
  needsAttackRoll?: RequiredCheck;
}

export function castSpell(id: string, playerId: PlayerId, params: CastSpellParams): Promise<CastResult> {
  return apiFetch(playerPath(id, playerId, '/cast'), { method: 'POST', body: JSON.stringify(params) });
}

export function rest(id: string, playerId: PlayerId, type: RestType): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/rest'), { method: 'POST', body: JSON.stringify({ type }) });
}

export function levelUp(id: string, playerId: PlayerId, className: string): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/level-up'), { method: 'POST', body: JSON.stringify({ className }) });
}

export function spendAbilityPoint(id: string, playerId: PlayerId, ability: AbilityKey): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/ability-point'), { method: 'POST', body: JSON.stringify({ ability }) });
}

export function setPreparedSpells(id: string, playerId: PlayerId, spellIds: string[]): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/prepared-spells'), { method: 'POST', body: JSON.stringify({ spellIds }) });
}

export function changeResource(id: string, playerId: PlayerId, resourceId: string, delta: number): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/resource'), { method: 'POST', body: JSON.stringify({ resourceId, delta }) });
}

export interface PlayerPatch {
  species?: string;
  background?: string;
  abilities?: AbilityScores;
  appearance?: string;
  portraitUrl?: string;
}

export function patchPlayer(id: string, playerId: PlayerId, patch: PlayerPatch): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId), { method: 'PATCH', body: JSON.stringify(patch) });
}

export function submitAction(id: string, playerId: PlayerId, text: string): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/action'), { method: 'POST', body: JSON.stringify({ text }) });
}

export function unlockAction(id: string, playerId: PlayerId | string): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/action'), { method: 'DELETE' });
}

// Revive a downed (0 HP) character: costs the rescuer's combat action in
// combat, or 1 exploration action point outside combat.
export function revive(id: string, targetId: PlayerId | string, rescuerId: PlayerId | string): Promise<Campaign> {
  return apiFetch(playerPath(id, targetId, '/revive'), { method: 'POST', body: JSON.stringify({ rescuerId }) });
}

// ---------------------------------------------------------------------------
// Equipment merchant

export interface ShopItem {
  id: string;
  name: string;
  kind: 'weapon' | 'armor' | 'gear' | 'potion';
  price: number;
  note: string;
}

export function shopCatalog(): Promise<{ items: ShopItem[] }> {
  return apiFetch('/api/shop/catalog');
}

export function buyItem(id: string, playerId: PlayerId | string, itemId: string): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/buy'), { method: 'POST', body: JSON.stringify({ itemId }) });
}

export function sellItem(id: string, playerId: PlayerId | string, itemName: string): Promise<Campaign> {
  return apiFetch(playerPath(id, playerId, '/sell'), { method: 'POST', body: JSON.stringify({ itemName }) });
}

// ---------------------------------------------------------------------------
// Combat

export interface EnemySpec {
  name: string;
  ac: number;
  hp: number;
  initiativeBonus: number;
  attackBonus: number;
  damage: string;
  damageType: string;
}

export interface AttackResolution {
  attackRoll: number;
  total: number;
  hit: boolean;
  critical: boolean;
  damage: number;
  text: string;
}

export interface CombatConclusion {
  outcome: 'victory' | 'defeat' | 'withdrawal';
  summary: string;
  /** Party wipe where the players chose to end the story: DM writes a final chapter. */
  final?: boolean;
}

export function combatStart(id: string, enemies: EnemySpec[]): Promise<Campaign> {
  return apiFetch(campaignPath(id, '/combat/start'), { method: 'POST', body: JSON.stringify({ enemies }) });
}

export function combatAttack(id: string, params: { attackId: string; targetId: string }): Promise<{ view: Campaign; resolution: AttackResolution }> {
  return apiFetch(campaignPath(id, '/combat/attack'), { method: 'POST', body: JSON.stringify(params) });
}

export function combatEndTurn(id: string): Promise<{ view: Campaign; enemyTurnPending: boolean }> {
  return apiFetch(campaignPath(id, '/combat/end-turn'), { method: 'POST' });
}

export function combatEnemyTurn(id: string): Promise<{ view: Campaign; resolution: AttackResolution; intent: string; fallback: boolean; enemyTurnPending: boolean }> {
  return apiFetch(campaignPath(id, '/combat/enemy-turn'), { method: 'POST' });
}

export function combatConclude(id: string): Promise<{ view: Campaign; conclusion: CombatConclusion }> {
  return apiFetch(campaignPath(id, '/combat/conclude'), { method: 'POST' });
}

export function combatRetry(id: string): Promise<Campaign> {
  return apiFetch(campaignPath(id, '/combat/retry'), { method: 'POST' });
}

// ---------------------------------------------------------------------------
// DM turn

export interface DmIntent {
  type: 'spell';
  spellId: string;
  targetId: string;
  asRitual: boolean;
}

export interface DmTurnRequest {
  campaignId: string;
  model?: string;
  effort?: string;
  dmProvider?: string;
  actions?: Array<{ playerId: PlayerId; text: string }>;
  intents?: Partial<Record<PlayerId, DmIntent>>;
  checkRoll?: { natural: number };
  combatConclusion?: CombatConclusion;
}

export interface DmCheck {
  character: string;
  playerId?: string;
  ability: string;
  skill: string;
  dc: number;
  reason: string;
}

export interface ActionIssue {
  playerId: PlayerId;
  message: string;
}

export interface SceneSlotPayload {
  id: string;
  scene: string;
  imagePrompt: string;
  createdAt: number;
}

export interface DmTurnResponse {
  view: Campaign;
  text: string;
  choices: Choice[];
  requiresCheck: boolean;
  check: DmCheck | null;
  privateMessages: Array<{ playerId: PlayerId; text: string }>;
  actionIssues: ActionIssue[];
  model: string;
  sceneSlot?: SceneSlotPayload;
}

export function dmTurn(body: DmTurnRequest, signal?: AbortSignal): Promise<DmTurnResponse> {
  return apiFetch('/api/dm', { method: 'POST', body: JSON.stringify(body), signal });
}

export function reviseStory(
  campaignId: string,
  body: { note: string; model?: string; effort?: string; dmProvider?: string },
  signal?: AbortSignal,
): Promise<{ view: Campaign; text: string; model: string }> {
  return apiFetch(campaignPath(campaignId, '/revise-story'), {
    method: 'POST',
    body: JSON.stringify(body),
    signal,
  });
}
