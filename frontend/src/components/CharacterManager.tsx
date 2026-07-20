import { useEffect, useState } from 'react';
import { ArrowUp, FloppyDisk, MagicWand, UserGear } from '@phosphor-icons/react';
import type { AbilityKey, PlayerCharacter, PlayerId, XpProgress } from '../types';
import { abilityLabels } from '../labels';
import { getCatalog, type RulesCatalog } from '../api';
import { StatHint } from './StatHint';

interface CharacterManagerProps {
  players: PlayerCharacter[];
  xpProgress?: Partial<Record<PlayerId, XpProgress>>;
  showStatHints?: boolean;
  onLevelUp: (playerId: PlayerId, className: string) => void;
  onSpendAbilityPoint: (playerId: PlayerId, ability: AbilityKey) => void;
  onSetPreparedSpells: (playerId: PlayerId, spellIds: string[]) => void;
  onSaveProfile: (playerId: PlayerId, patch: { species: string; background: string; appearance: string }) => void;
  onGeneratePortrait: (player: PlayerCharacter, appearance: string) => Promise<void>;
}

interface CharacterEditorProps {
  player: PlayerCharacter;
  xp?: XpProgress;
  catalog: RulesCatalog | null;
  showStatHints: boolean;
  onLevelUp: (playerId: PlayerId, className: string) => void;
  onSpendAbilityPoint: (playerId: PlayerId, ability: AbilityKey) => void;
  onSetPreparedSpells: (playerId: PlayerId, spellIds: string[]) => void;
  onSaveProfile: (playerId: PlayerId, patch: { species: string; background: string; appearance: string }) => void;
  onGeneratePortrait: (player: PlayerCharacter, appearance: string) => Promise<void>;
}

function characterClasses(player: PlayerCharacter) {
  return player.classLevels?.length ? player.classLevels : [{ className: player.className, level: player.level, subclass: player.subclass }];
}

