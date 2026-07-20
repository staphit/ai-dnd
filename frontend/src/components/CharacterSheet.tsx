import { useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Bed, BookOpenText, Minus, Shield, Sparkle, Sword, Timer, X } from '@phosphor-icons/react';
import type { CharacterSpell, PlayerCharacter, RestType } from '../types';
import { abilityLabels, abilityModifier } from '../labels';
import { StatHint } from './StatHint';

interface CharacterSheetProps {
  player: PlayerCharacter;
  showStatHints?: boolean;
  combatActive?: boolean;
  open: boolean;
  onClose: () => void;
  onResourceChange: (id: PlayerCharacter['id'], resourceId: string, delta: number) => void;
  spellTargets: Array<{ id: string; name: string; side: 'party' | 'enemy' }>;
  onCastSpell: (id: PlayerCharacter['id'], spell: CharacterSpell, asRitual: boolean, targetId?: string) => void;
  onRest: (id: PlayerCharacter['id'], type: RestType) => void;
}

function signed(value: number) {
  return value >= 0 ? `+${value}` : String(value);
}

export function CharacterSheet({ player, showStatHints = true, combatActive = false, open, onClose, spellTargets, onResourceChange, onCastSpell, onRest }: CharacterSheetProps) {
  const [spellTarget, setSpellTarget] = useState<Record<string, string>>({});
  // Free-text location for spells cast at the scene rather than a combatant.
  const [sceneTarget, setSceneTarget] = useState<Record<string, string>>({});
  const spellGroups = player.spellcasting
    ? [...new Set(player.spellcasting.spells.map((spell) => spell.level))].sort((a, b) => a - b)
    : [];

  return (
    <AnimatePresence>
      {open && (
        <motion.div className="sheet-backdrop" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onMouseDown={onClose}>
          <motion.article
            className="character-sheet"
            role="dialog"
            aria-modal="true"
            aria-label={`${player.name} 完整角色卡`}
            initial={{ opacity: 0, x: 40 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 30 }}
            transition={{ type: 'spring', stiffness: 120, damping: 22 }}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <header className="sheet-head">
              <div>
                <p className="eyebrow">2024 角色卡／等級 {player.level}</p>
                <h2>{player.name}</h2>
                <span>{player.species}・{player.background}・{player.className}／{player.subclass}</span>
              </div>
              <button type="button" onClick={onClose} aria-label="關閉角色卡"><X size={21} /></button>
            </header>

            <section className="sheet-vitals">
              <div><StatHint hint="hp" enabled={showStatHints}><small>生命</small></StatHint><strong>{player.hp}<i>／{player.maxHp}</i></strong><span>{player.temporaryHp ? `暫時生命 ${player.temporaryHp}` : player.condition}</span></div>
              <div><StatHint hint="ac" enabled={showStatHints}><small>護甲</small></StatHint><strong>{player.ac}</strong><Shield /></div>
              <div><StatHint hint="initiative" enabled={showStatHints}><small>先攻</small></StatHint><strong>{signed(player.initiative)}</strong><StatHint hint="speed" enabled={showStatHints}><span>速度 {player.speed} 呎</span></StatHint></div>
              <div><StatHint hint="proficiency" enabled={showStatHints}><small>熟練</small></StatHint><strong>+{player.proficiencyBonus}</strong><StatHint hint="passive" enabled={showStatHints}><span>被動察覺 {player.passive}</span></StatHint></div>
            </section>

            <section className="sheet-section">
              <div className="sheet-section-title"><span>能力與豁免</span><small>ABILITY SCORES</small></div>
              <div className="ability-grid">
                {(Object.keys(abilityLabels) as Array<keyof typeof abilityLabels>).map((ability) => (
                  <div key={ability}>
                    <StatHint hint={ability} enabled={showStatHints}><small>{abilityLabels[ability]}{player.savingThrowProficiencies.includes(ability) ? ' ◆' : ''}</small></StatHint>
                    <strong>{player.abilities[ability]}</strong>
                    <span>{signed(abilityModifier(player.abilities[ability]))}</span>
                  </div>
                ))}
              </div>
            </section>

            <section className="sheet-two-column">
              <div className="sheet-section">
                <div className="sheet-section-title"><span>技能</span><small>SKILLS</small></div>
                <div className="skill-list">
                  {player.skills.map((skill) => (
                    <div key={skill.name} className={skill.proficient ? 'skill-proficient' : ''}>
                      <i>{skill.expertise ? '◆' : skill.proficient ? '●' : '○'}</i><span>{skill.name}</span><small>{abilityLabels[skill.ability]}</small><strong>{signed(skill.bonus)}</strong>
                    </div>
                  ))}
                </div>
              </div>
              <div>
                <section className="sheet-section">
                  <div className="sheet-section-title"><span>攻擊</span><small>ATTACKS</small></div>
                  <div className="attack-list">
                    {player.attacks.map((entry) => (
                      <div key={entry.id}><Sword /><span><strong>{entry.name}</strong><small>{entry.properties.join('・')}</small></span><b>{signed(entry.attackBonus)}</b><em>{entry.damage} {entry.damageType}</em></div>
                    ))}
                  </div>
                </section>

                {player.resources.length > 0 && (
                  <section className="sheet-section">
                    <div className="sheet-section-title"><span>職業資源</span><small>RESOURCES</small></div>
                    <div className="resource-list">
                      {player.resources.map((entry) => (
                        <div key={entry.id}>
                          <span><strong>{entry.name}{entry.die ? ` ${entry.die}` : ''}</strong><small>{entry.description}</small></span>
                          <div><b>{entry.current}／{entry.max}</b><button type="button" onClick={() => onResourceChange(player.id, entry.id, -1)} disabled={entry.current === 0}><Minus />使用</button></div>
                        </div>
                      ))}
                    </div>
                  </section>
                )}
              </div>
            </section>

            {player.spellcasting && (
              <section className="sheet-section spellbook-section">
                <div className="sheet-section-title"><span>法術與法術位</span><small>SPELLBOOK</small></div>
                <div className="spellcasting-summary">
                  <div><StatHint hint="spellAbility" enabled={showStatHints}><small>施法屬性</small></StatHint><strong>{abilityLabels[player.spellcasting.ability]}</strong></div>
                  <div><StatHint hint="spellAttack" enabled={showStatHints}><small>法術攻擊</small></StatHint><strong>{signed(player.spellcasting.attackBonus)}</strong></div>
                  <div><StatHint hint="spellSaveDc" enabled={showStatHints}><small>豁免 DC</small></StatHint><strong>{player.spellcasting.saveDc}</strong></div>
                  <div><small>法器</small><strong>{player.spellcasting.focus}</strong></div>
                  {player.spellcasting.slots.map((slot) => (
                    <div key={slot.level} className="slot-counter"><StatHint hint="spellSlots" enabled={showStatHints}><small>{slot.level} 環法術位</small></StatHint><strong>{slot.current}／{slot.max}</strong></div>
                  ))}
                </div>
                {player.concentration && <p className="concentration-mark"><Sparkle />正在專注：{player.concentration}</p>}
                <div className="spell-groups">
                  {spellGroups.map((level) => (
                    <div key={level} className="spell-group">
                      <h3>{level === 0 ? '戲法' : `${level} 環法術`}</h3>
                      {player.spellcasting?.spells.filter((spell) => spell.level === level).map((spell) => {
                        const canCastNormally = spell.level === 0 || spell.prepared || spell.alwaysPrepared;
                        const canRitual = spell.ritual && (spell.prepared || spell.inSpellbook);
                        const hasFreeUse = Boolean(spell.freeUseResourceId && player.resources.some((entry) => entry.id === spell.freeUseResourceId && entry.current > 0));
                        const hasSlot = spell.level === 0 || hasFreeUse || Boolean(player.spellcasting?.slots.some((slot) => slot.level >= spell.level && slot.current > 0));
                        const candidates = spell.effect?.target === 'self' || /自身/.test(spell.range)
                          ? spellTargets.filter((entry) => entry.id === player.id)
                          : spell.effect?.target === 'ally'
                            ? spellTargets.filter((entry) => entry.side === 'party')
                            : spell.effect?.target === 'creature'
                              ? spellTargets.filter((entry) => entry.side === 'enemy')
                              : [...spellTargets, { id: 'scene', name: '目前場景／指定位置', side: 'party' as const }];
                        const selectedTarget = spellTarget[spell.id] || ((spell.effect?.target === 'self' || /自身/.test(spell.range)) ? player.id : undefined);
                        const isSceneTarget = selectedTarget === 'scene';
                        const resolvedTarget = isSceneTarget ? (sceneTarget[spell.id]?.trim() || 'scene') : selectedTarget;
                        return (
                          <article key={spell.id} className={!canCastNormally ? 'spell-unprepared' : ''}>
                            <div className="spell-name"><BookOpenText /><span><strong>{spell.name}</strong><small>{spell.englishName}・{spell.school}{spell.alwaysPrepared ? '・常備' : !spell.prepared ? '・未準備' : ''}</small></span></div>
                            <div className="spell-tags"><span>{spell.castingTime}</span><span>{spell.range}</span>{spell.concentration && <span>專注</span>}{spell.ritual && <span>儀式</span>}</div>
                            <p>{spell.description}</p>
                            <div className="spell-actions">
                              <select aria-label={`${spell.name}目標`} value={selectedTarget || ''} onChange={(event) => setSpellTarget((current) => ({ ...current, [spell.id]: event.target.value }))}><option value="" disabled>必須指定目標</option>{candidates.map((target) => <option key={target.id} value={target.id}>{target.name}</option>)}</select>
                              {isSceneTarget && <input className="spell-scene-target" aria-label={`${spell.name}施法位置`} placeholder="輸入施法位置／區域（例：洞穴深處的祭壇）" value={sceneTarget[spell.id] || ''} onChange={(event) => setSceneTarget((current) => ({ ...current, [spell.id]: event.target.value }))} />}
                              {canCastNormally && <button type="button" onClick={() => onCastSpell(player.id, spell, false, resolvedTarget)} disabled={!hasSlot || !selectedTarget}>施放並鎖定行動</button>}
                              {canRitual && <button type="button" className="ritual-button" disabled={!selectedTarget} onClick={() => onCastSpell(player.id, spell, true, resolvedTarget)}>儀式並鎖定</button>}
                            </div>
                          </article>
                        );
                      })}
                    </div>
                  ))}
                </div>
              </section>
            )}

            <section className="sheet-two-column sheet-bottom">
              <div className="sheet-section">
                <div className="sheet-section-title"><span>職業能力</span><small>FEATURES</small></div>
                <div className="feature-list">{player.features.map((entry) => <div key={entry.id}><strong>{entry.name}</strong><p>{entry.description}</p></div>)}</div>
              </div>
              <div className="sheet-section">
                <div className="sheet-section-title"><span>裝備</span><small>EQUIPMENT</small></div>
                <ul className="equipment-list">{player.equipment.map((entry) => <li key={entry}>{entry}</li>)}</ul>
              </div>
            </section>

            <footer className="sheet-rests">
              <span>生命骰 d{player.hitDie}：{player.hitDice}／{player.maxHitDice}<small>{combatActive ? '戰鬥進行中，休息暫時不可使用。' : '只能在沒有待裁定行動時休息。短休消耗 1 點探索行動時間；長休消耗 4 點。'}</small></span>
              <div><button type="button" disabled={combatActive} onClick={() => onRest(player.id, 'short')}><Timer />短休／1 點</button><button type="button" disabled={combatActive} onClick={() => onRest(player.id, 'long')}><Bed />長休／4 點</button></div>
            </footer>
          </motion.article>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
