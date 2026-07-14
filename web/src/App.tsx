import { useEffect, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { BookOpenText, ImageSquare, LockKey, MapTrifold, ShieldWarning, XCircle } from '@phosphor-icons/react';
import { initialCampaign, demoResponses } from './data';
import type { AiStatus, Campaign, CampaignSummary, CharacterSpell, CombatState, MessageAudience, Page, PlayerCharacter, PlayerId, RestType, StoryEntry } from './types';
import { Sidebar } from './components/Sidebar';
import { Topbar } from './components/Topbar';
import { StoryFeed } from './components/StoryFeed';
import { ActionComposer } from './components/ActionComposer';
import { CharacterPanel } from './components/CharacterPanel';
import { DiceTray } from './components/DiceTray';
import { MagneticButton } from './components/MagneticButton';
import { SceneVisual } from './components/SceneVisual';
import { PartySetup } from './components/PartySetup';
import { CombatTracker } from './components/CombatTracker';
import { CharacterManager } from './components/CharacterManager';
import { CampaignManager } from './components/CampaignManager';
import { changeResource, classNames, createLevel3Character, restCharacter, spendSpellSlot } from './rules/characters';
import { areAllActionsReady, createActionPayload } from './rules/party';
import { advanceTurn, syncPlayersFromCombat } from './rules/combat';
import { applyDmEffects, resolveShortRest, resolveSpellEffect, type DmEffect } from './rules/effects';
import { activateCampaign, addCampaign, duplicateCampaign, importCampaign, listCampaigns, loadActiveCampaign, saveActiveCampaign } from './campaign-storage';

function now() {
  return new Intl.DateTimeFormat('zh-TW', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date());
}

function makeEntry(speaker: StoryEntry['speaker'], text: string, audience: MessageAudience = 'public'): StoryEntry {
  return { id: `${Date.now()}-${crypto.randomUUID()}`, speaker, text, time: now(), audience };
}

function migrateCampaign(stored: Campaign): Campaign {
  const players = Array.isArray(stored.players) && stored.players.length > 0
    ? stored.players.slice(0, 4).map((player, index) => {
        const id = `player${index + 1}` as PlayerId;
        if (player.abilities && Array.isArray(player.resources) && Array.isArray(player.features)) {
          const baseClass = classNames.find((candidate) => String(player.className || '').includes(candidate)) || '戰士';
          return { ...player, id, classLevels: player.classLevels?.length ? player.classLevels : [{ className: baseClass, level: player.level || 3, subclass: player.subclass }] };
        }
        const className = classNames.find((candidate) => String(player.className || '').includes(candidate)) || '戰士';
        const migrated = createLevel3Character(id, String(player.name || `冒險者 ${index + 1}`), className);
        return { ...migrated, hp: Math.min(migrated.maxHp, Number(player.hp || migrated.maxHp)), classLevels: [{ className, level: 3, subclass: migrated.subclass }] };
      })
    : initialCampaign.players;
  return {
    ...initialCampaign,
    ...stored,
    schemaVersion: 3,
    setupComplete: stored.setupComplete === true,
    players,
    story: Array.isArray(stored.story) ? stored.story.map((entry) => ({ ...entry, audience: entry.audience || 'public' })) : initialCampaign.story,
    pending: stored.pending || {},
  };
}

function loadCampaign() {
  return migrateCampaign(loadActiveCampaign(initialCampaign));
}

export default function App() {
  const [campaign, setCampaign] = useState<Campaign>(loadCampaign);
  const [campaigns, setCampaigns] = useState<CampaignSummary[]>(() => listCampaigns(initialCampaign));
  const [page, setPage] = useState<Page>('table');
  const [status, setStatus] = useState<AiStatus | null>(null);
  const [viewer, setViewer] = useState<MessageAudience>('public');
  const [demoMode, setDemoMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [imageLoading, setImageLoading] = useState(false);
  const [imageError, setImageError] = useState('');

  useEffect(() => {
    saveActiveCampaign(campaign, initialCampaign);
    setCampaigns(listCampaigns(initialCampaign));
  }, [campaign]);

  useEffect(() => {
    const controller = new AbortController();
    fetch('/api/status', { signal: controller.signal }).then((response) => response.json()).then((data: AiStatus) => setStatus(data)).catch(() => setStatus({ connected: false, provider: 'Codex CLI', model: null, message: '本機伺服器未啟動' }));
    return () => controller.abort();
  }, []);

  const latestDm = useMemo(() => [...campaign.story].reverse().find((entry) => entry.speaker === 'dm' && (!entry.audience || entry.audience === 'public')), [campaign.story]);
  const visibleStory = useMemo(() => campaign.story.filter((entry) => !entry.audience || entry.audience === 'public' || entry.audience === viewer), [campaign.story, viewer]);

  function updatePlayer(updated: PlayerCharacter) {
    setCampaign((current) => ({ ...current, players: current.players.map((player) => player.id === updated.id ? updated : player) }));
  }

  function addLog(text: string, audience: MessageAudience = 'public') {
    setCampaign((current) => ({ ...current, story: [...current.story, makeEntry('system', text, audience)] }));
  }

  function changeClassResource(id: PlayerId, resourceId: string, delta: number) {
    setCampaign((current) => ({ ...current, players: current.players.map((player) => player.id === id ? changeResource(player, resourceId, delta) : player) }));
  }

  function castSpell(id: PlayerId, spell: CharacterSpell, asRitual: boolean, targetId?: string) {
    const player = campaign.players.find((entry) => entry.id === id);
    if (!player) return;
    if (spell.effect?.kind === 'damage' && !campaign.combat?.active) return setError('傷害法術必須在戰鬥追蹤器有有效目標時結算。');
    if (spell.effect && !targetId) return setError(`${spell.name} 需要選擇有效目標。`);
    const usedFreeClassCast = Boolean(spell.freeUseResourceId && player.resources.some((entry) => entry.id === spell.freeUseResourceId && entry.current > 0));
    const spent = spendSpellSlot(player, spell, asRitual);
    if (!spent) return setError(`${player.name} 沒有可用的 ${spell.level} 環法術位。`);
    const mode = asRitual ? '以儀式' : spell.level === 0 ? '施展戲法' : usedFreeClassCast ? '使用免費施法能力施放' : `消耗 ${spell.level} 環以上法術位施放`;
    setCampaign((current) => {
      const paidPlayers = current.players.map((entry) => entry.id === id ? spent : entry);
      const result = spell.effect && targetId ? resolveSpellEffect(paidPlayers, current.combat, id, targetId, spell.effect) : undefined;
      const combat = result?.combat && current.combat?.active && current.combat.combatants[current.combat.turnIndex]?.playerId === id
        ? advanceTurn(result.combat)
        : result?.combat || current.combat;
      const detail = result ? ` ${result.text}` : '';
      return { ...current, players: result?.players || paidPlayers, combat, story: [...current.story, makeEntry('system', `${player.name}${mode}「${spell.name}」。${detail}`)] };
    });
    setError('');
  }

  function rest(id: PlayerId, type: RestType) {
    const player = campaign.players.find((entry) => entry.id === id);
    if (!player) return;
    if (campaign.combat?.active) return setError('戰鬥進行中不能休息。');
    setCampaign((current) => {
      const recovered = restCharacter(player, type);
      const shortRest = type === 'short' ? resolveShortRest(recovered) : undefined;
      const updated = shortRest?.character || recovered;
      const detail = shortRest ? `，消耗 ${shortRest.diceSpent} 顆生命骰並恢復 ${shortRest.healed} HP` : '，生命、生命骰、法術位與職業資源已恢復';
      return { ...current, players: current.players.map((entry) => entry.id === id ? updated : entry), story: [...current.story, makeEntry('system', `${player.name}完成${type === 'short' ? '短休' : '長休'}${detail}。`)] };
    });
    setError('');
  }

  function changeCombat(combat: CombatState) {
    setCampaign((current) => ({ ...current, combat, players: syncPlayersFromCombat(current.players, combat) }));
  }

  async function generateImage() {
    if (!status?.connected || imageLoading) return;
    const narration = latestDm?.text;
    if (!narration) return setImageError('目前沒有可供繪製的公開 DM 場景敘事。');
    setImageLoading(true); setImageError('');
    try {
      const response = await fetch('/api/scene-image', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ campaign: { title: campaign.title, scene: campaign.scene }, narration, players: campaign.players }) });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || '場景插圖生成失敗');
      setCampaign((current) => ({ ...current, sceneImage: { url: data.url, scene: current.scene, createdAt: now(), model: data.model || status.imageModel || 'Codex Image' } }));
    } catch (caught) { setImageError(caught instanceof Error ? caught.message : String(caught)); } finally { setImageLoading(false); }
  }

  function submitAction(player: PlayerId, text: string) {
    if (loading || campaign.pending[player]) return;
    const nextPending = { ...campaign.pending, [player]: text };
    const nextStory = [...campaign.story, makeEntry(player, text)];
    setCampaign((current) => ({ ...current, pending: nextPending, story: nextStory }));
    if (areAllActionsReady(campaign.players, nextPending)) void advance(nextPending, nextStory);
  }

  async function advance(actions: Partial<Record<PlayerId, string>>, history: StoryEntry[]) {
    setLoading(true); setError('');
    try {
      let text: string;
      let nextScene: string | undefined;
      let privateMessages: Array<{ playerId: PlayerId; text: string }> = [];
      let effects: DmEffect[] = [];
      if (demoMode) {
        await new Promise((resolve) => window.setTimeout(resolve, 500));
        text = demoResponses[campaign.round % demoResponses.length];
      } else {
        const response = await fetch('/api/dm', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ model: campaign.selectedModel || '', actions: createActionPayload(campaign.players, actions), campaign: { title: campaign.title, scene: campaign.scene, round: campaign.round }, combat: campaign.combat, players: campaign.players, history }) });
        const data = await response.json();
        if (!response.ok) throw new Error(data.error || 'AI DM 無法回應');
        text = data.text;
        nextScene = typeof data.scene === 'string' ? data.scene : undefined;
        privateMessages = Array.isArray(data.privateMessages) ? data.privateMessages : [];
        effects = Array.isArray(data.effects) ? data.effects : [];
      }
      setCampaign((current) => {
        const settled = applyDmEffects(current.players, effects);
        const combat = current.combat ? { ...current.combat, combatants: current.combat.combatants.map((combatant) => {
          const player = combatant.playerId && settled.players.find((entry) => entry.id === combatant.playerId);
          return player ? { ...combatant, hp: player.hp, temporaryHp: player.temporaryHp, defeated: player.hp === 0 } : combatant;
        }) } : current.combat;
        return { ...current, scene: nextScene || current.scene, round: current.round + 1, pending: {}, players: settled.players, combat, story: [...current.story, makeEntry('dm', text), ...privateMessages.map((message) => makeEntry('dm', message.text, message.playerId)), ...settled.logs.map((entry) => makeEntry('system', `自動結算：${entry}`))] };
      });
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
      setCampaign((current) => ({ ...current, pending: {} }));
    } finally { setLoading(false); }
  }

  function resetCampaign() {
    setCampaign((current) => migrateCampaign({ ...structuredClone(initialCampaign), id: current.id, selectedModel: current.selectedModel }));
    setError(''); setPage('table');
  }

  function switchCampaign(id: string) {
    saveActiveCampaign(campaign, initialCampaign);
    const next = activateCampaign(id, initialCampaign);
    if (next) { setCampaign(migrateCampaign(next)); setViewer('public'); setPage(next.setupComplete ? 'table' : 'settings'); setNotice(`已載入「${next.title}」。`); }
  }

  function newCampaign() {
    saveActiveCampaign(campaign, initialCampaign);
    const next = addCampaign({ ...structuredClone(initialCampaign), setupComplete: false, id: undefined, story: [], pending: {} }, initialCampaign, true);
    setCampaign(migrateCampaign(next)); setPage('table'); setNotice('已建立新戰役；原戰役仍安全保存在戰役資料庫。');
  }

  function importFile(raw: string) {
    try { const imported = importCampaign(raw, initialCampaign); setCampaigns(listCampaigns(initialCampaign)); setNotice(`已匯入「${imported.title}」，目前戰役未切換。`); }
    catch (caught) { setError(caught instanceof Error ? caught.message : String(caught)); }
  }

  function completeSetup(setup: { title: string; players: PlayerCharacter[] }) {
    const names = setup.players.map((player) => player.name).join('、');
    setCampaign((current) => ({ ...current, setupComplete: true, title: setup.title, chapter: '第一章／沉鐘之夜', scene: '下城區・無燈禮拜堂', round: 1, objective: '在午夜鐘響前找到失蹤的製圖師伊薩克', players: setup.players, story: [makeEntry('dm', `禮拜堂的門在${names}身後闔上。沒有風，燭火卻同時朝祭壇偏斜；祭壇後方傳來三下緩慢的敲擊聲。`), makeEntry('system', `隊伍已建立，共 ${setup.players.length} 位冒險者。`)], pending: {}, combat: undefined }));
    setPage('table'); setError('');
  }

  function speakerLabel(entry: StoryEntry) {
    if (entry.speaker === 'dm') return entry.audience && entry.audience !== 'public' ? `地城主私訊 ${campaign.players.find((player) => player.id === entry.audience)?.name || entry.audience}` : '地城主';
    if (entry.speaker === 'system') return '紀錄';
    return campaign.players.find((player) => player.id === entry.speaker)?.name || '冒險者';
  }

  if (!campaign.setupComplete) return <PartySetup initialTitle={campaign.title} initialPlayers={campaign.players} onComplete={completeSetup} />;

  return (
    <div className="app-shell"><div className="grain" aria-hidden="true" /><Sidebar page={page} setPage={setPage} /><div className="workspace">
      <Topbar campaign={campaign} status={status} demoMode={demoMode} />
      {notice && <div className="notice-banner"><span>{notice}</span><button type="button" onClick={() => setNotice('')}><XCircle /></button></div>}
      <AnimatePresence mode="wait">
        {page === 'table' && <motion.main key="table" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="table-layout"><div className="table-main">
          <section className="scene-strip"><MapTrifold size={21} /><div><span>目前場景</span><strong>{campaign.scene}</strong></div><div className="round-mark"><span>回合</span><strong>{String(campaign.round).padStart(2, '0')}</strong></div></section>
          {status && !status.connected && !demoMode && <div className="model-notice"><ShieldWarning size={20} /><div><strong>本機 AI 尚未連線</strong><span>請先執行 codex login，或使用示範 DM。</span></div><MagneticButton variant="quiet" onClick={() => setDemoMode(true)}>使用示範 DM</MagneticButton></div>}
          {error && <div className="error-banner" role="alert"><XCircle size={19} /><div><strong>操作中斷</strong><span>{error}</span></div><button type="button" onClick={() => setError('')}><XCircle /></button></div>}
          <SceneVisual image={campaign.sceneImage} scene={campaign.scene} loading={imageLoading} error={imageError} canGenerate={Boolean(status?.connected)} onGenerate={generateImage} />
          <div className="viewer-switch"><LockKey /><span>訊息視角</span><select value={viewer} onChange={(event) => setViewer(event.target.value as MessageAudience)}><option value="public">公開訊息</option>{campaign.players.map((player) => <option key={player.id} value={player.id}>{player.name} 的私密訊息</option>)}</select></div>
          <StoryFeed story={campaign.story} players={campaign.players} loading={loading} viewer={viewer} />
          <div className={`composer-grid party-${campaign.players.length}`}>{campaign.players.map((player) => <ActionComposer key={player.id} player={player.id} name={player.name} className={player.className} pending={campaign.pending[player.id]} disabled={loading} partySize={campaign.players.length} onSubmit={submitAction} />)}</div>
        </div><aside className="table-rail"><section className="objective"><p className="eyebrow">當前目標</p><strong>{campaign.objective}</strong><span>{campaign.combat?.active ? `戰鬥第 ${campaign.combat.round} 輪` : '探索進行中'}</span></section>{campaign.players.map((player) => <CharacterPanel key={player.id} player={player} spellTargets={[...campaign.players.map((entry) => ({ id: entry.id, name: entry.name, side: 'party' as const })), ...(campaign.combat?.active ? campaign.combat.combatants.filter((entry) => entry.side === 'enemy' && !entry.defeated).map((entry) => ({ id: entry.id, name: entry.name, side: 'enemy' as const })) : [])]} onResourceChange={changeClassResource} onCastSpell={castSpell} onRest={rest} />)}<DiceTray players={campaign.players} onResult={addLog} /></aside></motion.main>}

        {page === 'combat' && <motion.main key="combat" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page"><div className="page-intro"><p className="eyebrow">Combat tracker</p><h2>先攻與自動結算</h2><p>依序執行回合，d20 命中 AC；自然 20 會加倍傷害骰，生命值同步回角色卡。</p></div><CombatTracker players={campaign.players} combat={campaign.combat} onChange={changeCombat} onLog={addLog} /></motion.main>}
        {page === 'characters' && <motion.div key="characters" initial={{ opacity: 0 }} animate={{ opacity: 1 }}><CharacterManager players={campaign.players} onUpdate={updatePlayer} onLog={addLog} /></motion.div>}
        {page === 'journal' && <motion.main key="journal" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page"><div className="page-intro"><p className="eyebrow">戰役記憶</p><h2>{campaign.title}</h2><p>公開與私密訊息都包含在本機存檔與匯出檔中。</p></div><div className="journal-list">{visibleStory.map((entry, index) => <article key={entry.id} className={entry.audience && entry.audience !== 'public' ? 'journal-private' : ''}><span>{String(index + 1).padStart(2, '0')}</span><div><small>{entry.time}／{speakerLabel(entry)}</small><p>{entry.text}</p></div></article>)}</div></motion.main>}
        {page === 'settings' && <motion.main key="settings" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page settings-page"><div className="page-intro"><p className="eyebrow">本機設定</p><h2>地城主與戰役</h2><p>所有切換操作都會先保存目前進度；匯入預設不切換。</p></div>
          <section className="settings-row"><div><strong>示範 DM</strong><span>完全不呼叫模型。</span></div><button type="button" className={`switch ${demoMode ? 'switch-on' : ''}`} onClick={() => setDemoMode((value) => !value)}><i /></button></section>
          <section className="settings-row model-selector"><div><strong>Codex 模型</strong><span>只影響之後的新 DM 請求；目前進度與既有訊息不會改變。</span></div><select value={campaign.selectedModel || ''} onChange={(event) => setCampaign((current) => ({ ...current, selectedModel: event.target.value }))}>{(status?.models || [{ id: '', label: 'Codex 預設（沿用目前設定）' }]).map((model) => <option key={model.id || 'default'} value={model.id}>{model.label}</option>)}</select></section>
          <section className="settings-row"><div><strong>Codex CLI</strong><span>{status?.connected ? `已登入／${status.model}` : status?.message || '正在檢查'}</span></div><ShieldWarning size={22} /></section>
          <section className="settings-row"><div><strong>場景圖片</strong><span>{status?.imageModel || 'Codex $imagegen'}，由玩家手動觸發。</span></div><ImageSquare size={22} /></section>
          <CampaignManager campaign={campaign} campaigns={campaigns} onSwitch={switchCampaign} onNew={newCampaign} onDuplicate={() => { const copy = duplicateCampaign(campaign, initialCampaign); setCampaigns(listCampaigns(initialCampaign)); setNotice(`已建立「${copy.title}」，目前戰役未切換。`); }} onImport={importFile} />
          <section className="settings-danger"><div><strong>重設目前戰役</strong><span>只重設目前選取的戰役，不影響其他存檔。</span></div><MagneticButton variant="quiet" onClick={resetCampaign}>重設目前戰役</MagneticButton></section>
        </motion.main>}
      </AnimatePresence>
      <footer><span>{campaign.scene}</span><span>{latestDm ? `最後裁定 ${latestDm.time}` : '等待第一個裁定'}</span></footer>
    </div></div>
  );
}
