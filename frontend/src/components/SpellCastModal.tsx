import { useEffect, useId, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Crosshair, MagicWand, Sparkle, Timer, X } from '@phosphor-icons/react';
import type { CharacterSpell, PlayerCharacter } from '../types';
import { abilityLabels } from '../labels';

export type SpellTargetOption = { id: string; name: string; side: 'party' | 'enemy' };

export function spellTargetOptions(
  player: PlayerCharacter,
  spell: CharacterSpell,
  spellTargets: SpellTargetOption[],
): SpellTargetOption[] {
  if (spell.effect?.target === 'self' || /自身/.test(spell.range)) {
    return spellTargets.filter((entry) => entry.id === player.id);
  }
  if (spell.effect?.target === 'ally') {
    return spellTargets.filter((entry) => entry.side === 'party');
  }
  if (spell.effect?.target === 'creature') {
    return spellTargets.filter((entry) => entry.side === 'enemy');
  }
  return [...spellTargets, { id: 'scene', name: '目前場景／指定位置', side: 'party' }];
}

export function defaultSpellTargetId(player: PlayerCharacter, spell: CharacterSpell): string | undefined {
  if (spell.effect?.target === 'self' || /自身/.test(spell.range)) return player.id;
  return undefined;
}

export function spellCastMeta(spell: CharacterSpell, player: PlayerCharacter) {
  const canCast = spell.level === 0 || spell.prepared || spell.alwaysPrepared;
  const canRitual = spell.ritual && (spell.prepared || spell.inSpellbook);
  const hasFreeUse = Boolean(spell.freeUseResourceId && player.resources.some((entry) => entry.id === spell.freeUseResourceId && entry.current > 0));
  const freeResource = spell.freeUseResourceId
    ? player.resources.find((entry) => entry.id === spell.freeUseResourceId)
    : undefined;
  const hasSlot = spell.level === 0 || hasFreeUse || Boolean(player.spellcasting?.slots.some((slot) => slot.level >= spell.level && slot.current > 0));
  const rollRule = spell.effect?.attackRoll
    ? '施放後擲法術攻擊 d20'
    : spell.effect?.saveAbility
      ? `目標進行${abilityLabels[spell.effect.saveAbility]}豁免`
      : spell.effect?.automaticHit
        ? '自動命中'
        : null;
  const costLabel = spell.level === 0
    ? '戲法・不消耗法術位'
    : hasFreeUse
      ? `可消耗「${freeResource?.name || '免費施法'}」`
      : `消耗 ${spell.level} 環以上法術位`;
  return { canCast, canRitual, hasFreeUse, hasSlot, rollRule, costLabel, freeResource };
}

interface SpellCastModalProps {
  open: boolean;
  player: PlayerCharacter;
  spell: CharacterSpell | null;
  spellTargets: SpellTargetOption[];
  onClose: () => void;
  onCast: (spell: CharacterSpell, asRitual: boolean, targetId: string) => void;
}

