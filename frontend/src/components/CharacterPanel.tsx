import { lazy, Suspense, useEffect, useState } from 'react';
import { BookOpenText, MagicWand, Shield, Sword } from '@phosphor-icons/react';
import type { CharacterSpell, Choice, PlayerCharacter, PlayerId, RestType, XpProgress } from '../types';
import { abilityLabels, abilityModifier } from '../labels';
import { ActionComposer } from './ActionComposer';
import { StatHint } from './StatHint';

const CharacterSheet = lazy(() => import('./CharacterSheet').then((module) => ({ default: module.CharacterSheet })));

interface CharacterPanelProps {
  player: PlayerCharacter;
  // Server-computed XP progress for this player (view.xpProgress[player.id]).
  xp?: XpProgress;
  showStatHints?: boolean;
  combatActive?: boolean;
  onResourceChange: (id: PlayerCharacter['id'], resourceId: string, delta: number) => void;
  spellTargets: Array<{ id: string; name: string; side: 'party' | 'enemy' }>;
  /** Opens cast UI (modal); ritual/target are chosen there. */
  onCastSpell: (id: PlayerCharacter['id'], spell: CharacterSpell, asRitual?: boolean, targetId?: string) => void;
  onRest: (id: PlayerCharacter['id'], type: RestType) => void;
  onGeneratePortrait: (player: PlayerCharacter, appearance: string) => Promise<void>;
  pending?: string;
  actionDisabled: boolean;
  partySize: number;
  choices?: Choice[];
  resourceSummary?: string;
  /** Scripted campaign: the composer offers choices only, no free text. */
  scripted?: boolean;
  onSubmitAction: (player: PlayerId, text: string) => void;
  onUnlockAction: (player: PlayerId) => void;
}

type QuickTab = 'action' | 'basic' | 'magic' | 'equipment' | 'features';

const tabs: Array<{ id: QuickTab; label: string }> = [
  { id: 'action', label: '本回合行動' },
  { id: 'basic', label: '基本資訊' },
  { id: 'magic', label: '法術／資源' },
  { id: 'equipment', label: '攻擊／裝備' },
  { id: 'features', label: '能力特性' },
];

function signed(value: number) {
  return value >= 0 ? `+${value}` : String(value);
}

// Rough visual grouping for equipment chips by item-name keywords.
function equipmentKindClass(name: string): string {
  if (/[劍弓斧鎚矛匕杖鞭弩箭]/.test(name)) return 'equip-weapon';
  if (/[甲盾盔]/.test(name)) return 'equip-armor';
  if (/藥|劑|卷軸/.test(name)) return 'equip-potion';
  return 'equip-gear';
}

