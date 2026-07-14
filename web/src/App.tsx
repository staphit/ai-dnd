import { useEffect, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { BookOpenText, ImageSquare, MapTrifold, ShieldWarning, XCircle } from '@phosphor-icons/react';
import { initialCampaign, demoResponses } from './data';
import type { AiStatus, Campaign, CharacterSpell, Page, PlayerCharacter, PlayerId, RestType, StoryEntry } from './types';
import { Sidebar } from './components/Sidebar';
import { Topbar } from './components/Topbar';
import { StoryFeed } from './components/StoryFeed';
import { ActionComposer } from './components/ActionComposer';
import { CharacterPanel } from './components/CharacterPanel';
import { DiceTray } from './components/DiceTray';
import { MagneticButton } from './components/MagneticButton';
import { SceneVisual } from './components/SceneVisual';
import { PartySetup } from './components/PartySetup';
import { changeResource, classNames, createLevel3Character, restCharacter, spendSpellSlot } from './rules/characters';
import { areAllActionsReady, createActionPayload } from './rules/party';

const storageKey = 'dnd-duet-web-v1';

function now() {
  return new Intl.DateTimeFormat('zh-TW', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date());
}

function makeEntry(speaker: StoryEntry['speaker'], text: string): StoryEntry {
  return { id: `${Date.now()}-${crypto.randomUUID()}`, speaker, text, time: now() };
}

function loadCampaign(): Campaign {
  try {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return initialCampaign;
    const stored = JSON.parse(raw) as Partial<Campaign>;
    const players = Array.isArray(stored.players) && stored.players.length > 0
      ? stored.players.slice(0, 4).map((player, index) => {
          const id = `player${index + 1}` as PlayerId;
          if (player.abilities && Array.isArray(player.resources) && Array.isArray(player.features)) {
            return { ...player, id };
          }
          const className = classNames.find((candidate) => String(player.className || '').endsWith(candidate)) || '戰士';
          const migrated = createLevel3Character(id, String(player.name || `冒險者 ${index + 1}`), className);
          return { ...migrated, hp: Math.min(migrated.maxHp, Number(player.hp || migrated.maxHp)) };
        })
      : initialCampaign.players;
    return {
      ...initialCampaign,
      ...stored,
      setupComplete: stored.setupComplete === true,
      players,
      story: Array.isArray(stored.story) ? stored.story : initialCampaign.story,
      pending: stored.pending || {},
    };
  } catch {
    return initialCampaign;
  }
}

export default function App() {
  const [campaign, setCampaign] = useState<Campaign>(loadCampaign);
  const [page, setPage] = useState<Page>('table');
  const [status, setStatus] = useState<AiStatus | null>(null);
  const [demoMode, setDemoMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [imageLoading, setImageLoading] = useState(false);
  const [imageError, setImageError] = useState('');

  useEffect(() => {
    localStorage.setItem(storageKey, JSON.stringify(campaign));
  }, [campaign]);

  useEffect(() => {
    const controller = new AbortController();
    fetch('/api/status', { signal: controller.signal })
      .then((response) => response.json())
      .then((data: AiStatus) => setStatus(data))
      .catch(() => setStatus({ connected: false, provider: 'OpenAI Agents SDK', model: null, message: '本機伺服器未啟動' }));
    return () => controller.abort();
  }, []);

  const latestDm = useMemo(() => [...campaign.story].reverse().find((entry) => entry.speaker === 'dm'), [campaign.story]);

  function changeHp(id: PlayerCharacter['id'], delta: number) {
    setCampaign((current) => ({
      ...current,
      players: current.players.map((player) => player.id === id
        ? { ...player, hp: Math.max(0, Math.min(player.maxHp, player.hp + delta)) }
        : player),
    }));
  }

  function addDiceResult(text: string) {
    setCampaign((current) => ({ ...current, story: [...current.story, makeEntry('system', text)] }));
  }

  function changeClassResource(id: PlayerId, resourceId: string, delta: number) {
    setCampaign((current) => ({
      ...current,
      players: current.players.map((player) => player.id === id ? changeResource(player, resourceId, delta) : player),
    }));
  }

  function castSpell(id: PlayerId, spell: CharacterSpell, asRitual: boolean) {
    const player = campaign.players.find((entry) => entry.id === id);
    if (!player) return;
    const usedFreeClassCast = Boolean(spell.freeUseResourceId && player.resources.some((entry) => entry.id === spell.freeUseResourceId && entry.current > 0));
    const updated = spendSpellSlot(player, spell, asRitual);
    if (!updated) {
      setError(`${player.name} 沒有可用的 ${spell.level} 環法術位。`);
      return;
    }
    const mode = asRitual ? '以儀式' : spell.level === 0 ? '施展戲法' : usedFreeClassCast ? '使用職業的免費施法能力施放' : `消耗 ${spell.level} 環以上法術位施放`;
    setCampaign((current) => ({
      ...current,
      players: current.players.map((entry) => entry.id === id ? updated : entry),
      story: [...current.story, makeEntry('system', `${player.name}${mode}「${spell.name}」：${spell.description}`)],
    }));
    setError('');
  }

  function rest(id: PlayerId, type: RestType) {
    const player = campaign.players.find((entry) => entry.id === id);
    if (!player) return;
    setCampaign((current) => ({
      ...current,
      players: current.players.map((entry) => entry.id === id ? restCharacter(entry, type) : entry),
      story: [...current.story, makeEntry('system', `${player.name}完成${type === 'short' ? '短休' : '長休'}，相應的生命、法術位與職業資源已恢復。`)],
    }));
  }

  async function generateImage() {
    if (!status?.connected || imageLoading) return;
    const narration = latestDm?.text;
    if (!narration) {
      setImageError('目前沒有可供繪製的 DM 場景敘事。');
      return;
    }
    setImageLoading(true);
    setImageError('');
    try {
      const response = await fetch('/api/scene-image', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          campaign: { title: campaign.title, scene: campaign.scene },
          narration,
          players: campaign.players,
        }),
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || '場景插圖生成失敗');
      setCampaign((current) => ({
        ...current,
        sceneImage: {
          url: data.url,
          scene: current.scene,
          createdAt: now(),
          model: data.model || status.imageModel || 'GPT Image',
        },
      }));
    } catch (caught) {
      setImageError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setImageLoading(false);
    }
  }

  function submitAction(player: PlayerId, text: string) {
    if (loading || campaign.pending[player]) return;
    const nextPending: Partial<Record<PlayerId, string>> = { ...campaign.pending, [player]: text };
    const nextStory = [...campaign.story, makeEntry(player, text)];
    setCampaign((current) => ({ ...current, pending: nextPending, story: nextStory }));
    if (areAllActionsReady(campaign.players, nextPending)) {
      void advance(nextPending, nextStory);
    }
  }

  async function advance(actions: Partial<Record<PlayerId, string>>, history: StoryEntry[]) {
    setLoading(true);
    setError('');
    try {
      let text: string;
      let nextScene: string | undefined;
      if (demoMode) {
        await new Promise((resolve) => window.setTimeout(resolve, 1100));
        text = demoResponses[campaign.round % demoResponses.length];
      } else {
        const response = await fetch('/api/dm', {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({
            actions: createActionPayload(campaign.players, actions),
            campaign: { title: campaign.title, scene: campaign.scene, round: campaign.round },
            players: campaign.players,
            history,
          }),
        });
        const data = await response.json();
        if (!response.ok) throw new Error(data.error || 'AI DM 無法回應');
        text = data.text;
        nextScene = typeof data.scene === 'string' ? data.scene : undefined;
      }
      setCampaign((current) => ({
        ...current,
        scene: nextScene || current.scene,
        round: current.round + 1,
        pending: {},
        story: [...current.story, makeEntry('dm', text)],
      }));
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
      setCampaign((current) => ({ ...current, pending: {} }));
    } finally {
      setLoading(false);
    }
  }

  function resetCampaign() {
    localStorage.removeItem(storageKey);
    setCampaign(initialCampaign);
    setError('');
    setPage('table');
  }

  function completeSetup(setup: { title: string; players: PlayerCharacter[] }) {
    const names = setup.players.map((player) => player.name).join('、');
    setCampaign({
      setupComplete: true,
      title: setup.title,
      chapter: '第一章／沉鐘之夜',
      scene: '下城區・無燈禮拜堂',
      round: 1,
      objective: '在午夜鐘響前找到失蹤的製圖師伊薩克',
      players: setup.players,
      story: [
        makeEntry('dm', `禮拜堂的門在${names}身後闔上。沒有風，燭火卻同時朝祭壇偏斜；石板中央留著一圈新鮮泥痕，像有沉重的東西剛被拖進地下。祭壇後方傳來三下緩慢的敲擊聲。`),
        makeEntry('system', `隊伍已建立，共 ${setup.players.length} 位冒險者。全隊提交行動後，地城主才會推進場景。`),
      ],
      pending: {},
    });
    setPage('table');
    setError('');
  }

  function speakerLabel(speaker: StoryEntry['speaker']) {
    if (speaker === 'dm') return '地城主';
    if (speaker === 'system') return '紀錄';
    return campaign.players.find((player) => player.id === speaker)?.name || '冒險者';
  }

  if (!campaign.setupComplete) {
    return (
      <PartySetup
        initialTitle={campaign.title}
        initialPlayers={campaign.players}
        onComplete={completeSetup}
      />
    );
  }

  return (
    <div className="app-shell">
      <div className="grain" aria-hidden="true" />
      <Sidebar page={page} setPage={setPage} />
      <div className="workspace">
        <Topbar campaign={campaign} status={status} demoMode={demoMode} />
        <AnimatePresence mode="wait">
          {page === 'table' && (
            <motion.main key="table" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} className="table-layout">
              <div className="table-main">
                <section className="scene-strip">
                  <MapTrifold size={21} />
                  <div><span>目前場景</span><strong>{campaign.scene}</strong></div>
                  <div className="round-mark"><span>回合</span><strong>{String(campaign.round).padStart(2, '0')}</strong></div>
                </section>

                {status && !status.connected && !demoMode && (
                  <div className="model-notice">
                    <ShieldWarning size={20} />
                    <div>
                      <strong>本機 AI 尚未連線</strong>
                      <span>設定 OpenAI API Key，或先用內建情節測試隊伍遊戲流程。</span>
                    </div>
                    <MagneticButton variant="quiet" onClick={() => setDemoMode(true)}>使用示範 DM</MagneticButton>
                  </div>
                )}

                {error && (
                  <div className="error-banner" role="alert">
                    <XCircle size={19} weight="fill" />
                    <div><strong>DM 回應中斷</strong><span>{error}。你可以切換示範 DM 後重新提交。</span></div>
                    <button type="button" onClick={() => setError('')} aria-label="關閉錯誤"><XCircle size={18} /></button>
                  </div>
                )}

                <SceneVisual
                  image={campaign.sceneImage}
                  scene={campaign.scene}
                  loading={imageLoading}
                  error={imageError}
                  canGenerate={Boolean(status?.connected)}
                  onGenerate={generateImage}
                />

                <StoryFeed story={campaign.story} players={campaign.players} loading={loading} />

                <div className={`composer-grid party-${campaign.players.length}`}>
                  {campaign.players.map((player) => (
                    <ActionComposer
                      key={player.id}
                      player={player.id}
                      name={player.name}
                      className={player.className}
                      pending={campaign.pending[player.id]}
                      disabled={loading}
                      partySize={campaign.players.length}
                      onSubmit={submitAction}
                    />
                  ))}
                </div>
              </div>

              <aside className="table-rail">
                <section className="objective">
                  <p className="eyebrow">當前目標</p>
                  <strong>{campaign.objective}</strong>
                  <span>距離午夜還有 19 分鐘</span>
                </section>
                {campaign.players.map((player) => (
                  <CharacterPanel
                    key={player.id}
                    player={player}
                    onHpChange={changeHp}
                    onResourceChange={changeClassResource}
                    onCastSpell={castSpell}
                    onRest={rest}
                  />
                ))}
                <DiceTray players={campaign.players} onResult={addDiceResult} />
              </aside>
            </motion.main>
          )}

          {page === 'journal' && (
            <motion.main key="journal" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} className="single-page">
              <div className="page-intro"><p className="eyebrow">戰役記憶</p><h2>沉鐘之夜</h2><p>所有敘事與擲骰都保存在這台裝置的瀏覽器中。</p></div>
              <div className="journal-list">
                {campaign.story.length === 0 ? (
                  <div className="empty-state"><BookOpenText size={32} /><strong>還沒有冒險紀錄</strong><span>回到遊戲桌提交第一個行動。</span></div>
                ) : campaign.story.map((entry, index) => (
                  <article key={entry.id}><span>{String(index + 1).padStart(2, '0')}</span><div><small>{entry.time}／{speakerLabel(entry.speaker)}</small><p>{entry.text}</p></div></article>
                ))}
              </div>
            </motion.main>
          )}

          {page === 'settings' && (
            <motion.main key="settings" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} className="single-page settings-page">
              <div className="page-intro"><p className="eyebrow">本機設定</p><h2>地城主與存檔</h2><p>正式模式由本機伺服器安全呼叫 OpenAI Agent；示範模式完全不呼叫模型。</p></div>
              <section className="settings-row">
                <div><strong>示範 DM</strong><span>未設定 API Key 時使用三段內建情節測試完整流程。</span></div>
                <button type="button" className={`switch ${demoMode ? 'switch-on' : ''}`} onClick={() => setDemoMode((value) => !value)} aria-pressed={demoMode}><i /></button>
              </section>
              <section className="settings-row">
                <div><strong>OpenAI Agent</strong><span>{status?.connected ? `已設定 ${status.model}` : status?.message || '正在檢查本機服務'}</span></div>
                <ShieldWarning size={22} />
              </section>
              <section className="settings-row">
                <div><strong>場景圖片</strong><span>{status?.imageModel || 'gpt-image-2'}，由玩家手動觸發以控制等待與費用。</span></div>
                <ImageSquare size={22} />
              </section>
              <section className="settings-danger">
                <div><strong>重設目前戰役</strong><span>清除瀏覽器內的故事、生命值與回合進度。</span></div>
                <MagneticButton variant="quiet" onClick={resetCampaign}>重設存檔</MagneticButton>
              </section>
            </motion.main>
          )}
        </AnimatePresence>
        <footer><span>{campaign.scene}</span><span>{latestDm ? `最後裁定 ${latestDm.time}` : '等待第一個裁定'}</span></footer>
      </div>
    </div>
  );
}