export function SpellCastModal({ open, player, spell, spellTargets, onClose, onCast }: SpellCastModalProps) {
  const titleId = useId();
  const targets = useMemo(
    () => (spell ? spellTargetOptions(player, spell, spellTargets) : []),
    [player, spell, spellTargets],
  );
  const [targetId, setTargetId] = useState('');
  const [sceneText, setSceneText] = useState('');

  useEffect(() => {
    if (!spell || !open) return;
    setTargetId(defaultSpellTargetId(player, spell) || '');
    setSceneText('');
  }, [spell, open, player]);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!spell) return null;
  const meta = spellCastMeta(spell, player);
  const isScene = targetId === 'scene';
  const resolvedTarget = isScene ? (sceneText.trim() || 'scene') : targetId;
  const canConfirm = Boolean(meta.canCast && meta.hasSlot && targetId && (!isScene || sceneText.trim()));
  const canRitualConfirm = Boolean(meta.canRitual && targetId && (!isScene || sceneText.trim()));
  const ability = player.spellcasting ? abilityLabels[player.spellcasting.ability] : '—';
  const attack = player.spellcasting ? (player.spellcasting.attackBonus >= 0 ? `+${player.spellcasting.attackBonus}` : String(player.spellcasting.attackBonus)) : '—';
  const saveDc = player.spellcasting?.saveDc ?? '—';

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="spell-cast-backdrop"
          role="presentation"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onMouseDown={(event) => {
            event.stopPropagation();
            onClose();
          }}
        >
          <motion.div
            className="spell-cast-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby={titleId}
            initial={{ opacity: 0, y: 28, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 16, scale: 0.98 }}
            transition={{ type: 'spring', stiffness: 280, damping: 26 }}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <div className="spell-cast-glow" aria-hidden="true" />
            <header className="spell-cast-head">
              <div className="spell-cast-sigil" aria-hidden="true"><MagicWand size={22} weight="duotone" /></div>
              <div className="spell-cast-titles">
                <p className="eyebrow">Cast spell／施法</p>
                <h2 id={titleId}>{spell.name}</h2>
                <span>{spell.englishName}・{spell.school}</span>
              </div>
              <button type="button" className="spell-cast-close" onClick={onClose} aria-label="關閉施法視窗"><X size={18} /></button>
            </header>

            <div className="spell-cast-badges">
              <span className="spell-cast-level">{spell.level === 0 ? '戲法' : `${spell.level} 環`}</span>
              <span><Timer size={12} />{spell.castingTime}</span>
              <span><Crosshair size={12} />{spell.range}</span>
              {spell.concentration && <span className="spell-cast-flag"><Sparkle size={12} />專注</span>}
              {spell.ritual && <span className="spell-cast-flag">儀式</span>}
              {spell.alwaysPrepared && <span className="spell-cast-flag">常備</span>}
              {!meta.canCast && <span className="spell-cast-warn">未準備</span>}
            </div>

            <p className="spell-cast-desc">{spell.description}</p>
            {meta.rollRule && <p className="spell-cast-roll"><strong>結算：</strong>{meta.rollRule}</p>}

            <div className="spell-cast-stats">
              <div><small>施法屬性</small><strong>{ability}</strong></div>
              <div><small>法術攻擊</small><strong>{attack}</strong></div>
              <div><small>豁免 DC</small><strong>{saveDc}</strong></div>
              <div><small>消耗</small><strong>{meta.costLabel}</strong></div>
            </div>

            <div className="spell-cast-form">
              <label>
                <span>目標</span>
                <select
                  aria-label={`${spell.name}目標`}
                  value={targetId}
                  onChange={(event) => setTargetId(event.target.value)}
                >
                  <option value="" disabled>選擇施法目標…</option>
                  {targets.map((target) => (
                    <option key={target.id} value={target.id}>
                      {target.name}{target.side === 'enemy' ? '（敵）' : target.id === player.id ? '（自己）' : target.id === 'scene' ? '' : '（友）'}
                    </option>
                  ))}
                </select>
              </label>
              {isScene && (
                <label>
                  <span>施法位置／區域</span>
                  <input
                    className="spell-scene-target"
                    aria-label={`${spell.name}施法位置`}
                    placeholder="例：洞穴深處的祭壇、門廊中央"
                    value={sceneText}
                    onChange={(event) => setSceneText(event.target.value)}
                    autoFocus
                  />
                </label>
              )}
            </div>

            {!meta.hasSlot && meta.canCast && (
              <p className="spell-cast-hint spell-cast-hint-warn" role="status">目前沒有可用的法術位或免費施法次數。</p>
            )}
            {!targetId && meta.canCast && meta.hasSlot && (
              <p className="spell-cast-hint" role="status">請先指定目標，才能施放並鎖定本回合行動。</p>
            )}

            <footer className="spell-cast-actions">
              <button type="button" className="spell-cast-cancel" onClick={onClose}>取消</button>
              {meta.canRitual && (
                <button
                  type="button"
                  className="spell-cast-ritual"
                  disabled={!canRitualConfirm}
                  onClick={() => onCast(spell, true, resolvedTarget)}
                >
                  儀式施放
                </button>
              )}
              {meta.canCast && (
                <button
                  type="button"
                  className="spell-cast-confirm"
                  disabled={!canConfirm}
                  onClick={() => onCast(spell, false, resolvedTarget)}
                >
                  <MagicWand size={16} weight="fill" />
                  施放並鎖定行動
                </button>
              )}
            </footer>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
