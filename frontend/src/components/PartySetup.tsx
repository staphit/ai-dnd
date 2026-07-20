import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { ArrowRight, BookOpenText, ShieldCheck, Sword } from '@phosphor-icons/react';
import { motion } from 'framer-motion';
import type { AbilityKey, AbilityScores, Campaign } from '../types';
import {
  buildCustomStorySeed,
  CUSTOM_STORY_ID,
  customStoryInstructions,
  localizedPreset,
  storyPresets,
  type CustomStorySeed,
  type StoryPreset,
} from '../data';
import { MagneticButton } from './MagneticButton';
import { StoryModeModal } from './StoryModeModal';
import { abilityLabels } from '../labels';
import { createCampaign, getCatalog, type PlayerSeed } from '../api';
import { useI18n } from '../i18n';

// abilities stays undefined until the player opts into custom scores; the
// server then applies the class preset values.
type DraftPlayer = { name: string; className: string; level: number; species: string; background: string; abilities?: AbilityScores };

const fallbackNames = ['冒險者一號', '冒險者二號', '冒險者三號', '冒險者四號'];
const fallbackClasses = ['戰士', '牧師', '法師', '聖武士'];
const customAbilityBase: AbilityScores = { str: 15, dex: 14, con: 13, int: 12, wis: 10, cha: 8 };

function makeDraft(index: number): DraftPlayer {
  return {
    name: fallbackNames[index],
    className: fallbackClasses[index] || '戰士',
    level: 3,
    species: '人類',
    background: '',
  };
}

interface PartySetupProps {
  onComplete: (view: Campaign) => void;
  onCancel?: () => void;
}

