import { useMemo, useState, type FormEvent } from 'react';
import { ArrowRight, ShieldCheck, Sword, UsersThree } from '@phosphor-icons/react';
import { motion } from 'framer-motion';
import type { AbilityKey, AbilityScores, PlayerCharacter, PlayerId } from '../types';
import { MagneticButton } from './MagneticButton';
import { abilityLabels, classDefinitions, classNames, type ClassName } from '../rules/characters';
import { createConfiguredCharacter } from '../rules/advancement';

type DraftPlayer = { name: string; className: ClassName; level: number; species: string; background: string; abilities: AbilityScores };

const fallbackNames = ['冒險者一號', '冒險者二號', '冒險者三號', '冒險者四號'];
const fallbackClasses: ClassName[] = ['戰士', '牧師', '遊俠', '法師'];

function normalizeClass(value?: string, index = 0): ClassName {
  return classNames.find((className) => value?.includes(className)) || fallbackClasses[index] || '戰士';
}

function toDraft(player: PlayerCharacter | undefined, index: number): DraftPlayer {
  const className = normalizeClass(player?.className, index);
  return {
    name: player?.name || fallbackNames[index],
    className,
    level: player?.level || 3,
    species: player?.species || '人類',
    background: player?.background || classDefinitions[className].background,
    abilities: player?.abilities || { ...classDefinitions[className].abilities },
  };
}

interface PartySetupProps {
  initialTitle: string;
  initialPlayers: PlayerCharacter[];
  onComplete: (setup: { title: string; players: PlayerCharacter[] }) => void;
}

export function PartySetup({ initialTitle, initialPlayers, onComplete }: PartySetupProps) {
  const initialCount = Math.min(4, Math.max(1, initialPlayers.length || 2));
  const [title, setTitle] = useState(initialTitle);
  const [partySize, setPartySize] = useState(initialCount);
  const [players, setPlayers] = useState<DraftPlayer[]>(() => Array.from({ length: 4 }, (_, index) => toDraft(initialPlayers[index], index)));
  const [error, setError] = useState('');
  const activePlayers = useMemo(() => players.slice(0, partySize), [partySize, players]);

  function updatePlayer(index: number, patch: Partial<DraftPlayer>) {
    setPlayers((current) => current.map((player, playerIndex) => playerIndex === index ? { ...player, ...patch } : player));
  }

  function changeClass(index: number, className: ClassName) {
    updatePlayer(index, { className, background: classDefinitions[className].background, abilities: { ...classDefinitions[className].abilities } });
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    const campaignTitle = title.trim();
    const names = activePlayers.map((player) => player.name.trim());
    if (!campaignTitle) return setError('請替這次冒險取一個戰役名稱。');
    if (names.some((name) => !name)) return setError('每位玩家角色都需要一個名稱。');
    if (new Set(names).size !== names.length) return setError('角色名稱不能重複，這樣 DM 才能正確辨認行動。');
    const configuredPlayers = activePlayers.map((player, index): PlayerCharacter => createConfiguredCharacter(
      `player${index + 1}` as PlayerId,
      player.name,
      player.className,
      { level: player.level, species: player.species, background: player.background, abilities: player.abilities },
    ));
    onComplete({ title: campaignTitle, players: configuredPlayers });
  }

  return (
    <main className="setup-shell">
      <div className="grain" aria-hidden="true" />
      <motion.div className="setup-intro" initial={{ opacity: 0, y: 18 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: .55 }}>
        <div className="setup-mark"><Sword size={26} weight="thin" /></div>
        <p className="eyebrow">Session zero／開團設定</p><h1>先決定，<br />誰踏進黑暗。</h1>
        <p>建立 1–4 人、等級 1–20 的隊伍。進階欄位可自訂種族、背景與能力值。</p>
        <div className="setup-note"><ShieldCheck size={18} /><span>角色與故事只保存在這台裝置</span></div>
      </motion.div>
      <motion.form className="setup-form" onSubmit={submit} initial={{ opacity: 0, y: 24 }} animate={{ opacity: 1, y: 0 }}>
        <header><div><p className="eyebrow">隊伍名冊</p><h2>建立冒險者</h2></div><UsersThree size={24} /></header>
        <label className="setup-field"><span>戰役名稱</span><input value={title} onChange={(event) => setTitle(event.target.value)} maxLength={60} /></label>
        <fieldset className="party-size"><legend>隊伍人數</legend><div>{[1, 2, 3, 4].map((size) => <button key={size} type="button" className={partySize === size ? 'party-size-active' : ''} onClick={() => setPartySize(size)} aria-pressed={partySize === size}><strong>{size}</strong><span>人</span></button>)}</div></fieldset>
        <div className="party-roster">
          {activePlayers.map((player, index) => (
            <motion.section key={index} className="player-setup-row advanced-setup-row" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
              <span className="player-number">{String(index + 1).padStart(2, '0')}</span>
              <label><span>角色名稱</span><input value={player.name} onChange={(event) => updatePlayer(index, { name: event.target.value })} aria-label={`玩家 ${index + 1} 角色名稱`} /></label>
              <label><span>職業</span><select value={player.className} onChange={(event) => changeClass(index, event.target.value as ClassName)} aria-label={`玩家 ${index + 1} 職業`}>{classNames.map((name) => <option key={name}>{name}</option>)}</select><small>{classDefinitions[player.className].subclass}</small></label>
              <label><span>起始等級</span><input type="number" min="1" max="20" value={player.level} onChange={(event) => updatePlayer(index, { level: Math.min(20, Math.max(1, Number(event.target.value))) })} aria-label={`玩家 ${index + 1} 起始等級`} /></label>
              <details className="setup-advanced"><summary>自訂種族、背景與能力值</summary><div className="setup-advanced-fields">
                <label><span>種族</span><input value={player.species} onChange={(event) => updatePlayer(index, { species: event.target.value })} /></label>
                <label><span>背景</span><input value={player.background} onChange={(event) => updatePlayer(index, { background: event.target.value })} /></label>
                {(Object.keys(abilityLabels) as AbilityKey[]).map((key) => <label key={key}><span>{abilityLabels[key]}</span><input type="number" min="3" max="30" value={player.abilities[key]} onChange={(event) => updatePlayer(index, { abilities: { ...player.abilities, [key]: Number(event.target.value) } })} /></label>)}
              </div></details>
            </motion.section>
          ))}
        </div>
        {error && <p className="setup-error" role="alert">{error}</p>}
        <div className="setup-submit"><span>{partySize} 位冒險者／自訂等級</span><MagneticButton type="submit"><span>開始冒險</span><ArrowRight size={17} /></MagneticButton></div>
      </motion.form>
    </main>
  );
}
