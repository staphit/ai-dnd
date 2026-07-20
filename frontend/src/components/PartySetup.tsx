import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { ArrowRight, BookOpenText, ShieldCheck, Sword } from '@phosphor-icons/react';
import { motion } from 'framer-motion';
import type { AbilityKey, AbilityScores, Campaign } from '../types';
import { storyPresets, type StoryPreset } from '../data';
import { MagneticButton } from './MagneticButton';
import { StoryModeModal } from './StoryModeModal';
import { abilityLabels } from '../labels';
import { createCampaign, getCatalog, type PlayerSeed } from '../api';

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
  const [title, setTitle] = useState(storyPresets[0].title);
  const [selectedStoryId, setSelectedStoryId] = useState(storyPresets[0].id);
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
    setTitle(story.title);
  }

  async function create(preset: StoryPreset, campaignTitle: string, seeds: PlayerSeed[], storyMode?: 'scripted' | 'freeform') {
    if (submitting) return;
    setSubmitting(true);
    setError('');
    try {
      const view = await createCampaign({
        storyId: preset.id,
        title: campaignTitle,
        chapter: preset.chapter,
        scene: preset.scene,
        objective: preset.objective,
        objectiveContext: preset.objectiveContext,
        stakes: preset.stakes,
        opening: preset.opening,
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
          {storyPresets.map((story) => <button key={story.id} type='button' className={selectedStoryId === story.id ? 'story-option story-option-active' : 'story-option'} onClick={() => selectStory(story)} aria-pressed={selectedStoryId === story.id}>
            <span className='story-option-head'><strong>{story.title}</strong><small>{story.genre}</small></span>
            <span className='story-option-summary'>{story.summary}</span>
            <span className='story-option-tags'>{story.tags.map((tag) => <i key={tag}>{tag}</i>)}</span>
          </button>)}
        </div></fieldset>
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
        <div className='setup-submit'><span>{selectedStory.genre}／{partySize} 位冒險者</span><MagneticButton type='submit' disabled={submitting}><span>{submitting ? '建立中…' : '開始冒險'}</span><ArrowRight size={17} /></MagneticButton></div>
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
