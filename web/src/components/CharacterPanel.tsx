import { useState } from 'react';
import { BookOpenText, Minus, Plus, Shield, Sword } from '@phosphor-icons/react';
import type { CharacterSpell, PlayerCharacter, RestType } from '../types';
import { CharacterSheet } from './CharacterSheet';

interface CharacterPanelProps {
  player: PlayerCharacter;
  onHpChange: (id: PlayerCharacter['id'], delta: number) => void;
  onResourceChange: (id: PlayerCharacter['id'], resourceId: string, delta: number) => void;
  onCastSpell: (id: PlayerCharacter['id'], spell: CharacterSpell, asRitual: boolean) => void;
  onRest: (id: PlayerCharacter['id'], type: RestType) => void;
}

export function CharacterPanel({ player, onHpChange, onResourceChange, onCastSpell, onRest }: CharacterPanelProps) {
  const [sheetOpen, setSheetOpen] = useState(false);
  const hpRatio = Math.max(0, Math.min(100, (player.hp / player.maxHp) * 100));

  return (
    <article className="character-panel">
      <div className="character-head">
        <div className="character-sigil" aria-hidden="true">{player.initials}</div>
        <div>
          <p>{player.name}</p>
          <span>{player.className}／等級 {player.level}</span>
        </div>
        <Sword size={20} aria-hidden="true" />
      </div>
      <div className="hp-block">
        <div className="hp-title">
          <span>生命值</span>
          <strong>{player.hp}<i>／{player.maxHp}</i></strong>
        </div>
        <div className="hp-track"><span style={{ transform: `scaleX(${hpRatio / 100})` }} /></div>
        <div className="hp-controls">
          <button type="button" onClick={() => onHpChange(player.id, -1)} aria-label={`${player.name} 失去一點生命`}><Minus size={15} /></button>
          <span>{player.condition}</span>
          <button type="button" onClick={() => onHpChange(player.id, 1)} aria-label={`${player.name} 恢復一點生命`}><Plus size={15} /></button>
        </div>
      </div>
      <div className="stat-line">
        <div><Shield size={17} /><span>護甲</span><strong>{player.ac}</strong></div>
        <div><span>被動察覺</span><strong>{player.passive}</strong></div>
      </div>
      {player.spellcasting ? (
        <div className="slots"><span>法術位</span><div>{player.spellcasting.slots.map((slot) => <b key={slot.level}>{slot.level}環 {slot.current}/{slot.max}</b>)}</div></div>
      ) : (
        <div className="slots"><span>職業資源</span><div>{player.resources.slice(0, 2).map((entry) => <b key={entry.id}>{entry.name} {entry.current}/{entry.max}</b>)}</div></div>
      )}
      <button type="button" className="open-sheet" onClick={() => setSheetOpen(true)}><BookOpenText />完整角色卡</button>
      <CharacterSheet player={player} open={sheetOpen} onClose={() => setSheetOpen(false)} onHpChange={onHpChange} onResourceChange={onResourceChange} onCastSpell={onCastSpell} onRest={onRest} />
    </article>
  );
}