export function CharacterPanel({ player, xp, showStatHints = true, combatActive = false, spellTargets, onResourceChange, onCastSpell, onRest, onGeneratePortrait, pending, actionDisabled, partySize, choices, resourceSummary, scripted = false, onSubmitAction, onUnlockAction }: CharacterPanelProps) {
  const [sheetOpen, setSheetOpen] = useState(false);
  const [tab, setTab] = useState<QuickTab>('action');
  const [appearance, setAppearance] = useState(player.appearance || '');
  const [portraitLoading, setPortraitLoading] = useState(false);
  const hpRatio = Math.max(0, Math.min(100, (player.hp / player.maxHp) * 100));
  const experience: XpProgress = xp ?? { current: player.experience, required: player.experience, remaining: 0, ready: false, progress: 0 };
  useEffect(() => { if (pending) setTab('action'); }, [pending]);

  return (
    <article className="character-panel character-quick-panel">
      <div className="character-head">
        <div className={`character-sigil ${player.portraitUrl ? 'character-portrait' : ''}`} style={player.portraitUrl ? { backgroundImage: `url(${player.portraitUrl})` } : undefined} aria-hidden="true">{player.portraitUrl ? '' : player.initials}</div>
        <div>
          <p>{player.name}</p>
          <span>{player.species}・{player.className}／等級 {player.level}</span>
        </div>
        <Sword size={20} aria-hidden="true" />
      </div>

      <div className="quick-tabs" role="tablist" aria-label={`${player.name}角色資訊`}>
        {tabs.map((entry) => <button key={entry.id} type="button" role="tab" aria-selected={tab === entry.id} className={tab === entry.id ? 'selected' : ''} onClick={() => setTab(entry.id)}>{entry.label}</button>)}
      </div>

      <div className="quick-tab-content" role="tabpanel">
        {tab === 'action' && <ActionComposer player={player.id} name={player.name} className={player.className} pending={pending} disabled={actionDisabled} partySize={partySize} choices={choices} resourceSummary={resourceSummary} scripted={scripted} combatActive={combatActive} onSubmit={onSubmitAction} onUnlock={onUnlockAction} />}
        {tab === 'basic' && <>
          <section className="quick-portrait">
            {player.portraitUrl ? <img src={player.portraitUrl} alt={`${player.name}的角色肖像`} /> : <div className="quick-portrait-placeholder">尚未生成<br />角色圖</div>}
            <label><span>外觀描述</span><textarea value={appearance} maxLength={1200} placeholder="描述髮色、服裝、裝備、神情、年齡與明顯特徵……" onChange={(event) => setAppearance(event.target.value)} /></label>
            <button type="button" disabled={portraitLoading || !appearance.trim()} onClick={async () => { setPortraitLoading(true); await onGeneratePortrait(player, appearance); setPortraitLoading(false); }}><MagicWand />{portraitLoading ? '角色圖生成中…' : player.portraitUrl ? '重新生成角色圖' : '生成角色圖'}</button>
          </section>
          <div className="hp-block">
            <div className="hp-title"><StatHint hint="hp" enabled={showStatHints}><span>生命值／{player.condition}</span></StatHint><strong>{player.hp}<i>／{player.maxHp}</i>{player.temporaryHp ? <small> +{player.temporaryHp} 暫時</small> : null}</strong></div>
            <div className="hp-track"><span style={{ transform: `scaleX(${hpRatio / 100})` }} /></div>
          </div>
          <div className="quick-experience"><div><StatHint hint="experience" enabled={showStatHints}><span>等級 {player.level}／{player.experience.toLocaleString()} XP</span></StatHint><b>{player.level >= 20 ? '最高等級' : experience.ready ? '可升級' : `還差 ${experience.remaining.toLocaleString()} XP`}</b></div><div><span style={{ transform: `scaleX(${experience.progress})` }} /></div>{(player.abilityPoints || 0) > 0 && <small>有 {player.abilityPoints} 點能力值可在角色成長頁分配</small>}</div>
          <div className="quick-vitals"><StatHint hint="ac" enabled={showStatHints}><Shield />AC <b>{player.ac}</b></StatHint><StatHint hint="initiative" enabled={showStatHints}>先攻 <b>{signed(player.initiative)}</b></StatHint><StatHint hint="speed" enabled={showStatHints}>速度 <b>{player.speed}</b></StatHint><StatHint hint="passive" enabled={showStatHints}>被動察覺 <b>{player.passive}</b></StatHint></div>
          <div className="quick-abilities">{(Object.keys(abilityLabels) as Array<keyof typeof abilityLabels>).map((key) => <div key={key}><StatHint hint={key} enabled={showStatHints}><small>{abilityLabels[key]}</small></StatHint><strong>{player.abilities[key]}</strong><span>{signed(abilityModifier(player.abilities[key]))}</span></div>)}</div>
          <details className="quick-details"><summary>查看技能加值</summary><div className="quick-skills">{player.skills.map((skill) => <span key={skill.name} className={skill.proficient ? 'proficient' : ''}>{skill.name}<b>{signed(skill.bonus)}</b></span>)}</div></details>
        </>}

        {tab === 'magic' && <>
          {player.spellcasting ? <>
            <div className="quick-casting"><StatHint hint="spellAbility" enabled={showStatHints}>施法：{abilityLabels[player.spellcasting.ability]}</StatHint><StatHint hint="spellAttack" enabled={showStatHints}>攻擊 {signed(player.spellcasting.attackBonus)}</StatHint><StatHint hint="spellSaveDc" enabled={showStatHints}>DC {player.spellcasting.saveDc}</StatHint></div>
            <div className="quick-slots">{player.spellcasting.slots.map((slot) => <StatHint key={slot.level} hint="spellSlots" enabled={showStatHints}><b>{slot.level} 環 {slot.current}/{slot.max}</b></StatHint>)}{player.concentration && <b>專注：{player.concentration}</b>}</div>
            <p className="quick-spell-hint">點選法術開啟施法面板，再選目標／儀式後確認。</p>
            <div className="quick-spells">
              {player.spellcasting.spells.map((spell) => {
                const canCast = spell.level === 0 || spell.prepared || spell.alwaysPrepared;
                const hasFreeUse = Boolean(spell.freeUseResourceId && player.resources.some((entry) => entry.id === spell.freeUseResourceId && entry.current > 0));
                const hasSlot = spell.level === 0 || hasFreeUse || Boolean(player.spellcasting?.slots.some((slot) => slot.level >= spell.level && slot.current > 0));
                const rollRule = spell.effect?.attackRoll
                  ? '施放後擲法術攻擊'
                  : spell.effect?.saveAbility
                    ? `目標${abilityLabels[spell.effect.saveAbility]}豁免`
                    : spell.effect?.automaticHit
                      ? '自動命中'
                      : '';
                const levelLabel = spell.level === 0 ? '戲法' : `${spell.level} 環`;
                return (
                  <button
                    key={spell.id}
                    type="button"
                    className={`quick-spell-card${!canCast ? ' disabled' : ''}${canCast && !hasSlot ? ' no-slot' : ''}`}
                    disabled={!canCast}
                    onClick={() => onCastSpell(player.id, spell)}
                  >
                    <span className="quick-spell-level">{levelLabel}</span>
                    <span className="quick-spell-body">
                      <strong>{spell.name}</strong>
                      <small>
                        {spell.castingTime}・{spell.range}
                        {rollRule ? `・${rollRule}` : ''}
                        {!canCast ? '・未準備' : ''}
                        {spell.ritual ? '・可儀式' : ''}
                      </small>
                      <em>{spell.description}</em>
                    </span>
                    <span className="quick-spell-cta">
                      <MagicWand size={14} weight="fill" />
                      施放
                    </span>
                  </button>
                );
              })}
            </div>
          </> : <p className="quick-empty">這名角色沒有法術。</p>}
          {player.resources.length > 0 && <div className="quick-resources">{player.resources.map((resource) => <div key={resource.id}><span><strong>{resource.name}</strong><small>{resource.description}</small></span><b>{resource.current}/{resource.max}</b><button type="button" disabled={resource.current === 0} onClick={() => onResourceChange(player.id, resource.id, -1)}>使用</button></div>)}</div>}
        </>}

        {tab === 'equipment' && <>
          <div className="quick-attacks">{player.attacks.map((attack) => <div key={attack.id}><Sword /><span><strong>{attack.name}</strong><small>{attack.properties.join('・') || '一般攻擊'}</small></span><b>{signed(attack.attackBonus)}</b><em>{attack.damage} {attack.damageType}</em></div>)}</div>
          <div className="quick-gold" aria-label="金幣">
            <span className="quick-gold-coin" aria-hidden="true" />
            <strong>{player.gold ?? 0}</strong>
            <small>gp・寶箱與任務會增加，裝備商可買賣</small>
          </div>
          <ul className="quick-equipment">
            {player.equipment.map((item, index) => (
              <li key={`${item}-${index}`} className={equipmentKindClass(item)}>{item}</li>
            ))}
            {player.equipment.length === 0 && <li className="quick-equipment-empty">行囊空空如也</li>}
          </ul>
        </>}

        {tab === 'features' && <div className="quick-features">{player.features.map((feature) => <div key={feature.id}><strong>{feature.name}</strong><p>{feature.description}</p></div>)}</div>}
      </div>

      <div className={`rest-explanation ${combatActive ? 'rest-blocked' : ''}`}><span><strong>行動關係</strong> {combatActive ? '目前正在戰鬥，短休與長休都不可使用。' : '休息只消耗探索行動時間，不占戰鬥動作。'}</span><span><strong>短休／1 點</strong> 約 1 小時，自動使用生命骰補血並恢復短休資源。</span><span><strong>長休／4 點</strong> 約 8 小時，恢復生命、生命骰、所有法術位與職業資源。</span></div>
      <div className="quick-footer"><div><button type="button" disabled={combatActive} title={combatActive ? '戰鬥中不能休息' : undefined} onClick={() => onRest(player.id, 'short')}>短休／1 點</button><button type="button" disabled={combatActive} title={combatActive ? '戰鬥中不能休息' : undefined} onClick={() => onRest(player.id, 'long')}>長休／4 點</button></div><button type="button" className="open-sheet" onClick={() => setSheetOpen(true)}><BookOpenText />進階完整角色卡</button></div>
      {sheetOpen && <Suspense fallback={<div className="sheet-loading" role="status">正在開啟完整角色卡…</div>}><CharacterSheet player={player} showStatHints={showStatHints} combatActive={combatActive} open={sheetOpen} onClose={() => setSheetOpen(false)} spellTargets={spellTargets} onResourceChange={onResourceChange} onCastSpell={onCastSpell} onRest={onRest} /></Suspense>}
    </article>
  );
}
