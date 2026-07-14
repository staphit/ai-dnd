import { useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { DiceFive } from '@phosphor-icons/react';
import { MagneticButton } from './MagneticButton';
import type { PlayerCharacter } from '../types';
import { abilityLabels, getCheckBonus } from '../rules/characters';

const dice = [4, 6, 8, 10, 12, 20];

interface DiceTrayProps {
  players: PlayerCharacter[];
  onResult: (result: string) => void;
}

const checks = ['自訂', '先攻', ...Object.values(abilityLabels), '運動', '雜技', '巧手', '隱匿', '奧秘', '歷史', '調查', '自然', '宗教', '馴獸', '洞悉', '醫藥', '察覺', '求生', '欺瞞', '威嚇', '表演', '說服'];

function randomDie(sides: number) {
  const values = new Uint32Array(1);
  crypto.getRandomValues(values);
  return (values[0] % sides) + 1;
}

export function DiceTray({ players, onResult }: DiceTrayProps) {
  const [selected, setSelected] = useState(20);
  const [playerId, setPlayerId] = useState(players[0]?.id || 'player1');
  const [check, setCheck] = useState('自訂');
  const [modifier, setModifier] = useState(0);
  const [result, setResult] = useState<{ natural: number; total: number } | null>(null);
  const player = players.find((entry) => entry.id === playerId) || players[0];
  const effectiveModifier = check === '自訂' || !player ? modifier : getCheckBonus(player, check);

  function roll() {
    const natural = randomDie(selected);
    const total = natural + effectiveModifier;
    setResult({ natural, total });
    const owner = player ? `${player.name}進行${check === '自訂' ? '自訂擲骰' : check}：` : '';
    onResult(`${owner}擲 d${selected}${effectiveModifier ? `${effectiveModifier > 0 ? '+' : ''}${effectiveModifier}` : ''}：${total}（骰面 ${natural}）`);
  }

  return (
    <section className="dice-tray">
      <div className="section-heading compact">
        <div><p className="eyebrow">公開擲骰</p><h2>骰盤</h2></div>
        <DiceFive size={20} />
      </div>
      <div className="dice-options" role="group" aria-label="選擇骰子">
        {dice.map((sides) => (
          <button key={sides} type="button" className={selected === sides ? 'selected' : ''} onClick={() => setSelected(sides)}>d{sides}</button>
        ))}
      </div>
      <div className="check-controls">
        <label><span>角色</span><select value={player?.id} onChange={(event) => setPlayerId(event.target.value as PlayerCharacter['id'])}>{players.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
        <label><span>檢定</span><select value={check} onChange={(event) => setCheck(event.target.value)}>{checks.map((entry) => <option key={entry}>{entry}</option>)}</select></label>
      </div>
      <div className="modifier-row">
        <label htmlFor="modifier">加值</label>
        <input id="modifier" type="number" min={-10} max={20} value={effectiveModifier} disabled={check !== '自訂'} onChange={(event) => setModifier(Number(event.target.value))} />
      </div>
      <div className="roll-row">
        <AnimatePresence mode="wait">
          <motion.div
            key={result ? `${result.total}-${result.natural}` : 'empty'}
            initial={{ opacity: 0, scale: 0.82, rotate: -4 }}
            animate={{ opacity: 1, scale: 1, rotate: 0 }}
            exit={{ opacity: 0, scale: 0.9 }}
            transition={{ type: 'spring', stiffness: 180, damping: 18 }}
            className="dice-result"
          >
            <strong>{result?.total ?? '—'}</strong>
            <span>{result ? `d${selected} 骰面 ${result.natural}` : '等待擲骰'}</span>
          </motion.div>
        </AnimatePresence>
        <MagneticButton onClick={roll}>擲骰</MagneticButton>
      </div>
    </section>
  );
}
