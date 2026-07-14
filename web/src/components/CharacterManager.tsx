import { useEffect, useState } from 'react';
import { ArrowUp, FloppyDisk, MagicWand, UserGear } from '@phosphor-icons/react';
import type { AbilityKey, AbilityScores, PlayerCharacter } from '../types';
import { abilityLabels, classNames, type ClassName } from '../rules/characters';
import { customizeCharacter, getCharacterClasses, levelUpCharacter, setPreparedSpells } from '../rules/advancement';
import { spellCatalog } from '../rules/spells';

interface CharacterManagerProps {
  players: PlayerCharacter[];
  onUpdate: (player: PlayerCharacter) => void;
  onLog: (text: string) => void;
}

function CharacterEditor({ player, onUpdate, onLog }: { player: PlayerCharacter; onUpdate: (player: PlayerCharacter) => void; onLog: (text: string) => void }) {
  const [species, setSpecies] = useState(player.species);
  const [background, setBackground] = useState(player.background);
  const [abilities, setAbilities] = useState<AbilityScores>(player.abilities);
  const [nextClass, setNextClass] = useState<ClassName>((getCharacterClasses(player)[0]?.className as ClassName) || '戰士');
  useEffect(() => { setSpecies(player.species); setBackground(player.background); setAbilities(player.abilities); }, [player]);

  function saveProfile() {
    onUpdate(customizeCharacter(player, { species: species.trim(), background: background.trim(), abilities }));
    onLog(`${player.name}的種族、背景與能力值配置已更新。`);
  }

  function levelUp() {
    try {
      const updated = levelUpCharacter(player, nextClass);
      onUpdate(updated);
      onLog(`${player.name}升至 ${updated.level} 級，新增 ${nextClass} 1 個職業等級。`);
    } catch (error) {
      onLog(error instanceof Error ? error.message : String(error));
    }
  }

  function toggleSpell(id: string) {
    const selected = new Set(player.spellcasting?.spells.map((spell) => spell.id) || []);
    selected.has(id) ? selected.delete(id) : selected.add(id);
    onUpdate(setPreparedSpells(player, [...selected]));
  }

  return (
    <article className="character-editor">
      <header><div className="character-sigil">{player.initials}</div><div><h2>{player.name}</h2><p>{getCharacterClasses(player).map((entry) => `${entry.className} ${entry.level}`).join('／')}・總等級 {player.level}</p></div><UserGear size={24} /></header>
      <div className="profile-fields">
        <label>自訂種族<input value={species} maxLength={80} onChange={(event) => setSpecies(event.target.value)} /></label>
        <label>自訂背景<input value={background} maxLength={80} onChange={(event) => setBackground(event.target.value)} /></label>
      </div>
      <div className="ability-editor">
        {(Object.keys(abilityLabels) as AbilityKey[]).map((key) => (
          <label key={key}>{abilityLabels[key]}<input type="number" min="3" max="30" value={abilities[key]} onChange={(event) => setAbilities({ ...abilities, [key]: Math.min(30, Math.max(3, Number(event.target.value))) })} /></label>
        ))}
      </div>
      <button type="button" onClick={saveProfile}><FloppyDisk />儲存角色配置</button>
      <section className="level-up-panel">
        <div><strong>升級／多職業</strong><span>總等級上限 20；選原職業為升級，選其他職業為多職業。</span></div>
        <select value={nextClass} onChange={(event) => setNextClass(event.target.value as ClassName)}>{classNames.map((name) => <option key={name}>{name}</option>)}</select>
        <button type="button" onClick={levelUp} disabled={player.level >= 20}><ArrowUp />升級</button>
      </section>
      {player.spellcasting && (
        <details className="spell-config">
          <summary><MagicWand />法術配置（{player.spellcasting.spells.length}）</summary>
          <div>{Object.values(spellCatalog).map((spell) => {
            const checked = player.spellcasting?.spells.some((entry) => entry.id === spell.id) || false;
            return <label key={spell.id}><input type="checkbox" checked={checked} onChange={() => toggleSpell(spell.id)} /><span><strong>{spell.name}</strong><small>{spell.level === 0 ? '戲法' : `${spell.level} 環`}・{spell.school}</small></span></label>;
          })}</div>
        </details>
      )}
    </article>
  );
}

export function CharacterManager({ players, onUpdate, onLog }: CharacterManagerProps) {
  return (
    <main className="single-page character-manager-page">
      <div className="page-intro"><p className="eyebrow">Character workshop</p><h2>角色成長與自訂</h2><p>修改會保留目前生命百分比與故事進度；升級支援總等級 1–20 與多職業。</p></div>
      <div className="character-editor-grid">{players.map((player) => <CharacterEditor key={player.id} player={player} onUpdate={onUpdate} onLog={onLog} />)}</div>
    </main>
  );
}
