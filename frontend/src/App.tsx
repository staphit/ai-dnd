import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import { AnimatePresence } from 'framer-motion';
import { CloudArrowUp, Lightbulb, XCircle } from '@phosphor-icons/react';
import { initialCampaign } from './data';
import type { AiStatus, Campaign, CampaignSummary, CharacterSpell, ForgeSettings, MessageAudience, Page, PlayerCharacter, PlayerId, RequiredCheck, RestType } from './types';
import { Sidebar } from './components/Sidebar';
import { Topbar } from './components/Topbar';
import { MagneticButton } from './components/MagneticButton';
import { PartySetup } from './components/PartySetup';
import type { StageClearInfo } from './components/StageClearModal';
import type { RevisionChatLine } from './components/StoryRevisionPanel';
import { ToastLayer, useToasts } from './components/Toasts';
import * as api from './api';
import { ApiError, type ActionIssue } from './api';
import { setActiveCampaignId } from './campaign-storage';
import {
  actionsFrom,
  areAllActionsReady,
  errorMessage as message,
  settingsOf,
  timeLabel as now,
  type AdvanceInput,
} from './app/app-utils';
import { useDiceAnimation } from './hooks/useDiceAnimation';
import { useNarrationAudio } from './hooks/useNarrationAudio';
import { useSceneMedia } from './hooks/useSceneMedia';
import { useCampaignSettings } from './hooks/useCampaignSettings';
import { useCampaignLibrary } from './hooks/useCampaignLibrary';
import { CharactersPage } from './components/pages/CharactersPage';
import { JournalPage } from './components/pages/JournalPage';
import { SettingsPage } from './components/pages/SettingsPage';
import { TablePage } from './components/pages/TablePage';

