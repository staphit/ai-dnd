import { useMemo, useState } from 'react';
import { ArrowClockwise, Crosshair, Plus, Shield, Sword, X } from '@phosphor-icons/react';
import type { CombatState, Combatant, PlayerCharacter } from '../types';
import { advanceTurn, partyCombatants, resolveAttack, startCombat } from '../rules/combat';

interface CombatTrackerProps {
  players: PlayerCharacter[];
  combat?: CombatState;
  onChange: (combat: CombatState) => void;
  onLog: (text: string) => void;
}

const emptyEnemy = { name: '骸骨守衛', ac: 13, hp: 13, initiativeBonus: 2, attackBonus: 4, damage: '1d6+2', damageType: '穿刺' };

export function CombatTracker({ players, combat, onChange, onLog }: CombatTrackerProps) {
  const [enemies, setEnemies] = useState<Combatant[]>([]);
  const [draft, setDraft] = useState(emptyEnemy);
  const [targetId, setTargetId] = useState('');
  const [attackId, setAttackId] = useState('');
  const current = combat?.combatants[combat.turnIndex];
  const currentPlayer = players.find((player) => player.id === current?.playerId);
  const availableAttacks = currentPlayer?.attacks || [];
  const validTargets = useMemo(() => combat?.combatants.filter((entry) => !entry.defeated && entry.id !== current?.id) || [], [combat, current]);

  function addEnemy() {
    const enemy: Combatant = {
      id: `enemy-${crypto.randomUUID()}`,
      side: 'enemy',
      initiative: 0,
      maxHp: Math.max(1, draft.hp),
      defeated: false,
      ...draft,
      name: draft.name.trim() || '未命名敵人',
      hp: Math.max(1, draft.hp),
    };
    setEnemies((list) => [...list, enemy]);
  }

  function begin() {
    const state = startCombat([...partyCombatants(players), ...enemies]);
    onChange(state);
    onLog(`戰鬥開始。先攻順序：${state.combatants.map((entry) => `${entry.name} ${entry.initiative}`).join(' → ')}`);
  }

  function attack() {
    if (!combat || !current) return;
    const target = targetId || validTargets[0]?.id;
    if (!target) return;
    try {
      const chosenAttack = availableAttacks.find((entry) => entry.id === attackId) || availableAttacks[0];
      const prepared = chosenAttack ? { ...combat, combatants: combat.combatants.map((entry) => entry.id === current.id ? { ...entry, attackBonus: chosenAttack.attackBonus, damage: chosenAttack.damage, damageType: chosenAttack.damageType } : entry) } : combat;
      const result = resolveAttack(prepared, current.id, target);
      onLog(result.resolution.text);
      onChange(advanceTurn(result.state));
      setTargetId('');
    } catch (error) {
      onLog(error instanceof Error ? error.message : String(error));
    }
  }

  if (!combat?.active) {
    return (
      <section className="combat-console">
        <header><div><p className="eyebrow">Encounter setup</p><h2>建立戰鬥</h2></div><Sword size={24} /></header>
        <p className="muted-copy">玩家會自動加入。新增敵人後擲先攻；每次攻擊會自動判斷命中、重擊、傷害並前進至下一位。</p>
        <div className="enemy-builder">
          <label>名稱<input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
          <label>AC<input type="number" min="1" max="40" value={draft.ac} onChange={(event) => setDraft({ ...draft, ac: Number(event.target.value) })} /></label>
          <label>HP<input type="number" min="1" max="999" value={draft.hp} onChange={(event) => setDraft({ ...draft, hp: Number(event.target.value) })} /></label>
          <label>先攻<input type="number" min="-10" max="20" value={draft.initiativeBonus} onChange={(event) => setDraft({ ...draft, initiativeBonus: Number(event.target.value) })} /></label>
          <label>命中<input type="number" min="-10" max="30" value={draft.attackBonus} onChange={(event) => setDraft({ ...draft, attackBonus: Number(event.target.value) })} /></label>
          <label>傷害<input value={draft.damage} pattern="\d+d\d+([+-]\d+)?" onChange={(event) => setDraft({ ...draft, damage: event.target.value })} /></label>
          <button type="button" onClick={addEnemy}><Plus />加入敵人</button>
        </div>
        <div className="enemy-roster">
          {enemies.map((enemy) => <span key={enemy.id}>{enemy.name}／AC {enemy.ac}／HP {enemy.hp}<button type="button" onClick={() => setEnemies((list) => list.filter((entry) => entry.id !== enemy.id))}><X /></button></span>)}
        </div>
        <button type="button" className="primary-action" onClick={begin} disabled={enemies.length === 0}><Crosshair />擲先攻並開始</button>
      </section>
    );
  }

  return (
    <section className="combat-console">
      <header><div><p className="eyebrow">第 {combat.round} 輪</p><h2>{current?.name} 的回合</h2></div><Sword size={24} /></header>
      <div className="initiative-list">
        {combat.combatants.map((entry, index) => (
          <article key={entry.id} className={`${index === combat.turnIndex ? 'initiative-active' : ''} ${entry.defeated ? 'initiative-defeated' : ''}`}>
            <b>{entry.initiative}</b><span><strong>{entry.name}</strong><small>{entry.side === 'party' ? '隊伍' : '敵方'}</small></span><em><Shield />{entry.ac}</em><i>{entry.hp}/{entry.maxHp} HP</i>
          </article>
        ))}
      </div>
      <div className="combat-actions">
        {availableAttacks.length > 0 && <label>攻擊方式<select value={attackId || availableAttacks[0]?.id} onChange={(event) => setAttackId(event.target.value)}>{availableAttacks.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}／命中 +{entry.attackBonus}／{entry.damage}</option>)}</select></label>}
        <label>攻擊目標<select value={targetId} onChange={(event) => setTargetId(event.target.value)}>{validTargets.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}（AC {entry.ac}）</option>)}</select></label>
        <button type="button" className="primary-action" onClick={attack} disabled={!validTargets.length}><Crosshair />攻擊並結算</button>
        <button type="button" onClick={() => onChange(advanceTurn(combat))}><ArrowClockwise />跳過回合</button>
        <button type="button" onClick={() => onChange({ ...combat, active: false })}><X />結束戰鬥</button>
      </div>
    </section>
  );
}
