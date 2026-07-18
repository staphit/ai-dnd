import { useEffect, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { DiceFive } from '@phosphor-icons/react';
import { MagneticButton } from './MagneticButton';
import type { PlayerCharacter, RequiredCheck } from '../types';
import { getCheckBonus } from '../labels';
import { useI18n } from '../i18n';

interface DiceTrayProps {
  players: PlayerCharacter[];
  requiredCheck: RequiredCheck;
  onRequiredRoll: (result: DiceRollResult) => void;
  onRoll?: (result: DiceRollResult) => void;
}

export interface DiceRollResult {
  natural: number;
  total: number;
  modifier: number;
  success: boolean;
  text: string;
}

function randomD20() {
  const values = new Uint32Array(1);
  crypto.getRandomValues(values);
  return (values[0] % 20) + 1;
}

export function DiceTray({ players, requiredCheck, onRequiredRoll, onRoll }: DiceTrayProps) {
  const { lang, tz } = useI18n();
  const [result, setResult] = useState<{ natural: number; total: number } | null>(null);
  const timerRef = useRef<number | null>(null);
  const player = players.find((entry) => entry.id === requiredCheck.playerId)
    || players.find((entry) => entry.name === requiredCheck.character)
    || players[0];
  const checkLabel = requiredCheck.skill || requiredCheck.ability;
  // The server computes the modifier for required checks; the local lookup is
  // only a display fallback when the field is absent (e.g. modifier 0 omitted).
  const bonus = requiredCheck.modifier ?? (player ? getCheckBonus(player, checkLabel) : 0);

  // Clear the pending advance timer if the tray unmounts (e.g. page change)
  // before it fires, so the callback never runs against a stale closure.
  useEffect(() => () => { if (timerRef.current !== null) window.clearTimeout(timerRef.current); }, []);

  function roll() {
    if (!player) return;
    const natural = randomD20();
    const total = natural + bonus;
    const success = total >= requiredCheck.dc;
    const text = lang === 'en'
      ? `${player.name} rolls ${requiredCheck.ability} (${requiredCheck.skill}): d20 ${bonus >= 0 ? '+' : ''}${bonus} = ${total} (die ${natural}), DC ${requiredCheck.dc} — ${success ? 'success' : 'failure'}.`
      : `${player.name}進行 ${requiredCheck.ability}（${requiredCheck.skill}）檢定：d20 ${bonus >= 0 ? '+' : ''}${bonus} = ${total}（骰面 ${natural}），目標 DC ${requiredCheck.dc}，${success ? '成功' : '失敗'}。`;
    const rollResult = { natural, total, modifier: bonus, success, text };
    setResult({ natural, total });
    onRoll?.(rollResult);
    timerRef.current = window.setTimeout(() => { timerRef.current = null; onRequiredRoll(rollResult); }, 900);
  }

  return (
    <section className="dice-tray required-dice-tray" role="alert" aria-label={tz('必要檢定')}>
      <div className="section-heading compact"><div><p className="eyebrow">{tz('必要檢定')}</p><h2>{tz('現在擲 d20')}</h2></div><DiceFive size={20} /></div>
      <div className="required-roll">
        <strong>{player?.name}：{requiredCheck.ability}（{requiredCheck.skill}）</strong>
        <p>{lang === 'en' ? `Roll a d20, add your ${checkLabel} modifier of ${bonus >= 0 ? '+' : ''}${bonus}; the total must reach DC ${requiredCheck.dc}.` : `擲一顆二十面骰，加上 ${checkLabel} 加值 ${bonus >= 0 ? '+' : ''}${bonus}，總值需達到 DC ${requiredCheck.dc}。`}</p>
        <small>{requiredCheck.reason}</small>
        <AnimatePresence mode="wait">{result && <motion.div className="required-roll-result" initial={{ opacity: 0, scale: .85 }} animate={{ opacity: 1, scale: 1 }}><strong>{result.total}</strong><span>{tz('骰面')} {result.natural} {bonus >= 0 ? '+' : ''}{bonus}／{result.total >= requiredCheck.dc ? tz('成功') : tz('失敗')}</span></motion.div>}</AnimatePresence>
        <MagneticButton disabled={Boolean(result)} onClick={roll}>{result ? tz('已完成檢定') : tz('擲 d20 並自動加值')}</MagneticButton>
      </div>
    </section>
  );
}