export default function App() {
  const [campaign, setCampaignState] = useState<Campaign>(() => structuredClone(initialCampaign));
  const [campaigns, setCampaigns] = useState<CampaignSummary[]>([]);
  const [page, setPage] = useState<Page>('table');
  const [status, setStatus] = useState<AiStatus | null>(null);
  const [viewer, setViewer] = useState<MessageAudience>('public');
  const [demoMode, setDemoMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setErrorState] = useState('');
  const [notice, setNotice] = useState('');
  // Top-right toast stack + session history (bell in the Topbar).
  const { toasts, history: toastHistory, push: pushToast, dismiss: dismissToast, clearHistory: clearToastHistory } = useToasts();

  // Every error surface also lands in the toast stack; the inline banner keeps
  // its retry affordances.
  function setError(message: string) {
    setErrorState(message);
    if (message.trim()) pushToast('error', message);
  }
  const { speaking: dmSpeaking, speakNarration } = useNarrationAudio(setNotice);
  const { diceAnimation: diceAnim, playDiceAnimation: playDiceAnim } = useDiceAnimation();
  const [spellRoll, setSpellRoll] = useState<{ check: RequiredCheck; casterId: PlayerId; spell: CharacterSpell; asRitual: boolean; targetId: string } | null>(null);
  const [spellModal, setSpellModal] = useState<{ playerId: PlayerId; spell: CharacterSpell } | null>(null);
  const [revisionOpen, setRevisionOpen] = useState(false);
  // Act-completion popup (前期/中期/後期 goal reached), fed by dm responses.
  const [stageClear, setStageClear] = useState<StageClearInfo | null>(null);
  const [shopOpen, setShopOpen] = useState(false);
  const [shopBusy, setShopBusy] = useState(false);
  const [novelBusy, setNovelBusy] = useState(false);
  const [novelPlayerId, setNovelPlayerId] = useState<PlayerId | ''>('');
  const [revisionChat, setRevisionChat] = useState<RevisionChatLine[]>([]);
  const [revising, setRevising] = useState(false);
  const [codexConn, setCodexConn] = useState<{ alive: boolean; storyId: string } | null>(null);
  const [connecting, setConnecting] = useState(false);
  const advancingRef = useRef(false);
  // Holds the arguments of a turn that failed before advancing the story
  // (consent gate or post-conclude narration), so it can be replayed without
  // the players re-entering anything.
  const retryTurnRef = useRef<AdvanceInput | null>(null);
  const settings = settingsOf(campaign);
  const updateSettings = useCampaignSettings({
    campaignId: campaign.id,
    setCampaign: setCampaignState,
    onSyncError: setNotice,
  });

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
    adoptSceneMedia(view);
    setViewer('public');
    setSpellRoll(null);
    retryTurnRef.current = null;
  }

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
  const activeRequiredCheck = spellRoll?.check || campaign.requiredCheck || null;
  const currentCombatant = campaign.combat?.active ? campaign.combat.combatants[campaign.combat.turnIndex] : undefined;
  // Party wipe: combat is live and every party-side combatant is down.
  const partyWiped = useMemo(() => {
    if (!campaign.combat?.active) return false;
    const party = campaign.combat.combatants.filter((entry) => entry.side === 'party');
    return party.length > 0 && party.every((entry) => entry.defeated);
  }, [campaign.combat]);
  const selectedImageBackend = settings.imageBackend || status?.imageBackend || 'codex';
  const forgeDefaults = status?.ForgeDefaults?.[selectedImageBackend];
  const forgeSettings = settings.forgeSettings || (forgeDefaults ? { ...forgeDefaults, Enabled: false } : undefined);
  const {
    adoptSceneMedia,
    canGenerateImages,
    generateImage,
    generatePortrait,
    generatingSlotId,
    imageError,
    imageLoading,
    pendingSceneSlotId,
    refreshSceneSlots,
    resetSceneMedia,
    sceneImage,
    sceneSlots,
    setPendingSceneSlotId,
    setSceneImage,
  } = useSceneMedia({
    campaign,
    settings,
    status,
    forgeDefaults,
    latestNarration: latestDm?.text,
    updateSettings,
    onCampaign: setCampaign,
    onError: setError,
    onNotice: setNotice,
  });
  const {
    booting,
    legacyCampaigns,
    legacyImporting,
    setLegacyCampaigns,
    bootstrap,
    completeSetup,
    duplicateCurrentCampaign,
    importFile,
    importLegacyVault,
    newCampaign,
    removeCampaign,
    resetCampaign,
    switchCampaign,
  } = useCampaignLibrary({
    campaign,
    campaigns,
    setCampaignState,
    setCampaigns,
    adoptCampaign,
    resetSceneMedia,
    setPage,
    onError: setError,
    onNotice: setNotice,
  });
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
      setRevisionChat((chat) => [...chat, { id: `${Date.now()}-s`, role: 'system', text: '已依你的說明就地修正上一則 DM 對話。', time: now() }]);
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

  async function endCombat(final = false) {
    if (loading || !campaign.id) return;
    try {
      const result = await api.combatConclude(campaign.id);
      setCampaign(result.view);
      void advance({ combatConclusion: { ...result.conclusion, final } });
    } catch (caught) { setError(message(caught)); }
  }

  // Export the whole adventure as a first-person novel .txt (AI rewrite from
  // the chosen character's point of view, dialogue included).
  async function exportNovel(playerId: PlayerId) {
    if (!campaign.id || novelBusy) return;
    setNovelBusy(true); setError('');
    try {
      const resp = await api.exportNovel(campaign.id, playerId, activeDmProvider, settings.selectedModel || '');
      const blob = new Blob([`${resp.title}\n\n${resp.novel}\n`], { type: 'text/plain;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `${campaign.title}－${resp.narrator}視角劇本.txt`;
      anchor.click();
      URL.revokeObjectURL(url);
      setNotice(`劇本「${resp.title}」已輸出為 txt 檔。`);
    } catch (caught) { setError(message(caught)); } finally { setNovelBusy(false); }
  }

  async function shopAction(run: () => Promise<Campaign>) {
    if (!campaign.id || shopBusy) return;
    setShopBusy(true);
    try {
      setCampaign(await run());
      setError('');
    } catch (caught) { setError(message(caught)); } finally { setShopBusy(false); }
  }

  // Out-of-combat rescue: rescuer spends 1 exploration action point, the
  // downed character spends hit dice to stand back up.
  async function reviveDowned(targetId: PlayerId, rescuerId: PlayerId) {
    if (!campaign.id || loading) return;
    try {
      setCampaign(await api.revive(campaign.id, targetId, rescuerId));
      setError('');
    } catch (caught) { setError(message(caught)); }
  }

  // 戰鬥重來: restore party + enemies to the snapshot taken when this combat
  // started (same initiative order), taken from the party-wipe dialog.
  async function retryCombat() {
    if (loading || !campaign.id) return;
    try {
      setCampaign(await api.combatRetry(campaign.id));
      setError('');
      setNotice('戰鬥重來：隊伍與敵人已回到本場戰鬥開始時的狀態。');
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
    // DM rejections are narrative feedback, not failures: banner + info toast.
    setErrorState(`【行動駁回】${details}。故事尚未推進；請依照理由修改後重新鎖定。`);
    pushToast('info', `【行動駁回】${details}`);
  }

  async function advance(input: AdvanceInput) {
    const campaignId = campaign.id;
    if (!campaignId) return;
    // Re-entry guard: a double-fired dice roll or double submit must not send
    // two DM turns for the same round.
    if (advancingRef.current) return;
    const previousCheck = campaign.requiredCheck || null;
    const combatWasActive = campaign.combat?.active === true;
    setLoading(true); setError(''); advancingRef.current = true;
    retryTurnRef.current = null;
    if (input.checkRoll) setCampaignState((current) => ({ ...current, requiredCheck: null }));
    try {
      const controller = new AbortController();
      const timeout = window.setTimeout(() => controller.abort(), 220000);
      let resp: api.DmTurnResponse;
      try {
        resp = await api.dmTurn({
          campaignId,
          model: settings.selectedModel || '',
          effort: settings.selectedEffort || '',
          dmProvider: activeDmProvider,
          demo: demoMode,
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
      if (resp.stageClear) setStageClear(resp.stageClear);
      if (resp.sceneSlot?.id) setPendingSceneSlotId(resp.sceneSlot.id);
      void refreshSceneSlots(campaignId); // new beat = new slot in the row
      const rejectedIssues = resp.actionIssues || [];
      if (rejectedIssues.length > 0) {
        // AI narrative veto: the view already carries the 【行動駁回】 entry and
        // the released locks; mirror it in the error banner.
        showRejection(rejectedIssues, resp.view.players);
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
      // Stale dice tray: the server says there is no pending check (it was
      // already resolved — duplicate roll or another window). Adopt server
      // truth and close the tray instead of restoring it forever.
      if (input.checkRoll && caught instanceof ApiError && caught.status === 400) {
        try { setCampaign(await api.getCampaign(campaignId)); } catch { /* keep local view */ }
        setError('');
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

  if (booting) {
    return <main className="single-page lazy-page-loading" role="status"><span>正在載入戰役資料…</span></main>;
  }

  if (!campaign.setupComplete) {
    return (
      <>
        <ToastLayer toasts={toasts} history={toastHistory} onDismiss={dismissToast} onClear={clearToastHistory} />
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
    <div className="app-shell" style={appStyle}><div className="grain" aria-hidden="true" /><ToastLayer toasts={toasts} history={toastHistory} onDismiss={dismissToast} onClear={clearToastHistory} /><Sidebar page={page} setPage={setPage} script={campaign.script} /><div className="workspace">
      <Topbar campaign={campaign} status={status} demoMode={demoMode} />
      {/* Non-table pages still get global notice/tip above content */}
      {page !== 'table' && notice && <div className="notice-banner"><span>{notice}</span><button type="button" onClick={() => setNotice('')}><XCircle /></button></div>}
      {page !== 'table' && contextualTip && <aside className="novice-tip" aria-label="新手提示"><Lightbulb size={20} /><div><strong>{contextualTip.title}</strong><span>{contextualTip.text}</span></div>{contextualTip.page && <button type="button" className="tip-action" onClick={() => setPage(contextualTip.page!)}>前往查看</button>}<button type="button" className="tip-dismiss" aria-label="關閉提示" onClick={() => dismissTip(contextualTip.id)}><XCircle /></button></aside>}
      <AnimatePresence mode="wait">
        {page === 'table' && (
          <TablePage
            campaign={campaign}
            settings={settings}
            status={status}
            viewer={viewer}
            sceneImage={sceneImage}
            sceneSlots={sceneSlots}
            generatingSlotId={generatingSlotId}
            imageLoading={imageLoading}
            imageError={imageError}
            canGenerateImages={canGenerateImages}
            contextualTip={contextualTip}
            notice={notice}
            error={error}
            demoMode={demoMode}
            needsDmConnect={needsCodexConnect}
            canConnectDm={Boolean(status?.connected || activeDmInfo?.connected)}
            dmLabel={dmLabel}
            connecting={connecting}
            loading={loading}
            retryAvailable={Boolean(retryTurnRef.current)}
            activeRequiredCheck={activeRequiredCheck}
            currentCombatantName={currentCombatant?.name}
            diceAnimation={diceAnim}
            revisionOpen={revisionOpen}
            revising={revising}
            latestDm={latestDm}
            revisionChat={revisionChat}
            partyWiped={partyWiped}
            stageClear={stageClear}
            shopOpen={shopOpen}
            shopBusy={shopBusy}
            spellModal={spellModal}
            onPageChange={setPage}
            onViewerChange={setViewer}
            onSelectSceneImage={setSceneImage}
            onGenerateImage={(narration, prompt, scene, slotId) => void generateImage(narration, prompt, scene, slotId)}
            onDismissTip={dismissTip}
            onDismissNotice={() => setNotice('')}
            onEnableDemo={() => setDemoMode(true)}
            onConnectDm={() => void connectCodex()}
            onDismissError={() => setError('')}
            onRetryLastTurn={retryLastTurn}
            onRevive={(targetId, rescuerId) => void reviveDowned(targetId, rescuerId)}
            onOpenShop={() => setShopOpen(true)}
            onToggleRevision={() => setRevisionOpen((open) => !open)}
            onCloseRevision={() => setRevisionOpen(false)}
            onSubmitRevision={(note) => void submitStoryRevision(note)}
            onDiceRoll={(success, total) => { playDiceAnim(success); if (spellRoll) void resolveSpellAttack(total); }}
            onRequiredRoll={(natural, success) => { if (spellRoll) { setSpellRoll(null); return; } if (campaign.requiredCheck) void advance({ checkRoll: { natural, success } }); }}
            onCampaign={setCampaign}
            onEndCombat={(final) => void endCombat(final)}
            onOpenSpellCast={openSpellCast}
            onResourceChange={(playerId, resourceId, delta) => void changeClassResource(playerId, resourceId, delta)}
            onUseItem={(playerId, itemName) => { if (campaign.id) api.useItem(campaign.id, playerId, itemName).then(setCampaign).catch((caught) => setError(message(caught))); }}
            onSubmitAction={(playerId, text) => void submitAction(playerId, text)}
            onUnlockAction={(playerId) => void unlockAction(playerId)}
            onRest={(playerId, type) => void rest(playerId, type)}
            onGeneratePortrait={generatePortrait}
            onRetryCombat={() => void retryCombat()}
            onClearStage={() => setStageClear(null)}
            onCloseShop={() => setShopOpen(false)}
            onShopBuy={(playerId, itemId) => { if (campaign.id) void shopAction(() => api.buyItem(campaign.id!, playerId, itemId)); }}
            onShopSell={(playerId, itemName) => { if (campaign.id) void shopAction(() => api.sellItem(campaign.id!, playerId, itemName)); }}
            onShopForge={(playerId, kind, attackId) => { if (campaign.id) void shopAction(() => api.forgeUpgrade(campaign.id!, playerId, kind, attackId)); }}
            onCloseSpellModal={() => setSpellModal(null)}
            onCastSpell={(playerId, spell, asRitual, targetId) => void castSpell(playerId, spell, asRitual, targetId)}
          />
        )}
        {page === 'characters' && (
          <CharactersPage campaign={campaign} showStatHints={settings.showStatHints !== false} onCampaign={setCampaign} onError={setError} onNotice={setNotice} onGeneratePortrait={generatePortrait} />
        )}
        {page === 'journal' && (
          <JournalPage campaign={campaign} story={visibleStory} novelBusy={novelBusy} novelPlayerId={novelPlayerId} onNovelPlayerChange={setNovelPlayerId} onExportNovel={(playerId) => void exportNovel(playerId)} />
        )}
        {page === 'settings' && (
          <SettingsPage
            campaign={campaign}
            campaigns={campaigns}
            settings={settings}
            status={status}
            demoMode={demoMode}
            activeDmProvider={activeDmProvider}
            activeDmInfo={activeDmInfo}
            dmLabel={dmLabel}
            forgeDefaults={forgeDefaults}
            forgeSettings={forgeSettings}
            onToggleDemo={() => setDemoMode((value) => !value)}
            onProviderChange={(provider) => { updateSettings({ dmProvider: provider, selectedModel: '', selectedEffort: '' }); setCodexConn(null); }}
            onUpdateSettings={updateSettings}
            onUpdateForgeSettings={updateForgeSettings}
            onSwitchCampaign={(id) => void switchCampaign(id)}
            onNewCampaign={newCampaign}
            onDuplicateCampaign={() => void duplicateCurrentCampaign()}
            onImportCampaign={(raw) => void importFile(raw)}
            onDeleteCampaign={(id) => void removeCampaign(id)}
            onResetCampaign={() => void resetCampaign()}
          />
        )}
      </AnimatePresence>
      <footer><span>{campaign.scene}</span><span>{latestDm ? `最後裁定 ${latestDm.time}` : '等待第一個裁定'}</span></footer>
    </div></div>
  );
}