export function PartySetup({ onComplete, onCancel }: PartySetupProps) {
  const { lang } = useI18n();
  // Campaign text (title, opening, objective…) is authored in the UI language
  // chosen before setup; the created campaign keeps that language server-side.
  const [title, setTitle] = useState(() => localizedPreset(storyPresets[0], lang).title);
  const [selectedStoryId, setSelectedStoryId] = useState(storyPresets[0].id);
  const [customStory, setCustomStory] = useState<Partial<CustomStorySeed> & { brief: string }>({ brief: '' });
  const [partySize, setPartySize] = useState(2);
  const [players, setPlayers] = useState<DraftPlayer[]>(() => Array.from({ length: 4 }, (_, index) => makeDraft(index)));
  const [classNames, setClassNames] = useState<string[]>(fallbackClasses);
  const [scriptedStoryIds, setScriptedStoryIds] = useState<string[]>([]);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  // Validated submission waiting for the story-mode choice (scripted presets).
  const [pendingCreate, setPendingCreate] = useState<{ preset: StoryPreset; campaignTitle: string; seeds: PlayerSeed[] } | null>(null);
  const activePlayers = useMemo(() => players.slice(0, partySize), [partySize, players]);

  useEffect(() => {
    let mounted = true;
    getCatalog()
      .then((catalog) => {
        if (!mounted) return;
        if (catalog.classNames.length > 0) setClassNames(catalog.classNames);
        setScriptedStoryIds(catalog.scriptedStoryIds || []);
      })
      .catch(() => { /* keep the fallback list; the server normalizes classes anyway */ });
    return () => { mounted = false; };
  }, []);

  function updatePlayer(index: number, patch: Partial<DraftPlayer>) {
    setPlayers((current) => current.map((player, playerIndex) => playerIndex === index ? { ...player, ...patch } : player));
  }

  function toggleCustomAbilities(index: number, enabled: boolean) {
    updatePlayer(index, { abilities: enabled ? { ...customAbilityBase } : undefined });
  }

  function selectStory(story: StoryPreset) {
    setSelectedStoryId(story.id);
    setTitle(localizedPreset(story, lang).title);
  }

  function selectCustomStory() {
    setSelectedStoryId(CUSTOM_STORY_ID);
    setTitle(lang === 'en' ? 'Custom adventure' : '自訂冒險');
  }

  function updateCustomStory(patch: Partial<CustomStorySeed>) {
    setCustomStory((current) => ({ ...current, ...patch }));
  }

  async function create(preset: StoryPreset, campaignTitle: string, seeds: PlayerSeed[], storyMode?: 'scripted' | 'freeform') {
    if (submitting) return;
    setSubmitting(true);
    setError('');
    const localized = localizedPreset(preset, lang);
    try {
      const view = await createCampaign({
        storyId: preset.id,
        title: campaignTitle,
        chapter: localized.chapter,
        scene: localized.scene,
        objective: localized.objective,
        objectiveContext: localized.objectiveContext,
        stakes: localized.stakes,
        opening: localized.opening,
        players: seeds,
        storyMode,
      });
      onComplete(view);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setSubmitting(false);
    }
  }

  async function createCustom(campaignTitle: string, seeds: PlayerSeed[]) {
    if (submitting) return;
    setSubmitting(true);
    setError('');
    const brief = customStory.brief.trim();
    const custom = buildCustomStorySeed({
      ...customStory,
      title: campaignTitle,
      brief,
      ...(lang === 'en' ? {
        genre: customStory.genre?.trim() || 'Custom',
        summary: customStory.summary?.trim() || brief.slice(0, 120) || 'A player-authored adventure.',
        chapter: customStory.chapter?.trim() || 'Chapter I / Departure',
        scene: customStory.scene?.trim() || 'Adventure opening',
        objective: customStory.objective?.trim() || 'Investigate the immediate crisis',
        objectiveContext: customStory.objectiveContext?.trim() || brief.slice(0, 600),
        stakes: customStory.stakes?.trim() || 'The threat worsens if the party delays.',
        opening: customStory.opening?.trim() || `The story begins here.\n\n${brief.slice(0, 800)}`,
      } : {}),
    });
    try {
      const view = await createCampaign({
        storyId: CUSTOM_STORY_ID,
        title: campaignTitle,
        chapter: custom.chapter,
        scene: custom.scene,
        objective: custom.objective,
        objectiveContext: custom.objectiveContext,
        stakes: custom.stakes,
        opening: custom.opening,
        players: seeds,
        storyMode: 'freeform',
      });
      onComplete(view);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setSubmitting(false);
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    if (submitting) return;
    const campaignTitle = title.trim();
    const names = activePlayers.map((player) => player.name.trim());
    if (!campaignTitle) return setError('請替這次冒險取一個戰役名稱。');
    if (names.some((name) => !name)) return setError('每位玩家角色都需要一個名稱。');
    if (new Set(names).size !== names.length) return setError('角色名稱不能重複，這樣 DM 才能正確辨認行動。');
    const seeds = activePlayers.map((player): PlayerSeed => ({
      name: player.name.trim(),
      className: player.className,
      level: player.level,
      species: player.species.trim() || undefined,
      background: player.background.trim() || undefined,
      abilities: player.abilities,
    }));
    if (selectedStoryId === CUSTOM_STORY_ID) {
      if (!customStory.brief.trim()) return setError('請先寫下自訂劇本的冒險構想。');
      setError('');
      void createCustom(campaignTitle, seeds);
      return;
    }
    const preset = storyPresets.find((story) => story.id === selectedStoryId) || storyPresets[0];
    setError('');
    // Presets with a hand-written script module first ask how to play; the
    // modal choice then triggers the actual creation. When the catalog is
    // unknown (fetch failed) the mode is omitted so the server default
    // (scripted whenever a module exists) still applies — an explicit
    // 'freeform' opt-out only ever comes from the modal.
    if (scriptedStoryIds.includes(preset.id)) {
      setPendingCreate({ preset, campaignTitle, seeds });
      return;
    }
    void create(preset, campaignTitle, seeds);
  }

  const selectedStory = storyPresets.find((story) => story.id === selectedStoryId) || storyPresets[0];
  const selectedGenre = selectedStoryId === CUSTOM_STORY_ID
    ? customStory.genre?.trim() || (lang === 'en' ? 'Custom' : '自訂')
    : localizedPreset(selectedStory, lang).genre;

  return (
    <main className='setup-shell'>
      <div className='grain' aria-hidden='true' />
      <motion.div className='setup-intro' initial={{ opacity: 0, y: 18 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: .55 }}>
        <div className='setup-mark'><Sword size={26} weight='thin' /></div>
        <p className='eyebrow'>Session zero／開團設定</p><h1>先選故事，<br />再踏進黑暗。</h1>
        <p>從四種不同調性的冒險中選一本，再建立 1–4 人、等級 1–20 的隊伍。</p>
        <div className='setup-note'><ShieldCheck size={18} /><span>戰役與角色會保存在本機伺服器</span></div>
        {onCancel && <button type='button' className='setup-cancel' onClick={onCancel}>返回目前戰役</button>}
      </motion.div>
      <motion.form className='setup-form' onSubmit={submit} initial={{ opacity: 0, y: 24 }} animate={{ opacity: 1, y: 0 }}>
        <header><div><p className='eyebrow'>劇本與隊伍</p><h2>選擇這次的冒險</h2></div><BookOpenText size={24} /></header>
        <fieldset className='story-selection'><legend>冒險劇本</legend><div className='story-options'>
          {storyPresets.map((story) => {
            const shown = localizedPreset(story, lang);
            return <button key={story.id} type='button' className={selectedStoryId === story.id ? 'story-option story-option-active' : 'story-option'} onClick={() => selectStory(story)} aria-pressed={selectedStoryId === story.id}>
              <span className='story-option-head'><strong>{shown.title}</strong><small>{shown.genre}</small></span>
              <span className='story-option-summary'>{shown.summary}</span>
              <span className='story-option-tags'>{shown.tags.map((tag) => <i key={tag}>{tag}</i>)}</span>
            </button>;
          })}
          <button type='button' className={selectedStoryId === CUSTOM_STORY_ID ? 'story-option story-option-custom story-option-active' : 'story-option story-option-custom'} onClick={selectCustomStory} aria-pressed={selectedStoryId === CUSTOM_STORY_ID}>
            <span className='story-option-head'><strong>{lang === 'en' ? 'Custom story' : '自訂劇本'}</strong><small>{lang === 'en' ? 'AI freeform' : 'AI 即興'}</small></span>
            <span className='story-option-summary'>{lang === 'en' ? 'Write your own premise, opening, objective, and stakes.' : '寫下自己的世界、衝突與開場，由 AI 地城主依構想即興推進。'}</span>
            <span className='story-option-tags'><i>{lang === 'en' ? 'Original' : '原創'}</i><i>{lang === 'en' ? 'Freeform' : '自由模式'}</i></span>
          </button>
        </div></fieldset>
        {selectedStoryId === CUSTOM_STORY_ID && <section className='custom-story-panel' aria-label={lang === 'en' ? 'Custom story details' : '自訂劇本內容'}>
          <div className='custom-story-heading'><BookOpenText size={20} /><div><strong>{lang === 'en' ? 'Write your adventure premise' : '撰寫冒險構想'}</strong><span>{lang === 'en' ? 'The AI DM uses these details as the campaign foundation.' : 'AI 地城主會以這些內容作為整場戰役的基礎。'}</span></div></div>
          {lang !== 'en' && <div className='custom-story-guide'><p className='custom-story-guide-title'>撰寫指南</p><ol>{customStoryInstructions.split('\n').map((instruction) => <li key={instruction}>{instruction}</li>)}</ol><p className='custom-story-guide-example'><strong>範例：</strong>港城每逢滿月就有人失去影子；玩家從封鎖的燈塔醒來，必須在下一次月升前找出偷走影子的儀式。</p></div>}
          <div className='custom-story-grid'>
            <label className='setup-field custom-brief-field custom-span-2'><span>{lang === 'en' ? 'Adventure premise (required)' : '冒險構想（必填）'}</span><textarea aria-label={lang === 'en' ? 'Custom story premise' : '自訂劇本構想'} value={customStory.brief} maxLength={2000} onChange={(event) => updateCustomStory({ brief: event.target.value })} /><small>{lang === 'en' ? 'Describe the setting, conflict, clues, and tone.' : '描述世界、主要衝突、線索與故事調性；最多 2000 字。'}</small></label>
            <label className='setup-field'><span>{lang === 'en' ? 'Genre' : '類型／調性'}</span><input aria-label={lang === 'en' ? 'Custom story genre' : '自訂劇本類型'} value={customStory.genre || ''} maxLength={80} placeholder={lang === 'en' ? 'Mystery, horror, heroic…' : '懸疑、恐怖、史詩……'} onChange={(event) => updateCustomStory({ genre: event.target.value })} /></label>
            <label className='setup-field'><span>{lang === 'en' ? 'Opening scene' : '起始場景'}</span><input aria-label={lang === 'en' ? 'Custom story scene' : '自訂劇本起始場景'} value={customStory.scene || ''} maxLength={120} placeholder={lang === 'en' ? 'Where the party begins' : '隊伍從哪裡開始'} onChange={(event) => updateCustomStory({ scene: event.target.value })} /></label>
            <label className='setup-field custom-span-2'><span>{lang === 'en' ? 'First objective' : '第一個目標'}</span><input aria-label={lang === 'en' ? 'Custom story objective' : '自訂劇本目標'} value={customStory.objective || ''} maxLength={180} placeholder={lang === 'en' ? 'What must the party accomplish first?' : '隊伍首先必須完成什麼？'} onChange={(event) => updateCustomStory({ objective: event.target.value })} /></label>
            <label className='setup-field custom-span-2'><span>{lang === 'en' ? 'Stakes' : '拖延／失敗的風險'}</span><textarea aria-label={lang === 'en' ? 'Custom story stakes' : '自訂劇本風險'} value={customStory.stakes || ''} maxLength={300} placeholder={lang === 'en' ? 'What worsens if the party fails or delays?' : '如果玩家失敗或拖延，情況會如何惡化？'} onChange={(event) => updateCustomStory({ stakes: event.target.value })} /></label>
            <label className='setup-field custom-span-2'><span>{lang === 'en' ? 'Opening narration (optional)' : '開場敘事（選填）'}</span><textarea aria-label={lang === 'en' ? 'Custom story opening narration' : '自訂劇本開場敘事'} value={customStory.opening || ''} maxLength={3000} placeholder={lang === 'en' ? 'Leave blank to generate an opening from the premise.' : '留空時會依冒險構想產生預設開場。'} onChange={(event) => updateCustomStory({ opening: event.target.value })} /></label>
          </div>
        </section>}
        <label className='setup-field'><span>戰役名稱</span><input value={title} onChange={(event) => setTitle(event.target.value)} maxLength={60} /></label>
        <fieldset className='party-size'><legend>隊伍人數</legend><div>{[1, 2, 3, 4].map((size) => <button key={size} type='button' className={partySize === size ? 'party-size-active' : ''} onClick={() => setPartySize(size)} aria-pressed={partySize === size}><strong>{size}</strong><span>人</span></button>)}</div></fieldset>
        <div className='party-roster'>
          {activePlayers.map((player, index) => (
            <motion.section key={index} className='player-setup-row advanced-setup-row' initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
              <span className='player-number'>{String(index + 1).padStart(2, '0')}</span>
              <label><span>角色名稱</span><input value={player.name} onChange={(event) => updatePlayer(index, { name: event.target.value })} aria-label={`玩家 ${index + 1} 角色名稱`} /></label>
              <label><span>職業</span><select value={player.className} onChange={(event) => updatePlayer(index, { className: event.target.value })} aria-label={`玩家 ${index + 1} 職業`}>{classNames.map((name) => <option key={name}>{name}</option>)}</select></label>
              <label><span>起始等級</span><input type='number' min='1' max='20' value={player.level} onChange={(event) => updatePlayer(index, { level: Math.min(20, Math.max(1, Number(event.target.value))) })} aria-label={`玩家 ${index + 1} 起始等級`} /></label>
              <details className='setup-advanced'><summary>自訂種族、背景與能力值</summary><div className='setup-advanced-fields'>
                <label><span>種族</span><input value={player.species} onChange={(event) => updatePlayer(index, { species: event.target.value })} /></label>
                <label><span>背景</span><input value={player.background} placeholder='留空使用職業預設' onChange={(event) => updatePlayer(index, { background: event.target.value })} /></label>
                <label className='setup-custom-abilities'><input type='checkbox' checked={Boolean(player.abilities)} onChange={(event) => toggleCustomAbilities(index, event.target.checked)} aria-label={`玩家 ${index + 1} 自訂能力值`} /><span>自訂能力值（未勾選時使用職業預設值）</span></label>
                {player.abilities && (Object.keys(abilityLabels) as AbilityKey[]).map((key) => <label key={key}><span>{abilityLabels[key]}</span><input type='number' min='3' max='30' value={player.abilities![key]} onChange={(event) => updatePlayer(index, { abilities: { ...player.abilities!, [key]: Number(event.target.value) } })} /></label>)}
              </div></details>
            </motion.section>
          ))}
        </div>
        {error && <p className='setup-error' role='alert'>{error}</p>}
        <div className='setup-submit'><span>{lang === 'en' ? `${selectedGenre} / ${partySize} adventurer${partySize > 1 ? 's' : ''}` : `${selectedGenre}／${partySize} 位冒險者`}</span><MagneticButton type='submit' disabled={submitting}><span>{submitting ? (lang === 'en' ? 'Creating…' : '建立中…') : (lang === 'en' ? 'Begin the adventure' : '開始冒險')}</span><ArrowRight size={17} /></MagneticButton></div>
      </motion.form>
      {pendingCreate && (
        <StoryModeModal
          busy={submitting}
          onClose={() => setPendingCreate(null)}
          onPick={(mode) => {
            const args = pendingCreate;
            setPendingCreate(null);
            if (args) void create(args.preset, args.campaignTitle, args.seeds, mode);
          }}
        />
      )}
    </main>
  );
}