function CharacterEditor({ player, xp, catalog, showStatHints, onLevelUp, onSpendAbilityPoint, onSetPreparedSpells, onSaveProfile, onGeneratePortrait }: CharacterEditorProps) {
  const [species, setSpecies] = useState(player.species);
  const [background, setBackground] = useState(player.background);
  const [appearance, setAppearance] = useState(player.appearance || '');
  const [portraitLoading, setPortraitLoading] = useState(false);
  const experience: XpProgress = xp ?? { current: player.experience, required: player.experience, remaining: 0, ready: false, progress: 0 };
  useEffect(() => { setSpecies(player.species); setBackground(player.background); setAppearance(player.appearance || ''); }, [player]);

  function toggleSpell(id: string) {
    const selected = new Set(player.spellcasting?.spells.map((spell) => spell.id) || []);
    if (selected.has(id)) selected.delete(id);
    else selected.add(id);
    onSetPreparedSpells(player.id, [...selected]);
  }

  return (
    <article className="character-editor">
      <header><div className="character-sigil">{player.initials}</div><div><h2>{player.name}</h2><p>{characterClasses(player).map((entry) => `${entry.className} ${entry.level}`).join('／')}・總等級 {player.level}</p></div><UserGear size={24} /></header>
      <div className="profile-fields">
        <label>自訂種族<input value={species} maxLength={80} onChange={(event) => setSpecies(event.target.value)} /></label>
        <label>自訂背景<input value={background} maxLength={80} onChange={(event) => setBackground(event.target.value)} /></label>
      </div>
      <section className="portrait-editor">
        {player.portraitUrl ? <img src={player.portraitUrl} alt={`${player.name}的角色肖像`} /> : <div className="portrait-placeholder">{player.initials}</div>}
        <label>角色外觀描述<textarea value={appearance} maxLength={1200} placeholder="髮色、服裝、裝備、神情、年齡與明顯特徵……" onChange={(event) => setAppearance(event.target.value)} /></label>
        <button type="button" disabled={portraitLoading || !appearance.trim()} onClick={async () => { setPortraitLoading(true); await onGeneratePortrait(player, appearance); setPortraitLoading(false); }}><MagicWand />{portraitLoading ? '生成中…' : '生成角色圖'}</button>
      </section>
      <section className="experience-panel"><div><StatHint hint="experience" enabled={showStatHints}><strong>{player.experience.toLocaleString()} XP</strong></StatHint><span>{player.level >= 20 ? '已達最高等級' : experience.ready ? `已達 ${player.level + 1} 級門檻` : `距離 ${player.level + 1} 級還差 ${experience.remaining.toLocaleString()} XP`}</span></div><div className="experience-track"><span style={{ transform: `scaleX(${experience.progress})` }} /></div></section>
      <div className="ability-editor ability-allocation">
        {(Object.keys(abilityLabels) as AbilityKey[]).map((key) => (
          <div key={key}><StatHint hint={key} enabled={showStatHints}><span>{abilityLabels[key]}</span></StatHint><strong>{player.abilities[key]}</strong><button type="button" aria-label={`提升${abilityLabels[key]}`} disabled={(player.abilityPoints || 0) < 1} onClick={() => onSpendAbilityPoint(player.id, key)}>＋</button></div>
        ))}
      </div>
      <p className="ability-points">可分配能力值點數：<strong>{player.abilityPoints || 0}</strong>（總等級 4、8、12、16、19 時各獲得 2 點）</p>
      <button type="button" onClick={() => onSaveProfile(player.id, { species: species.trim(), background: background.trim(), appearance: appearance.trim() })}><FloppyDisk />儲存角色配置</button>
      <section className="level-up-panel">
        <div><strong>升級（{player.className}）</strong><span>{player.level >= 20 ? '已達最高等級。' : experience.ready ? 'XP 已足夠；升級後會解鎖生命、熟練、職業與法術進展。' : `需要 ${experience.required.toLocaleString()} XP；目前 ${player.experience.toLocaleString()} XP。`}</span></div>
        <button type="button" onClick={() => onLevelUp(player.id, player.className)} disabled={player.level >= 20 || !experience.ready}><ArrowUp />升級</button>
      </section>
      {player.spellcasting && (
        <details className="spell-config">
          <summary><MagicWand />法術配置（{player.spellcasting.spells.length}）</summary>
          <div>{(catalog?.spells || []).map((spell) => {
            const checked = player.spellcasting?.spells.some((entry) => entry.id === spell.id) || false;
            return <label key={spell.id}><input type="checkbox" checked={checked} onChange={() => toggleSpell(spell.id)} /><span><strong>{spell.name}</strong><small>{spell.level === 0 ? '戲法' : `${spell.level} 環`}・{spell.school}</small></span></label>;
          })}</div>
        </details>
      )}
    </article>
  );
}

export function CharacterManager({ players, xpProgress, showStatHints = true, onLevelUp, onSpendAbilityPoint, onSetPreparedSpells, onSaveProfile, onGeneratePortrait }: CharacterManagerProps) {
  const [catalog, setCatalog] = useState<RulesCatalog | null>(null);

  useEffect(() => {
    let mounted = true;
    getCatalog().then((data) => { if (mounted) setCatalog(data); }).catch(() => { /* selects fall back to the current class */ });
    return () => { mounted = false; };
  }, []);

  return (
    <main className="single-page character-manager-page">
      <div className="page-intro"><p className="eyebrow">Character workshop</p><h2>經驗、升級與成長</h2><p>完成探索、任務與戰鬥可獲得 XP；達到門檻後才能升級。生命、熟練、職業能力與法術位會自動進展，指定等級另可分配能力值點數。</p></div>
      <div className="character-editor-grid">{players.map((player) => <CharacterEditor key={player.id} player={player} xp={xpProgress?.[player.id]} catalog={catalog} showStatHints={showStatHints} onLevelUp={onLevelUp} onSpendAbilityPoint={onSpendAbilityPoint} onSetPreparedSpells={onSetPreparedSpells} onSaveProfile={onSaveProfile} onGeneratePortrait={onGeneratePortrait} />)}</div>
    </main>
  );
}
