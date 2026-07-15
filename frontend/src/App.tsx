import { lazy, Suspense, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { BookOpenText, Compass, Lightbulb, LockKey, MapTrifold, ShieldWarning, Sword, XCircle } from '@phosphor-icons/react';
import { initialCampaign, demoResponses } from './data';
import type { AiStatus, Campaign, CampaignSummary, CharacterSpell, CombatState, Combatant, MessageAudience, Page, PlayerCharacter, PlayerId, RequiredCheck, RestType, StoryEntry } from './types';
import { Sidebar } from './components/Sidebar';
import { Topbar } from './components/Topbar';
import { StoryFeed } from './components/StoryFeed';
import { CharacterPanel } from './components/CharacterPanel';
import { DiceTray } from './components/DiceTray';
import type { DiceRollResult } from './components/DiceTray';
import { MagneticButton } from './components/MagneticButton';
import { SceneVisual } from './components/SceneVisual';
import { PartySetup } from './components/PartySetup';
import { CombatTracker } from './components/CombatTracker';
import { CampaignManager } from './components/CampaignManager';
import { abilityLabels, changeResource, classNames, createLevel3Character, restCharacter, spendSpellSlot } from './rules/characters';
import { areAllActionsReady, createActionPayload } from './rules/party';
import { combatResourceForCastingTime, partyCombatants, spendCombatResource, startCombat, syncPlayersFromCombat } from './rules/combat';
import { applyDmEffects, resolveShortRest, resolveSpellEffect, type DmEffect } from './rules/effects';
import { experienceForLevel, experienceToNextLevel, grantExperience } from './rules/advancement';
import { spellCatalog } from './rules/spells';
import { activateCampaign, addCampaign, duplicateCampaign, importCampaign, listCampaigns, loadActiveCampaign, saveActiveCampaign } from './campaign-storage';

const CharacterManager = lazy(() => import('./components/CharacterManager').then((module) => ({ default: module.CharacterManager })));

interface CombatConclusion {
  outcome: 'victory' | 'defeat' | 'withdrawal';
  summary: string;
}

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
          const spellcasting = player.spellcasting ? { ...player.spellcasting, spells: player.spellcasting.spells.map((spell) => ({ ...spellCatalog[spell.id], ...spell, effect: spellCatalog[spell.id]?.effect || spell.effect })) } : undefined;
          return { ...player, id, spellcasting, experience: Number.isFinite(player.experience) ? player.experience : experienceForLevel(player.level || 3), abilityPoints: Math.max(0, Number(player.abilityPoints || 0)), classLevels: player.classLevels?.length ? player.classLevels : [{ className: baseClass, level: player.level || 3, subclass: player.subclass }] };
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
    choices: Array.isArray(stored.choices) ? stored.choices : [],
    requiredCheck: stored.requiredCheck || null,
    sceneImages: Array.isArray(stored.sceneImages) ? stored.sceneImages : stored.sceneImage ? [stored.sceneImage] : [],
    fontScale: Math.max(.85, Math.min(1.25, Number(stored.fontScale || 1))),
    showStatHints: stored.showStatHints !== false,
    dismissedTips: Array.isArray(stored.dismissedTips) ? stored.dismissedTips : [],
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
  const [spellRoll, setSpellRoll] = useState<{ check: RequiredCheck; casterId: PlayerId; spell: CharacterSpell; asRitual: boolean; targetId: string } | null>(null);

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
  const appStyle = { '--font-scale': String(campaign.fontScale || 1) } as CSSProperties;
  const activeRequiredCheck = spellRoll?.check || campaign.requiredCheck;
  const currentCombatant = campaign.combat?.active ? campaign.combat.combatants[campaign.combat.turnIndex] : undefined;
  const contextualTip = [
    activeRequiredCheck && { id: 'required-roll', title: '先完成必要擲骰', text: '骰盤已自動選好 d20、角色加值與目標 DC。按下擲骰後，結果會寫入故事紀錄並解鎖下一步。' },
    currentCombatant && { id: currentCombatant.side === 'enemy' ? 'enemy-turn' : 'combat-turn', title: currentCombatant.side === 'enemy' ? '現在是怪獸回合' : `現在是 ${currentCombatant.name} 的回合`, text: currentCombatant.side === 'enemy' ? '在戰鬥區替怪獸選擇一名玩家作為目標，按「攻擊並結算」；系統會自動判斷命中與傷害。' : '在戰鬥區選擇攻擊或法術與目標。完成後系統會自動移到下一位。' },
    campaign.players.some((player) => experienceToNextLevel(player).ready) && { id: 'level-ready', title: '有角色可以升級', text: '前往「角色」頁選擇原職業升級或其他職業多職業；生命、熟練與法術位會自動更新。', page: 'characters' as Page },
    campaign.round <= 1 && { id: 'first-action', title: '如何進行一回合', text: '每位玩家可點選 DM 建議或輸入自己的做法，再鎖定行動；全員完成後故事才會推進，最後一人送出前都能修改。' },
    !campaign.sceneImage && { id: 'scene-image', title: '場景圖是選用功能', text: '可按「生成場景」繪製目前畫面；也能在設定開啟每回合自動生成。圖片不影響規則判定。' },
  ].filter(Boolean).find((tip) => tip && !(campaign.dismissedTips || []).includes(tip.id)) as { id: string; title: string; text: string; page?: Page } | undefined;

  function dismissTip(id: string) {
    setCampaign((current) => ({ ...current, dismissedTips: [...new Set([...(current.dismissedTips || []), id])] }));
  }

  function updatePlayer(updated: PlayerCharacter) {
    setCampaign((current) => ({ ...current, players: current.players.map((player) => player.id === updated.id ? updated : player) }));
  }

  function addLog(text: string, audience: MessageAudience = 'public') {
    setCampaign((current) => ({ ...current, story: [...current.story, makeEntry('system', text, audience)] }));
  }

  function changeClassResource(id: PlayerId, resourceId: string, delta: number) {
    setCampaign((current) => ({ ...current, players: current.players.map((player) => player.id === id ? changeResource(player, resourceId, delta) : player) }));
  }

  function applySpellCast(id: PlayerId, spell: CharacterSpell, asRitual: boolean, targetId?: string, attackTotal?: number) {
    const player = campaign.players.find((entry) => entry.id === id);
    if (!player) return;
    const usedFreeClassCast = Boolean(spell.freeUseResourceId && player.resources.some((entry) => entry.id === spell.freeUseResourceId && entry.current > 0));
    const spent = spendSpellSlot(player, spell, asRitual);
    if (!spent) return setError(`${player.name} 沒有可用的 ${spell.level} 環法術位。`);
    const mode = asRitual ? '以儀式' : spell.level === 0 ? '施展戲法' : usedFreeClassCast ? '使用免費施法能力施放' : `消耗 ${spell.level} 環以上法術位施放`;
    setCampaign((current) => {
      const paidPlayers = current.players.map((entry) => entry.id === id ? spent : entry);
      const result = spell.effect && targetId ? resolveSpellEffect(paidPlayers, current.combat, id, targetId, spell.effect, Math.random, attackTotal === undefined ? undefined : { attackTotal }) : undefined;
      let combat = result?.combat || current.combat;
      if (combat?.active) combat = spendCombatResource(combat, id, combatResourceForCastingTime(spell.castingTime));
      const detail = result ? ` ${result.text}` : '';
      return { ...current, players: result?.players || paidPlayers, combat, story: [...current.story, makeEntry('system', `${player.name}${mode}「${spell.name}」。${detail}`)] };
    });
    const targetName = targetId === 'scene' ? '目前場景' : [...campaign.players, ...(campaign.combat?.combatants || [])].find((entry) => entry.id === targetId)?.name || targetId || '自身';
    if (!campaign.combat?.active) window.queueMicrotask(() => submitAction(id, `對${targetName}施放「${spell.name}」`));
    setError('');
  }

  function castSpell(id: PlayerId, spell: CharacterSpell, asRitual: boolean, targetId?: string) {
    if (activeRequiredCheck) return setError('請先完成目前畫面上的必要擲骰，再進行施法。');
    if (campaign.pending[id]) return setError(`${campaign.players.find((entry) => entry.id === id)?.name || '這名角色'}本回合已鎖定行動，請先解鎖才能改為施法。`);
    const player = campaign.players.find((entry) => entry.id === id);
    if (!player) return;
    if (spell.effect?.kind === 'damage' && !campaign.combat?.active) return setError('傷害法術必須在戰鬥追蹤器有有效目標時結算。');
    if (!targetId) return setError(`${spell.name} 必須先指定目標。`);
    if (campaign.combat?.active) {
      try { spendCombatResource(campaign.combat, id, combatResourceForCastingTime(spell.castingTime)); }
      catch (caught) { return setError(caught instanceof Error ? caught.message : String(caught)); }
    }
    const usedFreeClassCast = Boolean(spell.freeUseResourceId && player.resources.some((entry) => entry.id === spell.freeUseResourceId && entry.current > 0));
    const cost = asRitual || spell.level === 0 ? '不消耗法術位' : usedFreeClassCast ? '消耗一次免費施法能力' : `消耗 ${spell.level} 環以上法術位`;
    const targetName = targetId ? [...campaign.players, ...(campaign.combat?.combatants || [])].find((entry) => entry.id === targetId)?.name || targetId : '';
    if (!window.confirm(`確認由 ${player.name} 施放「${spell.name}」？\n${cost}${targetName ? `，目標：${targetName}` : ''}`)) return;
    if (!spendSpellSlot(player, spell, asRitual)) return setError(`${player.name} 沒有可用的 ${spell.level} 環法術位。`);
    if (spell.effect?.attackRoll && targetId) {
      const target = campaign.combat?.combatants.find((entry) => entry.id === targetId || entry.playerId === targetId);
      if (!target) return setError(`${spell.name} 找不到可供法術攻擊的目標。`);
      const modifier = player.spellcasting?.attackBonus || 0;
      setSpellRoll({
        casterId: id, spell, asRitual, targetId,
        check: { character: player.name, ability: abilityLabels[player.spellcasting?.ability || 'int'], skill: `${spell.name}法術攻擊`, dc: target.ac, modifier, reason: `以法術攻擊命中 ${target.name}（AC ${target.ac}）後才會結算效果。` },
      });
      setError('');
      return;
    }
    applySpellCast(id, spell, asRitual, targetId);
  }

  function rest(id: PlayerId, type: RestType) {
    const player = campaign.players.find((entry) => entry.id === id);
    if (!player) return;
    if (campaign.combat?.active) return setError('戰鬥進行中不能休息。');
    if (activeRequiredCheck) return setError('請先完成目前的必要擲骰，再決定是否休息。');
    if (Object.keys(campaign.pending).length > 0) return setError('隊伍已有待裁定行動；請先完成或解鎖本輪行動，再開始休息。');
    const shortResources = player.resources.filter((resource) => resource.shortRestRecovery === 'all' || Number(resource.shortRestRecovery) > 0).map((resource) => resource.name);
    const explanation = type === 'short'
      ? `短休消耗 1 點探索行動時間（約一小時）。會自動花費生命骰直到生命全滿或生命骰用盡，並恢復短休資源${shortResources.length ? `（${shortResources.join('、')}）` : ''}${player.spellcasting?.mode === 'pact' ? '及契約法術位' : ''}；一般法術位不恢復。`
      : '長休消耗 4 點探索行動時間（約八小時）。會恢復全部生命、生命骰、法術位與職業資源，結束專注並將可恢復狀態重設。';
    if (!window.confirm(`確認讓 ${player.name}進行${type === 'short' ? '短休' : '長休'}？\n${explanation}`)) return;
    setCampaign((current) => {
      const recovered = restCharacter(player, type);
      const shortRest = type === 'short' ? resolveShortRest(recovered) : undefined;
      const updated = shortRest?.character || recovered;
      const detail = shortRest ? `，消耗 ${shortRest.diceSpent} 顆 d${player.hitDie} 生命骰、恢復 ${shortRest.healed} HP；短休資源已恢復，一般法術位不恢復` : '，生命、生命骰、法術位與職業資源已完全恢復，專注已結束';
      const actionCost = type === 'short' ? 1 : 4;
      return { ...current, round: current.round + actionCost, players: current.players.map((entry) => entry.id === id ? updated : entry), story: [...current.story, makeEntry('system', `${player.name}完成${type === 'short' ? '短休' : '長休'}（消耗 ${actionCost} 點探索行動時間）${detail}。`)] };
    });
    setError('');
  }

  function changeCombat(combat: CombatState) {
    setCampaign((current) => {
      const previousEnemies = current.combat?.combatants.filter((entry) => entry.side === 'enemy') || [];
      const nextEnemies = combat.combatants.filter((entry) => entry.side === 'enemy');
      const newlyWon = nextEnemies.length > 0 && nextEnemies.every((entry) => entry.defeated) && previousEnemies.some((entry) => !entry.defeated);
      let players = syncPlayersFromCombat(current.players, combat);
      if (!newlyWon) return { ...current, combat, players };
      const reward = Math.max(50, Math.ceil(nextEnemies.reduce((sum, enemy) => sum + enemy.maxHp * 10, 0) / Math.max(1, players.length)));
      players = players.map((player) => grantExperience(player, reward));
      return { ...current, combat, players, story: [...current.story, makeEntry('system', `戰鬥勝利：每位角色獲得 ${reward} XP。`)] };
    });
  }

  function endCombat(combat: CombatState) {
    if (loading) return;
    const enemies = combat.combatants.filter((entry) => entry.side === 'enemy');
    const party = combat.combatants.filter((entry) => entry.side === 'party');
    const outcome: CombatConclusion['outcome'] = enemies.length > 0 && enemies.every((entry) => entry.defeated)
      ? 'victory'
      : party.length > 0 && party.every((entry) => entry.defeated)
        ? 'defeat'
        : 'withdrawal';
    const summary = combat.combatants.map((entry) => `${entry.name}：${entry.hp}/${entry.maxHp} HP${entry.defeated ? '（失去戰鬥能力）' : ''}`).join('；');
    const resultLabel = outcome === 'victory' ? '隊伍勝利' : outcome === 'defeat' ? '隊伍戰敗' : '戰鬥中止或撤退';
    const log = makeEntry('system', `戰鬥結束：${resultLabel}。${summary}`);
    setCampaign((current) => ({ ...current, combat: { ...combat, active: false }, players: syncPlayersFromCombat(current.players, combat), story: [...current.story, log] }));
    void advance({}, [...campaign.story, log], undefined, { outcome, summary });
  }

  const localImages = (campaign.imageBackend || status?.imageBackend) === 'local';
  const canGenerateImages = Boolean(status?.connected) || localImages;

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
      ttsAudioRef.current = audio;
      void audio.play();
    } catch (caught) {
      setNotice(`語音朗讀失敗：${caught instanceof Error ? caught.message : String(caught)}`);
    }
  }

  async function generateImage(narrationOverride?: string) {
    if (!canGenerateImages || imageLoading) return;
    const narration = narrationOverride || latestDm?.text;
    if (!narration) return setImageError('目前沒有可供繪製的公開 DM 場景敘事。');
    setImageLoading(true); setImageError('');
    try {
      const response = await fetch('/api/scene-image', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ imageBackend: campaign.imageBackend || '', campaign: { title: campaign.title, scene: campaign.scene }, narration, players: campaign.players }) });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || '場景插圖生成失敗');
      setCampaign((current) => {
        const image = { url: data.url, scene: current.scene, createdAt: now(), model: data.model || status?.imageModel || 'Codex Image' };
        return { ...current, sceneImage: image, sceneImages: [...(current.sceneImages || []), image].slice(-24) };
      });
    } catch (caught) { setImageError(caught instanceof Error ? caught.message : String(caught)); } finally { setImageLoading(false); }
  }

  async function generatePortrait(player: PlayerCharacter, appearance: string) {
    if (!canGenerateImages) return setError('圖片服務尚未連線。');
    const description = appearance.trim();
    if (!description) return setError('請先輸入角色外觀描述。');
    try {
      const response = await fetch('/api/character-image', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ imageBackend: campaign.imageBackend || '', name: player.name, species: player.species, className: player.className, background: player.background, appearance: description }) });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || '角色圖片生成失敗');
      updatePlayer({ ...player, appearance: description, portraitUrl: data.url });
      addLog(`${player.name}的角色外觀與肖像已更新。`);
    } catch (caught) { setError(caught instanceof Error ? caught.message : String(caught)); }
  }

  function submitAction(player: PlayerId, text: string) {
    if (loading || campaign.pending[player]) return;
    const nextPending: Partial<Record<PlayerId, string>> = { ...campaign.pending, [player]: text };
    setCampaign((current) => ({ ...current, pending: nextPending }));
    if (areAllActionsReady(campaign.players, nextPending)) {
      const actionEntries = campaign.players.map((entry) => makeEntry(entry.id, nextPending[entry.id] || '本回合不行動，保持警戒。'));
      void advance(nextPending, [...campaign.story, ...actionEntries]);
    }
  }

  function unlockAction(player: PlayerId) {
    if (loading) return;
    setCampaign((current) => { const pending = { ...current.pending }; delete pending[player]; return { ...current, pending }; });
  }

  async function advance(actions: Partial<Record<PlayerId, string>>, history: StoryEntry[], resolution?: RequiredCheck & DiceRollResult, combatConclusion?: CombatConclusion) {
    const isContinuation = Boolean(resolution || combatConclusion);
    setLoading(true); setError('');
    if (resolution) setCampaign((current) => ({ ...current, requiredCheck: null }));
    try {
      let text: string;
      let nextScene: string | undefined;
      let nextObjective: string | undefined;
      let nextObjectiveContext: string | undefined;
      let nextStakes: string | undefined;
      let privateMessages: Array<{ playerId: PlayerId; text: string }> = [];
      let effects: DmEffect[] = [];
      let experienceAwards: Array<{ playerId: PlayerId; amount: number; reason: string }> = [];
      let choices: string[] = [];
      let requiredCheck: Campaign['requiredCheck'] = null;
      let combatStart: { starts: boolean; firstTurn?: 'initiative' | 'enemy'; enemies: Array<Omit<Combatant, 'id' | 'side' | 'initiative' | 'maxHp' | 'defeated'>> } | undefined;
      if (demoMode && combatConclusion) {
        await new Promise((resolve) => window.setTimeout(resolve, 350));
        text = combatConclusion.outcome === 'victory'
          ? '最後的兵刃聲落下，敵人的抵抗徹底停止。倖存者的喘息與戰場殘留的痕跡，讓隊伍看清這場衝突留下的代價，也暴露出下一步可追查的線索。戰鬥已經結束，接下來可以先確認傷勢與現場，再決定去向。'
          : combatConclusion.outcome === 'defeat'
            ? '戰線徹底崩解，眾人已無法繼續正面抵抗。敵人掌握了現場，但故事並未在此終止；倖存者、俘虜或外力將決定隊伍必須面對的新局勢。'
            : '雙方脫離交戰距離，兵刃聲逐漸被急促呼吸取代。這場戰鬥沒有以徹底勝負收場，撤退路線、傷勢與敵人的下一步成了眼前最迫切的問題。';
        choices = combatConclusion.outcome === 'victory' ? ['檢查戰場與敵人', '先救治傷者', '繼續追查線索'] : ['確認隊伍傷勢', '尋找安全退路', '觀察敵人動向'];
      } else if (demoMode && resolution) {
        await new Promise((resolve) => window.setTimeout(resolve, 350));
        text = resolution.success
          ? `${resolution.character}的${resolution.ability}（${resolution.skill}）檢定成功。${resolution.reason}的風險被穩穩克服，眼前的阻礙讓開，並露出足以讓隊伍繼續判斷的新線索。局勢已向前推進，不需要重複宣告剛才的行動。`
          : `${resolution.character}的${resolution.ability}（${resolution.skill}）檢定失敗。${resolution.reason}帶來了立即而具體的代價，但故事沒有停住；隊伍仍可依照眼前出現的新局勢決定下一步。`;
        choices = resolution.success ? ['檢查新出現的線索', '趁局勢有利繼續前進'] : ['處理失敗造成的後果', '改用另一條路徑'];
      } else if (demoMode) {
        await new Promise((resolve) => window.setTimeout(resolve, 500));
        const demoIndex = (campaign.round - 1) % demoResponses.length;
        text = demoResponses[demoIndex];
        choices = demoIndex === 0 ? ['尋找祭壇機關', '移動祭壇', '先觀察泥痕'] : demoIndex === 1 ? ['搶救燃燒的地圖', '追趕灰袍人', '封鎖鐘塔出口'] : ['由斥候先下階梯', '檢查羅盤的魔法', '在入口設置防線'];
        requiredCheck = demoIndex === 0 ? { character: campaign.players[0]?.name || '冒險者', ability: '力量', skill: '運動', dc: 13, reason: '祭壇沉重且卡在石槽中，強行移動有失手風險。' } : null;
        nextObjective = demoIndex === 0 ? '找出祭壇下方敲擊聲的來源' : demoIndex === 1 ? '在地圖燒毀前取得線索，或攔下逃往鐘塔的灰袍人' : '沿暗門階梯尋找伊薩克並確認下方威脅';
        nextObjectiveContext = demoIndex === 0 ? '伊薩克最後的地圖指向這座禮拜堂；新鮮泥痕與河底淤泥的氣味顯示，有東西剛從祭壇附近被拖入地下。' : demoIndex === 1 ? '藏在壁龕後的灰袍人攜帶伊薩克的染血地圖，暴露後焚毀證據並逃往鐘塔；樓梯間另有鎖鏈聲逼近。' : '祭壇下的暗門已開啟，伊薩克的羅盤留在第一級階梯，指針卻異常指向隊伍中央；下方同時傳來潮水與呼吸聲。';
        nextStakes = demoIndex === 0 ? '若拖延到午夜，地下漲潮可能淹沒入口與伊薩克留下的線索。' : demoIndex === 1 ? '地圖即將燒毀，灰袍人也可能敲響鐘聲召來更多守衛。' : '狹窄階梯不利撤退；若沒有先安排隊形，隊伍可能在黑暗中被分割。';
        experienceAwards = campaign.players.map((player) => ({ playerId: player.id, amount: 75, reason: '推進禮拜堂調查並取得新線索' }));
      } else {
        const response = await fetch('/api/dm', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ model: campaign.selectedModel || '', effort: campaign.selectedEffort || '', actions: isContinuation ? [] : createActionPayload(campaign.players, actions), resolution, combatConclusion, campaign: { title: campaign.title, scene: campaign.scene, objective: campaign.objective, objectiveContext: campaign.objectiveContext, stakes: campaign.stakes, round: campaign.round }, combat: combatConclusion && campaign.combat ? { ...campaign.combat, active: false } : campaign.combat, players: campaign.players, history }) });
        const data = await response.json();
        if (!response.ok) throw new Error(data.error || 'AI DM 無法回應');
        text = data.text;
        nextScene = typeof data.scene === 'string' ? data.scene : undefined;
        nextObjective = typeof data.objective === 'string' ? data.objective : undefined;
        nextObjectiveContext = typeof data.objectiveContext === 'string' ? data.objectiveContext : undefined;
        nextStakes = typeof data.stakes === 'string' ? data.stakes : undefined;
        privateMessages = Array.isArray(data.privateMessages) ? data.privateMessages : [];
        effects = !combatConclusion && Array.isArray(data.effects) ? data.effects : [];
        experienceAwards = !combatConclusion && Array.isArray(data.experienceAwards) ? data.experienceAwards : [];
        choices = Array.isArray(data.choices) ? data.choices.slice(0, 3) : [];
        requiredCheck = data.requiresCheck && data.check ? data.check : null;
        combatStart = !combatConclusion && data.combat?.starts && Array.isArray(data.combat.enemies) ? data.combat : undefined;
        const actionIssues = Array.isArray(data.actionIssues) ? data.actionIssues.filter((entry: { playerId?: string; message?: string }) => /^player[1-4]$/.test(entry.playerId || '') && entry.message) : [];
        if (actionIssues.length > 0) {
          if (isContinuation) throw new Error(combatConclusion ? 'DM 未直接接續戰鬥結果。' : 'DM 未直接接續檢定結果，請重新擲骰後再試一次。');
          setCampaign((current) => {
            const pending = { ...current.pending };
            actionIssues.forEach((issue: { playerId: PlayerId }) => delete pending[issue.playerId]);
            const details = actionIssues.map((issue: { playerId: PlayerId; message: string }) => `${current.players.find((player) => player.id === issue.playerId)?.name || issue.playerId}：${issue.message}`).join('；');
            return { ...current, pending, story: [...current.story, makeEntry('dm', `【行動駁回】${details}。故事尚未推進；請依照理由修改後重新鎖定。`)] };
          });
          setError('');
          return;
        }
      }
      setCampaign((current) => {
        const settled = applyDmEffects(current.players, effects);
        const awardByPlayer = new Map(experienceAwards.map((award) => [award.playerId, award]));
        const experiencedPlayers = settled.players.map((player) => {
          const award = awardByPlayer.get(player.id);
          return award ? grantExperience(player, award.amount) : player;
        });
        let combat = current.combat ? { ...current.combat, combatants: current.combat.combatants.map((combatant) => {
          const player = combatant.playerId && experiencedPlayers.find((entry) => entry.id === combatant.playerId);
          return player ? { ...combatant, hp: player.hp, temporaryHp: player.temporaryHp, defeated: player.hp === 0 } : combatant;
        }) } : current.combat;
        if (combatStart?.starts && !combat?.active) {
          const enemies: Combatant[] = combatStart.enemies.map((enemy) => ({ ...enemy, id: `enemy-${crypto.randomUUID()}`, side: 'enemy', initiative: 0, maxHp: enemy.hp, defeated: false }));
          combat = startCombat([...partyCombatants(experiencedPlayers), ...enemies], Math.random, combatStart.firstTurn);
        }
        const actionEntries = isContinuation ? [] : current.players.map((entry) => makeEntry(entry.id, actions[entry.id] || '本回合不行動，保持警戒。'));
        const experienceLogs = experienceAwards.filter((award) => award.amount > 0).map((award) => makeEntry('system', `${experiencedPlayers.find((player) => player.id === award.playerId)?.name || award.playerId}獲得 ${award.amount} XP：${award.reason}`));
        return { ...current, scene: nextScene || current.scene, objective: nextObjective || current.objective, objectiveContext: nextObjectiveContext || current.objectiveContext, stakes: nextStakes || current.stakes, round: current.round + (isContinuation ? 0 : 1), pending: {}, choices, requiredCheck, players: experiencedPlayers, combat, story: [...current.story, ...actionEntries, makeEntry('dm', text), ...privateMessages.map((message) => makeEntry('dm', message.text, message.playerId)), ...settled.logs.map((entry) => makeEntry('system', `自動結算：${entry}`)), ...experienceLogs] };
      });
      if ((campaign.autoSceneImages || combatStart?.starts) && text) void generateImage(text);
      if (campaign.ttsEnabled && text) void speakNarration(text);
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : String(caught);
      if (resolution) {
        const { character, ability, skill, dc, reason, modifier } = resolution;
        setCampaign((current) => ({ ...current, requiredCheck: { character, ability, skill, dc, reason, modifier } }));
        setError(`${message}\n檢定結果尚未推進故事，必要骰盤已恢復，請再試一次。`);
      } else if (combatConclusion) {
        setCampaign((current) => ({ ...current, combat: current.combat ? { ...current.combat, active: true } : current.combat }));
        setError(`${message}\n戰後敘述尚未完成，戰鬥介面已恢復；請再次按「結束戰鬥並敘述」。`);
      } else {
        setError(`${message}\n請修改或補充行動後重新提交；已鎖定內容仍保留。`);
      }
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
    setCampaign((current) => ({ ...current, setupComplete: true, title: setup.title, chapter: '第一章／沉鐘之夜', scene: '下城區・無燈禮拜堂', round: 1, objective: '在午夜鐘響前找到失蹤的製圖師伊薩克', objectiveContext: '製圖師伊薩克在調查下城區失蹤事件後失去音訊。他最後留下的地圖指向無燈禮拜堂，而祭壇下方傳來不自然的敲擊聲。', stakes: '午夜鐘響後，地下水道會漲潮，伊薩克留下的線索可能被淹沒，失蹤者也將更難救回。', players: setup.players, story: [makeEntry('dm', `禮拜堂的門在${names}身後闔上。沒有風，燭火卻同時朝祭壇偏斜；祭壇後方傳來三下緩慢的敲擊聲。`), makeEntry('system', `隊伍已建立，共 ${setup.players.length} 位冒險者。`)], pending: {}, combat: undefined }));
    setPage('table'); setError('');
  }

  function speakerLabel(entry: StoryEntry) {
    if (entry.speaker === 'dm') return entry.audience && entry.audience !== 'public' ? `地城主私訊 ${campaign.players.find((player) => player.id === entry.audience)?.name || entry.audience}` : '地城主';
    if (entry.speaker === 'system') return '紀錄';
    return campaign.players.find((player) => player.id === entry.speaker)?.name || '冒險者';
  }

  if (!campaign.setupComplete) return <PartySetup initialTitle={campaign.title} initialPlayers={campaign.players} onComplete={completeSetup} />;

  return (
    <div className="app-shell" style={appStyle}><div className="grain" aria-hidden="true" /><Sidebar page={page} setPage={setPage} /><div className="workspace">
      <Topbar campaign={campaign} status={status} demoMode={demoMode} />
      {notice && <div className="notice-banner"><span>{notice}</span><button type="button" onClick={() => setNotice('')}><XCircle /></button></div>}
      {contextualTip && <aside className="novice-tip" aria-label="新手提示"><Lightbulb size={20} /><div><strong>{contextualTip.title}</strong><span>{contextualTip.text}</span></div>{contextualTip.page && <button type="button" className="tip-action" onClick={() => setPage(contextualTip.page!)}>前往查看</button>}<button type="button" className="tip-dismiss" aria-label="關閉提示" onClick={() => dismissTip(contextualTip.id)}><XCircle /></button></aside>}
      <AnimatePresence mode="wait">
        {page === 'table' && <motion.main key="table" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="table-layout"><div className="table-main">
          <section className="scene-strip"><MapTrifold size={21} /><div><span>目前場景</span><strong>{campaign.scene}</strong></div><div className={`game-state ${campaign.combat?.active ? 'game-state-combat' : 'game-state-exploration'}`}>{campaign.combat?.active ? <Sword size={17} weight="fill" /> : <Compass size={17} />}<div><span>遊戲狀態</span><strong>{campaign.combat?.active ? `戰鬥中${currentCombatant ? `・${currentCombatant.name} 行動` : ''}` : '探索中'}</strong></div></div><div className="round-mark"><span>{campaign.combat?.active ? '戰鬥輪' : '探索回合'}</span><strong>{String(campaign.combat?.active ? campaign.combat.round : campaign.round).padStart(2, '0')}</strong></div></section>
          {status && !status.connected && !demoMode && <div className="model-notice"><ShieldWarning size={20} /><div><strong>本機 AI 尚未連線</strong><span>請先執行 codex login，或使用示範 DM。</span></div><MagneticButton variant="quiet" onClick={() => setDemoMode(true)}>使用示範 DM</MagneticButton></div>}
          {error && <div className="error-banner" role="alert"><XCircle size={19} /><div><strong>操作中斷</strong><span>{error}</span></div><button type="button" onClick={() => setError('')}><XCircle /></button></div>}
          <SceneVisual image={campaign.sceneImage} images={campaign.sceneImages} scene={campaign.scene} loading={imageLoading} error={imageError} canGenerate={canGenerateImages} onGenerate={() => void generateImage()} onSelect={(image) => setCampaign((current) => ({ ...current, sceneImage: image }))} />
          <div className="viewer-switch"><LockKey /><span>訊息視角</span><select value={viewer} onChange={(event) => setViewer(event.target.value as MessageAudience)}><option value="public">公開訊息</option>{campaign.players.map((player) => <option key={player.id} value={player.id}>{player.name} 的私密訊息</option>)}</select></div>
          <StoryFeed story={campaign.story} players={campaign.players} loading={loading} viewer={viewer} />
          {activeRequiredCheck && <DiceTray players={campaign.players} requiredCheck={activeRequiredCheck} onRoll={({ total }) => { if (spellRoll) applySpellCast(spellRoll.casterId, spellRoll.spell, spellRoll.asRitual, spellRoll.targetId, total); }} onRequiredRoll={(roll) => { if (spellRoll) { setSpellRoll(null); return; } const check = campaign.requiredCheck; if (check) void advance({}, [...campaign.story, makeEntry('system', roll.text)], { ...check, ...roll }); }} onResult={addLog} />}
          {campaign.combat?.active && <section className="inline-combat"><div className="section-heading"><div><p className="eyebrow">戰鬥進行中</p><h2>先攻與回合操作</h2></div></div><CombatTracker players={campaign.players} combat={campaign.combat} onChange={changeCombat} onEnd={endCombat} onLog={addLog} /></section>}
          <div className={`composer-grid party-${campaign.players.length}`}>{campaign.players.map((player) => <section className="player-console" key={player.id} aria-label={`${player.name}玩家操作區`}><CharacterPanel player={player} showStatHints={campaign.showStatHints !== false} combatActive={campaign.combat?.active === true} pending={campaign.pending[player.id]} actionDisabled={loading || Boolean(activeRequiredCheck)} partySize={campaign.players.length} choices={campaign.choices} resourceSummary={player.spellcasting ? player.spellcasting.slots.map((slot) => `${slot.level}環 ${slot.current}/${slot.max}`).join('、') : player.resources.slice(0, 3).map((resource) => `${resource.name} ${resource.current}/${resource.max}`).join('、')} onSubmitAction={submitAction} onUnlockAction={unlockAction} spellTargets={[...campaign.players.map((entry) => ({ id: entry.id, name: entry.name, side: 'party' as const })), ...(campaign.combat?.active ? campaign.combat.combatants.filter((entry) => entry.side === 'enemy' && !entry.defeated).map((entry) => ({ id: entry.id, name: entry.name, side: 'enemy' as const })) : [])]} onResourceChange={changeClassResource} onCastSpell={castSpell} onRest={rest} onGeneratePortrait={generatePortrait} /></section>)}</div>
        </div><aside className="table-rail"><section className="objective"><p className="eyebrow">任務摘要</p><strong>{campaign.objective}</strong><p className="objective-context">{campaign.objectiveContext}</p><div className="objective-stakes"><span>風險</span><p>{campaign.stakes}</p></div><small>{campaign.combat?.active ? `戰鬥第 ${campaign.combat.round} 輪（戰鬥操作已顯示於主畫面）` : '探索進行中'}</small></section></aside></motion.main>}
        {page === 'characters' && <motion.div key="characters" initial={{ opacity: 0 }} animate={{ opacity: 1 }}><Suspense fallback={<main className="single-page lazy-page-loading" role="status"><span>正在載入角色成長資料…</span></main>}><CharacterManager players={campaign.players} showStatHints={campaign.showStatHints !== false} onUpdate={updatePlayer} onLog={addLog} onGeneratePortrait={generatePortrait} /></Suspense></motion.div>}
        {page === 'journal' && <motion.main key="journal" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page"><div className="page-intro"><p className="eyebrow">戰役記憶</p><h2>{campaign.title}</h2><p>公開與私密訊息都包含在本機存檔與匯出檔中。</p></div><div className="journal-list">{visibleStory.map((entry, index) => <article key={entry.id} className={entry.audience && entry.audience !== 'public' ? 'journal-private' : ''}><span>{String(index + 1).padStart(2, '0')}</span><div><small>{entry.time}／{speakerLabel(entry)}</small><p>{entry.text}</p></div></article>)}</div></motion.main>}
        {page === 'settings' && <motion.main key="settings" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page settings-page"><div className="page-intro"><p className="eyebrow">本機設定</p><h2>地城主與戰役</h2><p>所有切換操作都會先保存目前進度；匯入預設不切換。</p></div>
          <section className="settings-row"><div><strong>示範 DM</strong><span>完全不呼叫模型。</span></div><button type="button" className={`switch ${demoMode ? 'switch-on' : ''}`} onClick={() => setDemoMode((value) => !value)}><i /></button></section>
          <section className="settings-row model-selector"><div><strong>Codex 模型</strong><span>只影響之後的新 DM 請求；目前進度與既有訊息不會改變。</span></div><select value={campaign.selectedModel || ''} onChange={(event) => setCampaign((current) => ({ ...current, selectedModel: event.target.value }))}>{(status?.models || [{ id: '', label: 'Codex 預設（沿用目前設定）' }]).map((model) => <option key={model.id || 'default'} value={model.id}>{model.label}</option>)}</select></section>
          <section className="settings-row model-selector"><div><strong>推理強度（effort）</strong><span>越高越深思但回應越慢；只影響之後的新 DM 請求。</span></div><select value={campaign.selectedEffort || ''} onChange={(event) => setCampaign((current) => ({ ...current, selectedEffort: event.target.value }))}>{(status?.efforts || [{ id: '', label: 'Codex 預設推理強度' }]).map((effort) => <option key={effort.id || 'default'} value={effort.id}>{effort.label}</option>)}</select></section>
          <section className="settings-row"><div><strong>Codex CLI</strong><span>{status?.connected ? `已登入／${status.model}` : status?.message || '正在檢查'}</span></div><ShieldWarning size={22} /></section>
          <section className="settings-row model-selector"><div><strong>圖片生成引擎</strong><span>場景圖與角色肖像使用的後端；本地選項需先啟動 SD Forge（--api）。</span></div><select value={campaign.imageBackend || status?.imageBackend || 'codex'} onChange={(event) => setCampaign((current) => ({ ...current, imageBackend: event.target.value }))}>{(status?.imageBackends || [{ id: 'codex', label: status?.imageModel || 'Codex $imagegen' }]).map((backend) => <option key={backend.id} value={backend.id}>{backend.label}</option>)}</select></section>
          <section className="settings-row"><div><strong>每回合自動生成場景圖</strong><span>開啟後，每次 DM 完成公開敘事便自動生成並加入圖庫。</span></div><button type="button" className={`switch ${campaign.autoSceneImages ? 'switch-on' : ''}`} onClick={() => setCampaign((current) => ({ ...current, autoSceneImages: !current.autoSceneImages }))}><i /></button></section>
          <section className="settings-row"><div><strong>語音朗讀 DM 敘事</strong><span>使用本地 GPT-SoVITS 朗讀每回合公開敘事；需先啟動 scripts/sovits.sh 並設定聲線。</span></div><button type="button" role="switch" aria-checked={Boolean(campaign.ttsEnabled)} aria-label="語音朗讀 DM 敘事" className={`switch ${campaign.ttsEnabled ? 'switch-on' : ''}`} onClick={() => setCampaign((current) => ({ ...current, ttsEnabled: !current.ttsEnabled }))}><i /></button></section>
          <section className="settings-row"><div><strong>角色屬性懸浮說明</strong><span>滑鼠停留或用鍵盤聚焦屬性時，顯示規則用途與計算方式。</span></div><button type="button" role="switch" aria-checked={campaign.showStatHints !== false} aria-label="角色屬性懸浮說明" className={`switch ${campaign.showStatHints !== false ? 'switch-on' : ''}`} onClick={() => setCampaign((current) => ({ ...current, showStatHints: current.showStatHints === false }))}><i /></button></section>
          <section className="settings-row"><div><strong>介面字型大小</strong><span>{Math.round((campaign.fontScale || 1) * 100)}%</span></div><div className="font-controls"><button type="button" onClick={() => setCampaign((current) => ({ ...current, fontScale: Math.max(.85, (current.fontScale || 1) - .1) }))}>A−</button><button type="button" onClick={() => setCampaign((current) => ({ ...current, fontScale: 1 }))}>重設</button><button type="button" onClick={() => setCampaign((current) => ({ ...current, fontScale: Math.min(1.25, (current.fontScale || 1) + .1) }))}>A＋</button></div></section>
          <CampaignManager campaign={campaign} campaigns={campaigns} onSwitch={switchCampaign} onNew={newCampaign} onDuplicate={() => { const copy = duplicateCampaign(campaign, initialCampaign); setCampaigns(listCampaigns(initialCampaign)); setNotice(`已建立「${copy.title}」，目前戰役未切換。`); }} onImport={importFile} />
          <section className="settings-danger"><div><strong>重設目前戰役</strong><span>只重設目前選取的戰役，不影響其他存檔。</span></div><MagneticButton variant="quiet" onClick={resetCampaign}>重設目前戰役</MagneticButton></section>
        </motion.main>}
      </AnimatePresence>
      <footer><span>{campaign.scene}</span><span>{latestDm ? `最後裁定 ${latestDm.time}` : '等待第一個裁定'}</span></footer>
    </div></div>
  );
}
