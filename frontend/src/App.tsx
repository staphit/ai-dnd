import { lazy, Suspense, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { ArrowClockwise, CloudArrowUp, Compass, Lightbulb, LockKey, MapTrifold, Plugs, ShieldWarning, Sword, XCircle } from '@phosphor-icons/react';
import { initialCampaign, storyPresets } from './data';
import type { AbilityKey, AiStatus, Campaign, CampaignSettings, CampaignSummary, CharacterSpell, Choice, ForgeSettings, MessageAudience, Page, PlayerCharacter, PlayerId, RequiredCheck, RestType, SceneImage, StoryEntry } from './types';
import { Sidebar } from './components/Sidebar';
import { Topbar } from './components/Topbar';
import { StoryFeed } from './components/StoryFeed';
import { CharacterPanel } from './components/CharacterPanel';
import { DiceTray } from './components/DiceTray';
import { MagneticButton } from './components/MagneticButton';
import { SceneVisual } from './components/SceneVisual';
import { PartySetup } from './components/PartySetup';
import { CombatTracker } from './components/CombatTracker';
import { CampaignManager } from './components/CampaignManager';
import { SpellCastModal } from './components/SpellCastModal';
import { StoryRevisionPanel, type RevisionChatLine } from './components/StoryRevisionPanel';
import * as api from './api';
import { ApiError, type ActionIssue, type CombatConclusion } from './api';
import { getActiveCampaignId, readLegacyVault, setActiveCampaignId } from './campaign-storage';

const CharacterManager = lazy(() => import('./components/CharacterManager').then((module) => ({ default: module.CharacterManager })));
// DM 3D avatar is loaded inside StoryFeed (not duplicated above the dialogue).

// One DM continuation request. actions come from the server pending lock;
// checkRoll carries the local d20 (the server recomputes modifier/success);
// combatConclusion is the payload returned by /combat/conclude.
interface AdvanceInput {
  actions?: Array<{ playerId: PlayerId; text: string }>;
  checkRoll?: { natural: number; success?: boolean };
  combatConclusion?: CombatConclusion;
}

function now() {
  return new Intl.DateTimeFormat('zh-TW', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date());
}

// Local-only entries are used by the demo DM overlay; real story entries come
// from the server view.
function makeEntry(speaker: StoryEntry['speaker'], text: string, audience: MessageAudience = 'public'): StoryEntry {
  return { id: `${Date.now()}-${crypto.randomUUID()}`, speaker, text, time: now(), audience };
}

function message(caught: unknown) {
  return caught instanceof Error ? caught.message : String(caught);
}

function settingsOf(campaign: Campaign): CampaignSettings {
  return (campaign.settings || {}) as CampaignSettings;
}

function forgeRequest(settings?: ForgeSettings) {
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

function areAllActionsReady(view: Campaign) {
  return view.players.length > 0 && view.players.every((player) => Boolean(view.pending[player.id]?.trim()));
}

function actionsFrom(view: Campaign): Array<{ playerId: PlayerId; text: string }> {
  return view.players.map((player) => ({ playerId: player.id, text: view.pending[player.id]?.trim() || '' }));
}

export default function App() {
  const [campaign, setCampaignState] = useState<Campaign>(() => structuredClone(initialCampaign));
  const [campaigns, setCampaigns] = useState<CampaignSummary[]>([]);
  const [booting, setBooting] = useState(true);
  const [legacyCampaigns, setLegacyCampaigns] = useState<Campaign[]>([]);
  const [legacyImporting, setLegacyImporting] = useState(false);
  const [page, setPage] = useState<Page>('table');
  const [status, setStatus] = useState<AiStatus | null>(null);
  const [viewer, setViewer] = useState<MessageAudience>('public');
  const [demoMode, setDemoMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [imageLoading, setImageLoading] = useState(false);
  const [imageError, setImageError] = useState('');
  // True while a TTS narration clip is playing; drives the 3D DM's talking pose.
  const [dmSpeaking, setDmSpeaking] = useState(false);
  const [sceneImage, setSceneImage] = useState<SceneImage | null>(null);
  const [spellRoll, setSpellRoll] = useState<{ check: RequiredCheck; casterId: PlayerId; spell: CharacterSpell; asRitual: boolean; targetId: string } | null>(null);
  const [spellModal, setSpellModal] = useState<{ playerId: PlayerId; spell: CharacterSpell } | null>(null);
  const [revisionOpen, setRevisionOpen] = useState(false);
  const [revisionChat, setRevisionChat] = useState<RevisionChatLine[]>([]);
  const [revising, setRevising] = useState(false);
  const [pendingSceneSlotId, setPendingSceneSlotId] = useState('');
  const [codexConn, setCodexConn] = useState<{ alive: boolean; storyId: string } | null>(null);
  const [connecting, setConnecting] = useState(false);
  const advancingRef = useRef(false);
  const campaignRef = useRef(campaign);
  // Holds the arguments of a turn that failed before advancing the story
  // (consent gate or post-conclude narration), so it can be replayed without
  // the players re-entering anything.
  const retryTurnRef = useRef<AdvanceInput | null>(null);
  // Debounce box for settings PATCHes triggered by typed inputs.
  const settingsBoxRef = useRef<{ id: string; timer: number | null; patch: CampaignSettings }>({ id: '', timer: null, patch: {} });

  useEffect(() => { campaignRef.current = campaign; }, [campaign]);

  const settings = settingsOf(campaign);

  // setCampaign adopts a server view wholesale and keeps the active-id marker
  // plus the campaign list entry in sync.
  function setCampaign(view: Campaign) {
    setCampaignState(view);
    if (!view.id) return;
    setActiveCampaignId(view.id);
    setCampaigns((current) => [
      { id: view.id!, title: view.title, scene: view.scene, round: view.round, updatedAt: view.updatedAt || new Date().toISOString() },
      ...current.filter((entry) => entry.id !== view.id),
    ]);
  }

  // adoptCampaign switches to another campaign: resets per-campaign UI state.
  function adoptCampaign(view: Campaign) {
    setCampaign(view);
    const images = settingsOf(view).sceneImages || [];
    setSceneImage(images.length > 0 ? images[images.length - 1] : null);
    setViewer('public');
    setSpellRoll(null);
    retryTurnRef.current = null;
  }

  async function refreshCampaigns() {
    try {
      const { campaigns: list } = await api.listCampaigns();
      setCampaigns(list);
    } catch {
      // Non-fatal: the local list keeps working.
    }
  }

  // bootstrap loads the last-active campaign, else the most recent one, else
  // surfaces the legacy-vault import banner / PartySetup.
  async function bootstrap() {
    const lastId = getActiveCampaignId();
    if (lastId) {
      try {
        adoptCampaign(await api.getCampaign(lastId));
        setPage('table');
        void refreshCampaigns();
        return;
      } catch (caught) {
        if (!(caught instanceof ApiError && caught.status === 404)) throw caught;
        // 404: the campaign was deleted elsewhere; fall through to the list.
      }
    }
    const { campaigns: list } = await api.listCampaigns();
    setCampaigns(list);
    if (list.length > 0) {
      adoptCampaign(await api.getCampaign(list[0].id));
      setPage('table');
      return;
    }
    setLegacyCampaigns(readLegacyVault());
    setCampaignState(structuredClone(initialCampaign));
    setSceneImage(null);
  }

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        await bootstrap();
      } catch (caught) {
        if (!cancelled) setError(`無法連線本機伺服器：${message(caught)}`);
      } finally {
        if (!cancelled) setBooting(false);
      }
    })();
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const activeDmProvider = settings.dmProvider || status?.dmProvider || 'codex';
  const activeDmInfo = status?.dmProviders?.find((entry) => entry.id === activeDmProvider);

  useEffect(() => {
    const controller = new AbortController();
    const q = settings.dmProvider ? `?dmProvider=${encodeURIComponent(settings.dmProvider)}` : '';
    fetch(`/api/status${q}`, { signal: controller.signal }).then((response) => response.json()).then((data: AiStatus) => setStatus(data)).catch(() => setStatus({ connected: false, provider: 'Codex CLI', model: null, message: '本機伺服器未啟動' }));
    return () => controller.abort();
  }, [settings.dmProvider]);

  // Refresh the DM connection binding on load and whenever the active story
  // or provider changes; switching stories invalidates the connection until
  // the player re-consents to connect.
  useEffect(() => {
    const controller = new AbortController();
    const q = activeDmProvider ? `?dmProvider=${encodeURIComponent(activeDmProvider)}` : '';
    fetch(`/api/codex/connection${q}`, { signal: controller.signal })
      .then((response) => response.json())
      .then((data: { alive?: boolean; storyId?: string }) => setCodexConn({ alive: Boolean(data.alive), storyId: data.storyId || '' }))
      .catch(() => {});
    return () => controller.abort();
  }, [campaign.id, activeDmProvider]);

  const codexReady = !campaign.id || (Boolean(codexConn?.alive) && codexConn?.storyId === campaign.id);
  // Grok can still "connect" (bind story) for consent UX; treat unbound as needs connect.
  const needsCodexConnect = !demoMode && !codexReady;
  const dmLabel = activeDmInfo?.label || (activeDmProvider === 'grok' ? 'Grok' : 'Codex');

  async function connectCodex() {
    if (connecting) return;
    setConnecting(true); setError('');
    try {
      const response = await fetch('/api/codex/connect', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ campaignId: campaign.id, dmProvider: activeDmProvider }),
      });
      const data = await response.json().catch(() => ({} as { alive?: boolean; storyId?: string; error?: string }));
      if (!response.ok) throw new Error(data.error || '連線失敗');
      setCodexConn({ alive: Boolean(data.alive), storyId: data.storyId || '' });
      setNotice(`已連線 ${dmLabel}，這個故事現在可以請 DM 裁定。`);
      // Replay a turn that was blocked by the consent gate, if any.
      const retry = retryTurnRef.current;
      retryTurnRef.current = null;
      if (retry && data.alive) void advance(retry);
    } catch (caught) {
      setError(`連線 ${dmLabel} 失敗：${message(caught)}`);
    } finally {
      setConnecting(false);
    }
  }

  const latestDm = useMemo(() => [...campaign.story].reverse().find((entry) => entry.speaker === 'dm' && (!entry.audience || entry.audience === 'public')), [campaign.story]);
  const visibleStory = useMemo(() => campaign.story.filter((entry) => !entry.audience || entry.audience === 'public' || entry.audience === viewer), [campaign.story, viewer]);
  const appStyle = { '--font-scale': String(settings.fontScale || 1) } as CSSProperties;
  const activeRequiredCheck = spellRoll?.check || campaign.requiredCheck;
  const currentCombatant = campaign.combat?.active ? campaign.combat.combatants[campaign.combat.turnIndex] : undefined;
  const selectedImageBackend = settings.imageBackend || status?.imageBackend || 'codex';
  const forgeDefaults = status?.ForgeDefaults?.[selectedImageBackend];
  const forgeSettings = settings.forgeSettings || (forgeDefaults ? { ...forgeDefaults, Enabled: false } : undefined);
  const contextualTip = [
    activeRequiredCheck && { id: 'required-roll', title: '先完成必要擲骰', text: '骰盤已自動選好 d20、角色加值與目標 DC。按下擲骰後，結果會寫入故事紀錄並解鎖下一步。' },
    currentCombatant && { id: currentCombatant.side === 'enemy' ? 'enemy-turn' : 'combat-turn', title: currentCombatant.side === 'enemy' ? '現在是怪獸回合' : `現在是 ${currentCombatant.name} 的回合`, text: currentCombatant.side === 'enemy' ? '敵方回合會自動由 AI 選擇目標並結算；也可在戰鬥區手動按「敵方行動」。' : '在戰鬥區選擇攻擊或法術與目標。完成後系統會自動移到下一位。' },
    campaign.players.some((player) => campaign.xpProgress?.[player.id]?.ready) && { id: 'level-ready', title: '有角色可以升級', text: '前往「角色」頁選擇原職業升級或其他職業多職業；生命、熟練與法術位會自動更新。', page: 'characters' as Page },
    campaign.round <= 1 && { id: 'first-action', title: '如何進行一回合', text: '每位玩家可點選 DM 建議或輸入自己的做法，再鎖定行動；全員完成後故事才會推進，最後一人送出前都能修改。' },
    !sceneImage && { id: 'scene-image', title: '場景圖是選用功能', text: '可按「生成場景」繪製目前畫面；也能在設定開啟每回合自動生成。圖片不影響規則判定。' },
  ].filter(Boolean).find((tip) => tip && !(settings.dismissedTips || []).includes(tip.id)) as { id: string; title: string; text: string; page?: Page } | undefined;

  function dismissTip(id: string) {
    updateSettings({ dismissedTips: [...new Set([...(settings.dismissedTips || []), id])] });
  }

  // ---------------------------------------------------------------------------
  // Settings (server-side document; PATCH shallow-merges)

  function flushSettingsBox() {
    const box = settingsBoxRef.current;
    if (box.timer !== null) { window.clearTimeout(box.timer); box.timer = null; }
    const { id, patch } = box;
    box.patch = {};
    if (!id || Object.keys(patch).length === 0) return;
    api.patchSettings(id, patch as Record<string, unknown>).catch((caught) => setNotice(`設定尚未同步到伺服器：${message(caught)}`));
  }

  function updateSettings(patch: CampaignSettings, options: { debounce?: boolean } = {}) {
    // Optimistic local merge mirrors the server's shallow merge.
    setCampaignState((current) => ({ ...current, settings: { ...(current.settings || {}), ...patch } }));
    const id = campaignRef.current.id;
    if (!id) return;
    const box = settingsBoxRef.current;
    if (box.id !== id) flushSettingsBox();
    box.id = id;
    box.patch = { ...box.patch, ...patch };
    if (box.timer !== null) window.clearTimeout(box.timer);
    if (options.debounce) box.timer = window.setTimeout(() => { box.timer = null; flushSettingsBox(); }, 600);
    else flushSettingsBox();
  }

  function updateForgeSettings(patch: Partial<ForgeSettings>) {
    const base = forgeSettings;
    if (!base) return;
    updateSettings({ forgeSettings: { ...base, ...patch } }, { debounce: true });
  }

  // ---------------------------------------------------------------------------
  // Character actions (all server-authoritative)

  async function changeClassResource(id: PlayerId, resourceId: string, delta: number) {
    if (!campaign.id) return;
    try {
      setCampaign(await api.changeResource(campaign.id, id, resourceId, delta));
    } catch (caught) { setError(message(caught)); }
  }

  // After an out-of-combat cast the server also locks the action, which may
  // complete the round.
  function maybeAdvance(view: Campaign) {
    if (areAllActionsReady(view)) void advance({ actions: actionsFrom(view) });
  }

  function openSpellCast(casterId: PlayerId, spell: CharacterSpell, asRitual?: boolean, targetId?: string) {
    if (activeRequiredCheck) return setError('請先完成目前畫面上的必要擲骰，再進行施法。');
    const player = campaign.players.find((entry) => entry.id === casterId);
    if (!player) return;
    if (campaign.pending[casterId] && !campaign.combat?.active) {
      return setError(`${player.name}本回合已鎖定行動，請先解鎖才能改為施法。`);
    }
    if (spell.effect?.kind === 'damage' && !campaign.combat?.active) {
      return setError('傷害法術必須在戰鬥追蹤器有有效目標時結算。');
    }
    // Character sheet may already pass ritual + target; cast immediately.
    if (targetId) {
      void castSpell(casterId, spell, Boolean(asRitual), targetId);
      return;
    }
    setSpellModal({ playerId: casterId, spell });
    setError('');
  }

  async function castSpell(casterId: PlayerId, spell: CharacterSpell, asRitual: boolean, targetId?: string) {
    if (!campaign.id) return;
    if (activeRequiredCheck) return setError('請先完成目前畫面上的必要擲骰，再進行施法。');
    const player = campaign.players.find((entry) => entry.id === casterId);
    if (!player) return;
    if (campaign.pending[casterId]) return setError(`${player.name}本回合已鎖定行動，請先解鎖才能改為施法。`);
    if (spell.effect?.kind === 'damage' && !campaign.combat?.active) return setError('傷害法術必須在戰鬥追蹤器有有效目標時結算。');
    if (!targetId) return setError(`${spell.name} 必須先指定目標。`);
    try {
      const result = await api.castSpell(campaign.id, casterId, { spellId: spell.id, asRitual, targetId });
      setSpellModal(null);
      if (result.needsAttackRoll) {
        setSpellRoll({ check: result.needsAttackRoll, casterId, spell, asRitual, targetId });
        setError('');
        return;
      }
      if (result.view) {
        setCampaign(result.view);
        maybeAdvance(result.view);
      }
      setError('');
    } catch (caught) { setError(message(caught)); }
  }

  async function submitStoryRevision(note: string) {
    if (!campaign.id || revising) return;
    setRevising(true); setError('');
    const stamp = now();
    setRevisionChat((chat) => [...chat, { id: `${Date.now()}-p`, role: 'player', text: note, time: stamp }]);
    try {
      const resp = await api.reviseStory(campaign.id, {
        note,
        model: settings.selectedModel || '',
        effort: settings.selectedEffort || '',
        dmProvider: activeDmProvider,
      });
      setCampaign(resp.view);
      setRevisionChat((chat) => [...chat, { id: `${Date.now()}-s`, role: 'system', text: '已依你的說明重寫上一則公開敘事。', time: now() }]);
      if (settings.ttsEnabled && resp.text) void speakNarration(resp.text);
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 409 && caught.data.needsConsent) {
        setCodexConn({ alive: false, storyId: '' });
        setError(`${caught.message}\n請先連線 ${dmLabel} 後再修正敘事。`);
      } else {
        setError(message(caught));
      }
    } finally {
      setRevising(false);
    }
  }

  // Resubmit the cast with the rolled spell-attack total (DiceTray flow).
  async function resolveSpellAttack(total: number) {
    const pendingRoll = spellRoll;
    if (!pendingRoll || !campaign.id) return;
    try {
      const result = await api.castSpell(campaign.id, pendingRoll.casterId, { spellId: pendingRoll.spell.id, asRitual: pendingRoll.asRitual, targetId: pendingRoll.targetId, attackTotal: total });
      if (result.view) {
        setCampaign(result.view);
        maybeAdvance(result.view);
      }
      setError('');
    } catch (caught) { setError(message(caught)); }
  }

  async function rest(id: PlayerId, type: RestType) {
    const player = campaign.players.find((entry) => entry.id === id);
    if (!player || !campaign.id) return;
    if (campaign.combat?.active) return setError('戰鬥進行中不能休息。');
    if (activeRequiredCheck) return setError('請先完成目前的必要擲骰，再決定是否休息。');
    if (Object.keys(campaign.pending).length > 0) return setError('隊伍已有待裁定行動；請先完成或解鎖本輪行動，再開始休息。');
    const shortResources = player.resources.filter((resource) => resource.shortRestRecovery === 'all' || Number(resource.shortRestRecovery) > 0).map((resource) => resource.name);
    const explanation = type === 'short'
      ? `短休消耗 1 點探索行動時間（約一小時）。會自動花費生命骰直到生命全滿或生命骰用盡，並恢復短休資源${shortResources.length ? `（${shortResources.join('、')}）` : ''}${player.spellcasting?.mode === 'pact' ? '及契約法術位' : ''}；一般法術位不恢復。`
      : '長休消耗 4 點探索行動時間（約八小時）。會恢復全部生命、生命骰、法術位與職業資源，結束專注並將可恢復狀態重設。';
    if (!window.confirm(`確認讓 ${player.name}進行${type === 'short' ? '短休' : '長休'}？\n${explanation}`)) return;
    try {
      setCampaign(await api.rest(campaign.id, id, type));
      setError('');
    } catch (caught) { setError(message(caught)); }
  }

  async function endCombat() {
    if (loading || !campaign.id) return;
    try {
      const result = await api.combatConclude(campaign.id);
      setCampaign(result.view);
      void advance({ combatConclusion: result.conclusion });
    } catch (caught) { setError(message(caught)); }
  }

  const localImages = (settings.imageBackend || status?.imageBackend || '').startsWith('local');
  const canGenerateImages = Boolean(status?.imageBackends?.length) || Boolean(status?.connected) || localImages;

  const ttsAudioRef = useRef<HTMLAudioElement | null>(null);

  async function speakNarration(text: string) {
    try {
      const response = await fetch('/api/tts', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ text }) });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || '語音合成失敗');
      }
      const url = URL.createObjectURL(await response.blob());
      if (ttsAudioRef.current) {
        ttsAudioRef.current.pause();
        if (ttsAudioRef.current.src.startsWith('blob:')) URL.revokeObjectURL(ttsAudioRef.current.src);
      }
      const audio = new Audio(url);
      // Revoke this clip's blob URL once it finishes or errors, so the final
      // clip (which no later clip replaces) does not leak. The same lifecycle
      // drives dmSpeaking, which the 3D DM avatar reads to animate its mouth.
      const stop = () => { URL.revokeObjectURL(url); setDmSpeaking(false); };
      audio.addEventListener('playing', () => setDmSpeaking(true));
      audio.addEventListener('ended', stop, { once: true });
      audio.addEventListener('error', stop, { once: true });
      audio.addEventListener('pause', () => setDmSpeaking(false));
      ttsAudioRef.current = audio;
      setDmSpeaking(true); // optimistic; 'playing' confirms, avoids a start flicker
      void audio.play();
    } catch (caught) {
      setDmSpeaking(false);
      setNotice(`語音朗讀失敗：${message(caught)}`);
    }
  }

  function appendSceneImage(image: SceneImage) {
    setSceneImage(image);
    const current = campaignRef.current;
    const sceneImages = [...(settingsOf(current).sceneImages || []), image].slice(-24);
    updateSettings({ sceneImages });
  }

  async function generateImage(narrationOverride?: string, imagePromptOverride?: string, sceneOverride?: string, sceneSlotId?: string) {
    if (!canGenerateImages || imageLoading) return;
    const slotId = sceneSlotId || pendingSceneSlotId;
    const narration = narrationOverride || latestDm?.text;
    if (!narration && !slotId) return setImageError('目前沒有可供繪製的公開 DM 場景敘事。');
    setImageLoading(true); setImageError('');
    try {
      const response = await fetch('/api/scene-image', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          imageBackend: settings.imageBackend || '',
          campaignId: campaign.id || '',
          campaign: { title: campaign.title, scene: sceneOverride || campaign.scene },
          narration: narration || '',
          imagePrompt: imagePromptOverride ?? campaign.imagePrompt ?? '',
          sceneSlotId: slotId || undefined,
          forge: forgeDefaults ? forgeRequest(settings.forgeSettings) : undefined,
        }),
      });
      const data = await response.json().catch(() => ({} as { url?: string; model?: string; error?: string }));
      if (!response.ok) throw new Error(data.error || '場景插圖生成失敗');
      appendSceneImage({ url: data.url!, scene: sceneOverride || campaignRef.current.scene, createdAt: now(), model: data.model || status?.imageModel || 'Image' });
      if (slotId && slotId === pendingSceneSlotId) setPendingSceneSlotId('');
    } catch (caught) { setImageError(message(caught)); } finally { setImageLoading(false); }
  }

  async function generatePortrait(player: PlayerCharacter, appearance: string) {
    if (!canGenerateImages) return setError('圖片服務尚未連線。');
    if (!campaign.id) return;
    const description = appearance.trim();
    if (!description) return setError('請先輸入角色外觀描述。');
    try {
      const response = await fetch('/api/character-image', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ imageBackend: settings.imageBackend || '', name: player.name, species: player.species, className: player.className, background: player.background, appearance: description }) });
      const data = await response.json().catch(() => ({} as { url?: string; error?: string }));
      if (!response.ok) throw new Error(data.error || '角色圖片生成失敗');
      setCampaign(await api.patchPlayer(campaign.id, player.id, { appearance: description, portraitUrl: data.url }));
      setNotice(`${player.name}的角色外觀與肖像已更新。`);
    } catch (caught) { setError(message(caught)); }
  }

  async function submitAction(player: PlayerId, text: string) {
    if (!campaign.id || loading || advancingRef.current || campaign.pending[player]) return;
    try {
      const view = await api.submitAction(campaign.id, player, text);
      setCampaign(view);
      if (areAllActionsReady(view)) void advance({ actions: actionsFrom(view) });
    } catch (caught) { setError(message(caught)); }
  }

  async function unlockAction(player: PlayerId) {
    if (!campaign.id || loading) return;
    try {
      setCampaign(await api.unlockAction(campaign.id, player));
    } catch (caught) { setError(message(caught)); }
  }

  function showRejection(issues: ActionIssue[], players: PlayerCharacter[]) {
    const details = issues.map((issue) => `${players.find((player) => player.id === issue.playerId)?.name || issue.playerId}：${issue.message}`).join('；');
    setError(`【行動駁回】${details}。故事尚未推進；請依照理由修改後重新鎖定。`);
  }

  // Demo DM: narration/choices/XP are faked locally on top of the view; the
  // server still owns mechanical state (locks are released server-side).
  async function demoAdvance(input: AdvanceInput, previousCheck: RequiredCheck | null) {
    const campaignId = campaign.id!;
    const preset = storyPresets.find((story) => story.id === settings.storyId) || storyPresets[0];
    await new Promise((resolve) => window.setTimeout(resolve, input.actions ? 500 : 350));
    let text: string;
    let choices: Choice[] = [];
    let requiredCheck: Campaign['requiredCheck'] = null;
    let nextObjective: string | undefined;
    let nextObjectiveContext: string | undefined;
    let nextStakes: string | undefined;
    let award = 0;
    if (input.combatConclusion) {
      text = input.combatConclusion.outcome === 'victory'
        ? '最後的兵刃聲落下，敵人的抵抗徹底停止。倖存者的喘息與戰場殘留的痕跡，讓隊伍看清這場衝突留下的代價，也暴露出下一步可追查的線索。戰鬥已經結束，接下來可以先確認傷勢與現場，再決定去向。'
        : input.combatConclusion.outcome === 'defeat'
          ? '戰線徹底崩解，眾人已無法繼續正面抵抗。敵人掌握了現場，但故事並未在此終止；倖存者、俘虜或外力將決定隊伍必須面對的新局勢。'
          : '雙方脫離交戰距離，兵刃聲逐漸被急促呼吸取代。這場戰鬥沒有以徹底勝負收場，撤退路線、傷勢與敵人的下一步成了眼前最迫切的問題。';
      choices = (input.combatConclusion.outcome === 'victory' ? ['檢查戰場與敵人', '先救治傷者', '繼續追查線索'] : ['確認隊伍傷勢', '尋找安全退路', '觀察敵人動向']).map((choice) => ({ text: choice }));
    } else if (input.checkRoll) {
      const check = previousCheck;
      const success = input.checkRoll.success === true;
      text = check
        ? success
          ? `${check.character}的${check.ability}（${check.skill}）檢定成功。${check.reason}的風險被穩穩克服，眼前的阻礙讓開，並露出足以讓隊伍繼續判斷的新線索。局勢已向前推進，不需要重複宣告剛才的行動。`
          : `${check.character}的${check.ability}（${check.skill}）檢定失敗。${check.reason}帶來了立即而具體的代價，但故事沒有停住；隊伍仍可依照眼前出現的新局勢決定下一步。`
        : '檢定已完成，故事繼續推進。';
      choices = (success ? ['檢查新出現的線索', '趁局勢有利繼續前進'] : ['處理失敗造成的後果', '改用另一條路徑']).map((choice) => ({ text: choice }));
    } else {
      const demoBeat = preset.demoBeats[(campaign.round - 1) % preset.demoBeats.length];
      text = demoBeat.text;
      choices = demoBeat.choices.map((choice) => ({ text: choice }));
      requiredCheck = demoBeat.check ? { character: campaign.players[0]?.name || '冒險者', ...demoBeat.check } : null;
      nextObjective = demoBeat.objective;
      nextObjectiveContext = demoBeat.objectiveContext;
      nextStakes = preset.stakes;
      award = 75;
      // Release the server-side action locks; narration itself stays local.
      for (const player of campaign.players) {
        try { await api.unlockAction(campaignId, player.id); } catch { /* keep going */ }
      }
    }
    const isContinuation = !input.actions;
    setCampaignState((current) => {
      const actionEntries = isContinuation ? [] : current.players.map((entry) => makeEntry(entry.id, current.pending[entry.id] || '本回合不行動，保持警戒。'));
      const players = award > 0 ? current.players.map((player) => ({ ...player, experience: player.experience + award })) : current.players;
      const xpLogs = award > 0 ? players.map((player) => makeEntry('system', `${player.name}獲得 ${award} XP：推進「${preset.title}」並取得新線索`)) : [];
      return {
        ...current,
        round: current.round + (isContinuation ? 0 : 1),
        pending: {},
        choices,
        requiredCheck,
        objective: nextObjective || current.objective,
        objectiveContext: nextObjectiveContext || current.objectiveContext,
        stakes: nextStakes || current.stakes,
        players,
        story: [...current.story, ...actionEntries, makeEntry('dm', text), ...xpLogs],
      };
    });
    if (settings.autoSceneImages && text) void generateImage(text);
    if (settings.ttsEnabled && text) void speakNarration(text);
  }

  async function advance(input: AdvanceInput) {
    const campaignId = campaign.id;
    if (!campaignId) return;
    const previousCheck = campaign.requiredCheck || null;
    const combatWasActive = campaign.combat?.active === true;
    setLoading(true); setError(''); advancingRef.current = true;
    retryTurnRef.current = null;
    if (input.checkRoll) setCampaignState((current) => ({ ...current, requiredCheck: null }));
    try {
      if (demoMode) {
        await demoAdvance(input, previousCheck);
        return;
      }
      const controller = new AbortController();
      const timeout = window.setTimeout(() => controller.abort(), 220000);
      let resp: api.DmTurnResponse;
      try {
        resp = await api.dmTurn({
          campaignId,
          model: settings.selectedModel || '',
          effort: settings.selectedEffort || '',
          dmProvider: activeDmProvider,
          ...(input.checkRoll
            ? { checkRoll: { natural: input.checkRoll.natural } }
            : input.combatConclusion
              ? { combatConclusion: input.combatConclusion }
              : { actions: input.actions || [] }),
        }, controller.signal);
      } finally {
        window.clearTimeout(timeout);
      }
      setCampaign(resp.view);
      if (resp.sceneSlot?.id) setPendingSceneSlotId(resp.sceneSlot.id);
      if (resp.actionIssues.length > 0) {
        // AI narrative veto: the view already carries the 【行動駁回】 entry and
        // the released locks; mirror it in the error banner.
        showRejection(resp.actionIssues, resp.view.players);
        return;
      }
      const combatStarted = !combatWasActive && resp.view.combat?.active === true;
      if ((settings.autoSceneImages || combatStarted) && resp.text) {
        void generateImage(resp.text, resp.view.imagePrompt || resp.sceneSlot?.imagePrompt, resp.view.scene, resp.sceneSlot?.id);
      }
      if (settings.ttsEnabled && resp.text) void speakNarration(resp.text);
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 409 && caught.data.needsConsent) {
        setCodexConn({ alive: false, storyId: '' });
        retryTurnRef.current = input;
        if (input.checkRoll && previousCheck) setCampaignState((current) => ({ ...current, requiredCheck: previousCheck }));
        setError(`${caught.message}\n請按「連線 ${dmLabel}」後即會自動重試本回合。`);
        return;
      }
      if (caught instanceof ApiError && caught.status === 422) {
        // Mechanical pre-validation rejected the round; the story did not
        // advance and the server kept the locks — release the rejected ones.
        const issues = (Array.isArray(caught.data.actionIssues) ? caught.data.actionIssues as ActionIssue[] : [])
          .filter((issue) => /^player[1-4]$/.test(issue.playerId || '') && issue.message);
        let latest: Campaign | null = null;
        for (const issue of issues) {
          try { latest = await api.unlockAction(campaignId, issue.playerId); } catch { /* lock stays; player can unlock manually */ }
        }
        if (latest) setCampaign(latest);
        showRejection(issues, (latest || campaign).players);
        return;
      }
      const msg = message(caught);
      if (input.checkRoll) {
        if (previousCheck) setCampaignState((current) => ({ ...current, requiredCheck: previousCheck }));
        setError(`${msg}\n檢定結果尚未推進故事，必要骰盤已恢復，請再試一次。`);
      } else if (input.combatConclusion) {
        retryTurnRef.current = input;
        setError(`${msg}\n戰後敘述尚未完成；請按「重試上一步」再試一次。`);
      } else {
        setError(`${msg}\n請修改或補充行動後重新提交；已鎖定內容仍保留。`);
      }
    } finally { setLoading(false); advancingRef.current = false; }
  }

  function retryLastTurn() {
    const retry = retryTurnRef.current;
    retryTurnRef.current = null;
    if (retry) void advance(retry);
  }

  // ---------------------------------------------------------------------------
  // Campaign lifecycle

  async function resetCampaign() {
    if (!campaign.id) return;
    if (!window.confirm('重設會刪除伺服器上這個戰役的所有進度，並回到開團設定。確定繼續？')) return;
    const id = campaign.id;
    try {
      await api.deleteCampaign(id);
    } catch (caught) { return setError(message(caught)); }
    setActiveCampaignId('');
    setCampaigns((current) => current.filter((entry) => entry.id !== id));
    setCampaignState(structuredClone(initialCampaign));
    setSceneImage(null);
    setError(''); setPage('table');
  }

  async function switchCampaign(id: string) {
    try {
      const next = await api.getCampaign(id);
      adoptCampaign(next);
      setPage('table');
      setNotice(`已載入「${next.title}」。`);
    } catch (caught) { setError(message(caught)); }
  }

  function newCampaign() {
    // The real campaign is created server-side when PartySetup completes.
    setCampaignState(structuredClone(initialCampaign));
    setSceneImage(null);
    setPage('table');
  }

  async function duplicateCurrentCampaign() {
    if (!campaign.id) return;
    try {
      const response = await fetch(api.exportUrl(campaign.id));
      const envelope = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(envelope.error || '匯出戰役失敗');
      const source = (envelope.campaign || envelope) as Record<string, unknown>;
      const copyTitle = `${String(source.title || campaign.title)}（副本）`;
      const copy = { ...source, id: undefined, title: copyTitle, pending: {} };
      await api.importCampaign(JSON.stringify(copy), false);
      await refreshCampaigns();
      setNotice(`已建立「${copyTitle}」，目前戰役未切換。`);
    } catch (caught) { setError(message(caught)); }
  }

  async function importFile(raw: string) {
    try {
      const imported = await api.importCampaign(raw, false);
      await refreshCampaigns();
      setNotice(`已匯入「${imported.title}」，目前戰役未切換。`);
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 409) {
        if (!window.confirm('伺服器上已有同 ID 的戰役。要以匯入檔覆蓋伺服器上的版本嗎？')) return;
        try {
          const imported = await api.importCampaign(raw, true);
          await refreshCampaigns();
          if (imported.id && imported.id === campaign.id) adoptCampaign(imported);
          setNotice(`已匯入並覆蓋「${imported.title}」。`);
        } catch (again) { setError(message(again)); }
        return;
      }
      setError(message(caught));
    }
  }

  async function removeCampaign(id: string) {
    const target = campaigns.find((entry) => entry.id === id);
    if (!window.confirm(`確定要刪除「${target?.title || id}」嗎？伺服器上的進度將永久移除。`)) return;
    try {
      await api.deleteCampaign(id);
      setCampaigns((current) => current.filter((entry) => entry.id !== id));
      if (campaign.id === id) {
        setActiveCampaignId('');
        try { await bootstrap(); } catch { setCampaignState(structuredClone(initialCampaign)); }
        setPage('table');
      }
      setNotice('戰役已刪除。');
    } catch (caught) { setError(message(caught)); }
  }

  // Upload every legacy localStorage campaign to the server, then load the
  // last imported one. 409 (same id already on the server) asks per campaign.
  async function importLegacyVault() {
    if (legacyImporting) return;
    setLegacyImporting(true); setError('');
    let lastView: Campaign | null = null;
    const failures: string[] = [];
    for (const legacy of legacyCampaigns) {
      try {
        lastView = await api.importCampaign(JSON.stringify(legacy), false);
      } catch (caught) {
        if (caught instanceof ApiError && caught.status === 409) {
          if (window.confirm(`伺服器上已有「${legacy.title}」（相同 ID）。要以本機版本覆蓋嗎？`)) {
            try { lastView = await api.importCampaign(JSON.stringify(legacy), true); } catch (again) { failures.push(`${legacy.title}：${message(again)}`); }
          }
          continue;
        }
        failures.push(`${legacy.title}：${message(caught)}`);
      }
    }
    setLegacyImporting(false);
    setLegacyCampaigns([]);
    await refreshCampaigns();
    if (lastView) { adoptCampaign(lastView); setPage('table'); }
    if (failures.length > 0) setError(`部分戰役匯入失敗：${failures.join('；')}`);
    else if (lastView) setNotice('本機戰役已上傳到伺服器，之後的進度都會保存在伺服器上。');
  }

  function completeSetup(view: Campaign) {
    adoptCampaign(view);
    void refreshCampaigns();
    setPage('table'); setError('');
  }

  function speakerLabel(entry: StoryEntry) {
    if (entry.speaker === 'dm') return entry.audience && entry.audience !== 'public' ? `地城主私訊 ${campaign.players.find((player) => player.id === entry.audience)?.name || entry.audience}` : '地城主';
    if (entry.speaker === 'system') return '紀錄';
    return campaign.players.find((player) => player.id === entry.speaker)?.name || '冒險者';
  }

  if (booting) {
    return <main className="single-page lazy-page-loading" role="status"><span>正在載入戰役資料…</span></main>;
  }

  if (!campaign.setupComplete) {
    return (
      <>
        {legacyCampaigns.length > 0 && (
          <div className="notice-banner legacy-import-banner" role="region" aria-label="本機戰役匯入">
            <CloudArrowUp size={20} />
            <span>偵測到 {legacyCampaigns.length} 個舊版本機戰役。上傳後即可在伺服器上繼續遊玩。</span>
            <MagneticButton onClick={() => void importLegacyVault()} disabled={legacyImporting}>{legacyImporting ? '上傳中…' : '將本機戰役上傳到伺服器'}</MagneticButton>
            <button type="button" aria-label="關閉匯入提示" onClick={() => setLegacyCampaigns([])}><XCircle /></button>
          </div>
        )}
        {error && <div className="error-banner" role="alert"><XCircle size={19} /><div><strong>操作中斷</strong><span>{error}</span></div><button type="button" onClick={() => setError('')}><XCircle /></button></div>}
        <PartySetup onComplete={completeSetup} onCancel={campaigns.length > 0 ? () => { void bootstrap().catch((caught) => setError(message(caught))); } : undefined} />
      </>
    );
  }

  return (
    <div className="app-shell" style={appStyle}><div className="grain" aria-hidden="true" /><Sidebar page={page} setPage={setPage} /><div className="workspace">
      <Topbar campaign={campaign} status={status} demoMode={demoMode} />
      {/* Non-table pages still get global notice/tip above content */}
      {page !== 'table' && notice && <div className="notice-banner"><span>{notice}</span><button type="button" onClick={() => setNotice('')}><XCircle /></button></div>}
      {page !== 'table' && contextualTip && <aside className="novice-tip" aria-label="新手提示"><Lightbulb size={20} /><div><strong>{contextualTip.title}</strong><span>{contextualTip.text}</span></div>{contextualTip.page && <button type="button" className="tip-action" onClick={() => setPage(contextualTip.page!)}>前往查看</button>}<button type="button" className="tip-dismiss" aria-label="關閉提示" onClick={() => dismissTip(contextualTip.id)}><XCircle /></button></aside>}
      <AnimatePresence mode="wait">
        {page === 'table' && (
        <motion.main key="table" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="table-layout">
          <div className="table-main">
          {/* Match bug/image.png: image on top, then viewer switch, then
              tip → notices → scene/state → summary right above 最新對話. */}
          <div className="table-preamble">
            <SceneVisual
              image={sceneImage}
              images={settings.sceneImages || []}
              scene={campaign.scene}
              loading={imageLoading}
              error={imageError}
              canGenerate={canGenerateImages}
              onGenerate={() => void generateImage()}
              onSelect={setSceneImage}
            />
          </div>

          <div className="viewer-switch">
            <LockKey />
            <span>訊息視角</span>
            <select value={viewer} onChange={(event) => setViewer(event.target.value as MessageAudience)}>
              <option value="public">公開訊息</option>
              {campaign.players.map((player) => (
                <option key={player.id} value={player.id}>{player.name} 的私密訊息</option>
              ))}
            </select>
          </div>

          <div className="table-preamble table-context">
            {contextualTip && (
              <aside className="novice-tip novice-tip-inline" aria-label="新手提示">
                <Lightbulb size={20} />
                <div>
                  <strong>{contextualTip.title}</strong>
                  <span>{contextualTip.text}</span>
                </div>
                {contextualTip.page && (
                  <button type="button" className="tip-action" onClick={() => setPage(contextualTip.page!)}>前往查看</button>
                )}
                <button type="button" className="tip-dismiss" aria-label="關閉提示" onClick={() => dismissTip(contextualTip.id)}>
                  <XCircle />
                </button>
              </aside>
            )}
            {notice && (
              <div className="notice-banner">
                <span>{notice}</span>
                <button type="button" onClick={() => setNotice('')}><XCircle /></button>
              </div>
            )}
            {status && !status.connected && !demoMode && (
              <div className="model-notice">
                <ShieldWarning size={20} />
                <div><strong>本機 AI 尚未連線</strong><span>請先執行 codex login，或使用示範 DM。</span></div>
                <MagneticButton variant="quiet" onClick={() => setDemoMode(true)}>使用示範 DM</MagneticButton>
              </div>
            )}
            {needsCodexConnect && (status?.connected || activeDmInfo?.connected) && (
              <div className="model-notice">
                <Plugs size={20} />
                <div>
                  <strong>需要連線 {dmLabel}</strong>
                  <span>每個故事各自一條 DM 連線；「{campaign.title}」要先連線後 DM 才會裁定。切換故事、切換資料源或連線中斷後都需要重新連線。</span>
                </div>
                <MagneticButton onClick={() => void connectCodex()} disabled={connecting}>
                  {connecting ? '連線中…' : `連線 ${dmLabel}`}
                </MagneticButton>
              </div>
            )}
            {error && (
              <div className="error-banner" role="alert">
                <XCircle size={19} />
                <div><strong>操作中斷</strong><span>{error}</span></div>
                {retryTurnRef.current && !needsCodexConnect && !loading && (
                  <button type="button" className="retry-turn" onClick={retryLastTurn}><ArrowClockwise />重試上一步</button>
                )}
                <button type="button" onClick={() => setError('')}><XCircle /></button>
              </div>
            )}

            <section className="scene-strip" aria-label="場景與遊戲狀態">
              <MapTrifold size={21} />
              <div>
                <span>目前場景</span>
                <strong>{campaign.scene}</strong>
              </div>
              <div className={`game-state ${campaign.combat?.active ? 'game-state-combat' : 'game-state-exploration'}`}>
                {campaign.combat?.active ? <Sword size={17} weight="fill" /> : <Compass size={17} />}
                <div>
                  <span>遊戲狀態</span>
                  <strong>
                    {campaign.combat?.active
                      ? `戰鬥中${currentCombatant ? `・${currentCombatant.name} 行動` : ''}`
                      : '探索中'}
                  </strong>
                </div>
              </div>
              <div className="round-mark">
                <span>{campaign.combat?.active ? '戰鬥輪' : '探索回合'}</span>
                <strong>{String(campaign.combat?.active ? campaign.combat.round : campaign.round).padStart(2, '0')}</strong>
              </div>
            </section>

            <section className="objective objective-preamble" aria-label="任務摘要">
              <p className="eyebrow">任務摘要</p>
              <strong>{campaign.objective}</strong>
              <p className="objective-context">{campaign.objectiveContext}</p>
              <div className="objective-stakes">
                <span>風險</span>
                <p>{campaign.stakes}</p>
              </div>
              <small>
                {campaign.combat?.active
                  ? `戰鬥第 ${campaign.combat.round} 輪（戰鬥操作在對話下方）`
                  : '探索進行中'}
              </small>
            </section>
          </div>

          <div className={`story-workspace ${revisionOpen ? 'story-workspace-revision' : ''}`}>
            <div className="story-workspace-main">
              <StoryFeed story={campaign.story} players={campaign.players} loading={loading} viewer={viewer} />
              <div className="story-revision-toggle-row">
                <button
                  type="button"
                  className={`story-revision-toggle ${revisionOpen ? 'active' : ''}`}
                  disabled={!latestDm || loading || revising}
                  onClick={() => setRevisionOpen((open) => !open)}
                >
                  {revisionOpen ? '關閉敘事修正' : '修正上一則 DM 敘事'}
                </button>
              </div>
            </div>
            <StoryRevisionPanel
              open={revisionOpen}
              onClose={() => setRevisionOpen(false)}
              loading={revising}
              disabled={loading || needsCodexConnect || !latestDm}
              disabledReason={needsCodexConnect ? `請先連線 ${dmLabel}` : !latestDm ? '尚無公開 DM 敘事' : undefined}
              previousDraft={latestDm?.text || ''}
              chat={revisionChat}
              onSubmit={(note) => void submitStoryRevision(note)}
            />
          </div>
          {activeRequiredCheck && <DiceTray players={campaign.players} requiredCheck={activeRequiredCheck} onRoll={({ total }) => { if (spellRoll) void resolveSpellAttack(total); }} onRequiredRoll={(roll) => { if (spellRoll) { setSpellRoll(null); return; } if (campaign.requiredCheck) void advance({ checkRoll: { natural: roll.natural, success: roll.success } }); }} />}
          {campaign.combat?.active && campaign.id && (
            <section className="inline-combat">
              <div className="section-heading">
                <div><p className="eyebrow">戰鬥進行中</p><h2>先攻與回合操作</h2></div>
              </div>
              <CombatTracker
                campaignId={campaign.id}
                players={campaign.players}
                combat={campaign.combat}
                onView={setCampaign}
                onEnd={() => void endCombat()}
                onCastSpell={(playerId, spell) => openSpellCast(playerId, spell)}
              />
            </section>
          )}
          <div className={`composer-grid party-${campaign.players.length}`}>
            {campaign.players.map((player) => (
              <section className="player-console" key={player.id} aria-label={`${player.name}玩家操作區`}>
                <CharacterPanel
                  player={player}
                  xp={campaign.xpProgress?.[player.id]}
                  showStatHints={settings.showStatHints !== false}
                  combatActive={campaign.combat?.active === true}
                  pending={campaign.pending[player.id]}
                  actionDisabled={loading || Boolean(activeRequiredCheck)}
                  partySize={campaign.players.length}
                  choices={(campaign.choices || []).filter((choice) => !choice.playerId || choice.playerId === player.id)}
                  resourceSummary={player.spellcasting
                    ? player.spellcasting.slots.map((slot) => `${slot.level}環 ${slot.current}/${slot.max}`).join('、')
                    : player.resources.slice(0, 3).map((resource) => `${resource.name} ${resource.current}/${resource.max}`).join('、')}
                  onSubmitAction={(id, text) => void submitAction(id, text)}
                  onUnlockAction={(id) => void unlockAction(id)}
                  spellTargets={[
                    ...campaign.players.map((entry) => ({ id: entry.id, name: entry.name, side: 'party' as const })),
                    ...(campaign.combat?.active
                      ? campaign.combat.combatants
                        .filter((entry) => entry.side === 'enemy' && !entry.defeated)
                        .map((entry) => ({ id: entry.id, name: entry.name, side: 'enemy' as const }))
                      : []),
                  ]}
                  onResourceChange={(id, resourceId, delta) => void changeClassResource(id, resourceId, delta)}
                  onCastSpell={(id, spell, asRitual, targetId) => openSpellCast(id, spell, asRitual, targetId)}
                  onRest={(id, type) => void rest(id, type)}
                  onGeneratePortrait={generatePortrait}
                />
              </section>
            ))}
          </div>
          {spellModal && (() => {
            const caster = campaign.players.find((p) => p.id === spellModal.playerId);
            if (!caster) return null;
            const targets = [
              ...campaign.players.map((entry) => ({ id: entry.id, name: entry.name, side: 'party' as const })),
              ...(campaign.combat?.active
                ? campaign.combat.combatants.filter((entry) => entry.side === 'enemy' && !entry.defeated).map((entry) => ({ id: entry.id, name: entry.name, side: 'enemy' as const }))
                : []),
            ];
            return (
              <SpellCastModal
                open
                player={caster}
                spell={spellModal.spell}
                spellTargets={targets}
                onClose={() => setSpellModal(null)}
                onCast={(spell, asRitual, targetId) => void castSpell(spellModal.playerId, spell, asRitual, targetId)}
              />
            );
          })()}
          </div>
        </motion.main>
        )}
        {page === 'characters' && <motion.div key="characters" initial={{ opacity: 0 }} animate={{ opacity: 1 }}><Suspense fallback={<main className="single-page lazy-page-loading" role="status"><span>正在載入角色成長資料…</span></main>}><CharacterManager players={campaign.players} xpProgress={campaign.xpProgress} showStatHints={settings.showStatHints !== false} onLevelUp={(playerId, className) => { if (!campaign.id) return; api.levelUp(campaign.id, playerId, className).then(setCampaign).catch((caught) => setError(message(caught))); }} onSpendAbilityPoint={(playerId, ability: AbilityKey) => { if (!campaign.id) return; api.spendAbilityPoint(campaign.id, playerId, ability).then(setCampaign).catch((caught) => setError(message(caught))); }} onSetPreparedSpells={(playerId, spellIds) => { if (!campaign.id) return; api.setPreparedSpells(campaign.id, playerId, spellIds).then(setCampaign).catch((caught) => setError(message(caught))); }} onSaveProfile={(playerId, patch) => { if (!campaign.id) return; api.patchPlayer(campaign.id, playerId, patch).then((view) => { setCampaign(view); setNotice('角色配置已儲存。'); }).catch((caught) => setError(message(caught))); }} onGeneratePortrait={generatePortrait} /></Suspense></motion.div>}
        {page === 'journal' && <motion.main key="journal" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page"><div className="page-intro"><p className="eyebrow">戰役記憶</p><h2>{campaign.title}</h2><p>公開與私密訊息都保存在伺服器戰役資料庫與匯出檔中。</p></div><div className="journal-list">{visibleStory.map((entry, index) => <article key={entry.id} className={entry.audience && entry.audience !== 'public' ? 'journal-private' : ''}><span>{String(index + 1).padStart(2, '0')}</span><div><small>{entry.time}／{speakerLabel(entry)}</small><p>{entry.text}</p></div></article>)}</div></motion.main>}
        {page === 'settings' && <motion.main key="settings" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page settings-page"><div className="page-intro"><p className="eyebrow">戰役設定</p><h2>地城主與戰役</h2><p>設定會即時保存在伺服器上的這個戰役；匯入預設不切換。</p></div>
          <section className="settings-row"><div><strong>示範 DM</strong><span>完全不呼叫模型。</span></div><button type="button" className={`switch ${demoMode ? 'switch-on' : ''}`} onClick={() => setDemoMode((value) => !value)}><i /></button></section>
          <section className="settings-row model-selector"><div><strong>DM 資料源</strong><span>Codex（ChatGPT 登入）或 Grok（`grok login`／XAI_API_KEY）。切換後請重新連線該故事。</span></div><select value={activeDmProvider} onChange={(event) => { updateSettings({ dmProvider: event.target.value, selectedModel: '', selectedEffort: '' }); setCodexConn(null); }}>{(status?.dmProviders?.length ? status.dmProviders : [{ id: 'codex', label: 'Codex CLI', connected: true }]).map((p) => <option key={p.id} value={p.id}>{p.label}{'connected' in p && !p.connected ? '（未就緒）' : ''}</option>)}</select></section>
          <section className="settings-row model-selector"><div><strong>模型</strong><span>只影響之後的新 DM 請求；目前進度與既有訊息不會改變。</span></div><select value={settings.selectedModel || ''} onChange={(event) => updateSettings({ selectedModel: event.target.value })}>{(activeDmInfo?.models || status?.models || [{ id: '', label: '預設模型' }]).map((model) => <option key={model.id || 'default'} value={model.id}>{model.label}</option>)}</select></section>
          <section className="settings-row model-selector"><div><strong>推理強度（effort）</strong><span>越高越深思但回應越慢；Grok 可能僅有預設。</span></div><select value={settings.selectedEffort || ''} onChange={(event) => updateSettings({ selectedEffort: event.target.value })}>{(activeDmInfo?.efforts || status?.efforts || [{ id: '', label: '預設推理強度' }]).map((effort) => <option key={effort.id || 'default'} value={effort.id}>{effort.label}</option>)}</select></section>
          <section className="settings-row"><div><strong>{dmLabel} 狀態</strong><span>{(activeDmInfo?.connected ?? status?.connected) ? `已就緒／${activeDmInfo?.model || status?.model || '—'}` : activeDmInfo?.message || status?.message || '正在檢查'}</span></div><ShieldWarning size={22} /></section>
          <section className="settings-row model-selector"><div><strong>圖片生成引擎</strong><span>場景圖與角色肖像使用的後端；本地選項需先啟動 SD Forge（--api）。</span></div><select value={settings.imageBackend || status?.imageBackend || 'codex'} onChange={(event) => updateSettings({ imageBackend: event.target.value })}>{(status?.imageBackends || [{ id: 'codex', label: status?.imageModel || 'Codex $imagegen' }]).map((backend) => <option key={backend.id} value={backend.id}>{backend.label}</option>)}</select></section>
          <section className="settings-row"><div><strong>每回合自動生成場景圖</strong><span>開啟後，每次 DM 完成公開敘事便自動生成並加入圖庫。</span></div><button type="button" className={`switch ${settings.autoSceneImages ? 'switch-on' : ''}`} onClick={() => updateSettings({ autoSceneImages: !settings.autoSceneImages })}><i /></button></section>
          <section className="settings-row"><div><strong>語音朗讀 DM 敘事</strong><span>使用本地 GPT-SoVITS 朗讀每回合公開敘事；需先啟動 scripts/sovits.sh 並設定聲線。</span></div><button type="button" role="switch" aria-checked={Boolean(settings.ttsEnabled)} aria-label="語音朗讀 DM 敘事" className={`switch ${settings.ttsEnabled ? 'switch-on' : ''}`} onClick={() => updateSettings({ ttsEnabled: !settings.ttsEnabled })}><i /></button></section>
          <section className="settings-row"><div><strong>角色屬性懸浮說明</strong><span>滑鼠停留或用鍵盤聚焦屬性時，顯示規則用途與計算方式。</span></div><button type="button" role="switch" aria-checked={settings.showStatHints !== false} aria-label="角色屬性懸浮說明" className={`switch ${settings.showStatHints !== false ? 'switch-on' : ''}`} onClick={() => updateSettings({ showStatHints: settings.showStatHints === false })}><i /></button></section>
          <section className="settings-row"><div><strong>介面字型大小</strong><span>{Math.round((settings.fontScale || 1) * 100)}%</span></div><div className="font-controls"><button type="button" onClick={() => updateSettings({ fontScale: Math.max(.85, (settings.fontScale || 1) - .1) })}>A−</button><button type="button" onClick={() => updateSettings({ fontScale: 1 })}>重設</button><button type="button" onClick={() => updateSettings({ fontScale: Math.min(1.25, (settings.fontScale || 1) + .1) })}>A＋</button></div></section>
          <CampaignManager campaign={campaign} campaigns={campaigns} onSwitch={(id) => void switchCampaign(id)} onNew={newCampaign} onDuplicate={() => void duplicateCurrentCampaign()} onImport={(raw) => void importFile(raw)} onDelete={(id) => void removeCampaign(id)} />
          <section className="settings-danger"><div><strong>重設目前戰役</strong><span>刪除伺服器上這個戰役的所有進度並回到開團設定。</span></div><MagneticButton variant="quiet" onClick={() => void resetCampaign()}>重設目前戰役</MagneticButton></section>
          {forgeDefaults && forgeSettings && <>
            <section className='settings-row'><div><strong>自訂 Forge 場景圖參數</strong><span>僅本地 Forge 使用；關閉時完全沿用伺服器 preset。開啟後 negative prompt 會強制使用 CFG &gt; 1。</span></div><button type='button' role='switch' aria-checked={forgeSettings.Enabled} aria-label='自訂 Forge 場景圖參數' className={'switch ' + (forgeSettings.Enabled ? 'switch-on' : '')} onClick={() => updateForgeSettings({ Enabled: !forgeSettings.Enabled })}><i /></button></section>
            {forgeSettings.Enabled && <section className='forge-settings' aria-label='Forge 場景圖參數'>
              <label className='forge-prompt'><span>Positive prompt</span><textarea rows={4} value={forgeSettings.PositivePrompt} placeholder='留空時使用 DM 產生的場景提示詞與寫實場景前後綴' onChange={(event) => updateForgeSettings({ PositivePrompt: event.target.value })} /></label>
              <label className='forge-prompt'><span>Negative prompt</span><textarea rows={4} value={forgeSettings.NegativePrompt} onChange={(event) => updateForgeSettings({ NegativePrompt: event.target.value })} /></label>
              <div className='forge-grid'>
                <label><span>Steps</span><input type='number' min={1} max={150} step={1} value={forgeSettings.Steps} onChange={(event) => updateForgeSettings({ Steps: Number(event.target.value) })} /></label>
                <label><span>CFG scale</span><input type='number' min={1.1} max={30} step={0.1} value={forgeSettings.CFGScale} onChange={(event) => updateForgeSettings({ CFGScale: Number(event.target.value) })} /></label>
                <label><span>Seed</span><input type='number' min={-1} max={2147483647} step={1} value={forgeSettings.Seed} onChange={(event) => updateForgeSettings({ Seed: Number(event.target.value) })} /></label>
                <label><span>Sampler</span><input type='text' value={forgeSettings.Sampler} onChange={(event) => updateForgeSettings({ Sampler: event.target.value })} /></label>
                <label><span>Scheduler</span><input type='text' value={forgeSettings.Scheduler} onChange={(event) => updateForgeSettings({ Scheduler: event.target.value })} /></label>
                <label><span>寬度</span><input type='number' min={256} max={2048} step={8} value={forgeSettings.Width} onChange={(event) => updateForgeSettings({ Width: Number(event.target.value) })} /></label>
                <label><span>高度</span><input type='number' min={256} max={2048} step={8} value={forgeSettings.Height} onChange={(event) => updateForgeSettings({ Height: Number(event.target.value) })} /></label>
              </div>
              <p>Seed 設為 -1 會每次隨機；固定 seed 才能與 Forge WebUI 重現相同構圖。Positive 留空時，系統仍使用本回合 DM prompt。</p>
            </section>}
          </>}
        </motion.main>}
      </AnimatePresence>
      <footer><span>{campaign.scene}</span><span>{latestDm ? `最後裁定 ${latestDm.time}` : '等待第一個裁定'}</span></footer>
    </div></div>
  );
}
