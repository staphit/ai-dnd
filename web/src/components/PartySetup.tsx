import { useMemo, useState, type FormEvent } from 'react';
import { ArrowRight, ShieldCheck, Sword, UsersThree } from '@phosphor-icons/react';
import { motion } from 'framer-motion';
import type { PlayerCharacter, PlayerId } from '../types';
import { MagneticButton } from './MagneticButton';
import { classDefinitions, classNames, createLevel3Character, type ClassName } from '../rules/characters';

type DraftPlayer = { name: string; className: ClassName };

const fallbackNames = ['冒險者一號', '冒險者二號', '冒險者三號', '冒險者四號'];
const fallbackClasses: ClassName[] = ['戰士', '牧師', '遊俠', '法師'];

function normalizeClass(value?: string, index = 0): ClassName {
  return classNames.find((className) => value?.endsWith(className)) || fallbackClasses[index] || '戰士';
}

function toDraft(player: PlayerCharacter | undefined, index: number): DraftPlayer {
  return {
    name: player?.name || fallbackNames[index],
    className: normalizeClass(player?.className, index),
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
  const [players, setPlayers] = useState<DraftPlayer[]>(() =>
    Array.from({ length: 4 }, (_, index) => toDraft(initialPlayers[index], index)),
  );
  const [error, setError] = useState('');
  const activePlayers = useMemo(() => players.slice(0, partySize), [partySize, players]);

  function updatePlayer(index: number, patch: Partial<DraftPlayer>) {
    setPlayers((current) => current.map((player, playerIndex) =>
      playerIndex === index ? { ...player, ...patch } : player,
    ));
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    const campaignTitle = title.trim();
    const names = activePlayers.map((player) => player.name.trim());
    if (!campaignTitle) {
      setError('請替這次冒險取一個戰役名稱。');
      return;
    }
    if (names.some((name) => !name)) {
      setError('每位玩家角色都需要一個名稱。');
      return;
    }
    if (new Set(names).size !== names.length) {
      setError('角色名稱不能重複，這樣 DM 才能正確辨認行動。');
      return;
    }

    const configuredPlayers = activePlayers.map((player, index): PlayerCharacter =>
      createLevel3Character(`player${index + 1}` as PlayerId, player.name, player.className),
    );
    onComplete({ title: campaignTitle, players: configuredPlayers });
  }

  return (
    <main className="setup-shell">
      <div className="grain" aria-hidden="true" />
      <motion.div
        className="setup-intro"
        initial={{ opacity: 0, y: 18 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: .55, ease: [0.16, 1, 0.3, 1] }}
      >
        <div className="setup-mark"><Sword size={26} weight="thin" /></div>
        <p className="eyebrow">Session zero／開團設定</p>
        <h1>先決定，<br />誰踏進黑暗。</h1>
        <p>建立 1–4 人隊伍。每一輪由所有角色提交行動，再交給 OpenAI 地城主統一裁定。</p>
        <div className="setup-note"><ShieldCheck size={18} /><span>角色與故事只保存在這台裝置</span></div>
      </motion.div>

      <motion.form
        className="setup-form"
        onSubmit={submit}
        initial={{ opacity: 0, y: 24 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: .08, duration: .58, ease: [0.16, 1, 0.3, 1] }}
      >
        <header>
          <div><p className="eyebrow">隊伍名冊</p><h2>建立冒險者</h2></div>
          <UsersThree size={24} aria-hidden="true" />
        </header>

        <label className="setup-field">
          <span>戰役名稱</span>
          <input value={title} onChange={(event) => setTitle(event.target.value)} maxLength={60} placeholder="例如：灰燼王冠" />
        </label>

        <fieldset className="party-size">
          <legend>隊伍人數</legend>
          <div>
            {[1, 2, 3, 4].map((size) => (
              <button
                key={size}
                type="button"
                className={partySize === size ? 'party-size-active' : ''}
                onClick={() => { setPartySize(size); setError(''); }}
                aria-pressed={partySize === size}
              >
                <strong>{size}</strong><span>人</span>
              </button>
            ))}
          </div>
        </fieldset>

        <div className="party-roster">
          {activePlayers.map((player, index) => (
            <motion.section
              key={index}
              className="player-setup-row"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: .3 }}
            >
              <span className="player-number">{String(index + 1).padStart(2, '0')}</span>
              <label>
                <span>角色名稱</span>
                <input
                  value={player.name}
                  onChange={(event) => updatePlayer(index, { name: event.target.value })}
                  maxLength={40}
                  aria-label={`玩家 ${index + 1} 角色名稱`}
                />
              </label>
              <label>
                <span>職業</span>
                <select
                  value={player.className}
                  onChange={(event) => updatePlayer(index, { className: event.target.value as ClassName })}
                  aria-label={`玩家 ${index + 1} 職業`}
                >
                  {classNames.map((className) => <option key={className} value={className}>{className}</option>)}
                </select>
                <small className="class-preview">{classDefinitions[player.className].subclass}／{classDefinitions[player.className].spellcasting ? '施法職業' : '武藝職業'}</small>
              </label>
            </motion.section>
          ))}
        </div>

        {error && <p className="setup-error" role="alert">{error}</p>}
        <div className="setup-submit">
          <span>{partySize} 位冒險者／等級 3</span>
          <MagneticButton type="submit"><span>開始冒險</span><ArrowRight size={17} /></MagneticButton>
        </div>
      </motion.form>
    </main>
  );
}
