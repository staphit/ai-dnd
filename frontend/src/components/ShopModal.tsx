import { useEffect, useMemo, useState } from 'react';
import { motion } from 'framer-motion';
import { Coins, Flask, Hammer, Package, Shield, Storefront, Sword, X } from '@phosphor-icons/react';
import type { PlayerCharacter, PlayerId } from '../types';
import { shopCatalog, type ShopItem } from '../api';
import { useI18n } from '../i18n';

const FORGE_CAP = 3;
const forgeWeaponCost = (nextLevel: number) => nextLevel * 100;
const forgeArmorCost = (nextLevel: number) => nextLevel * 150;

interface ShopModalProps {
  players: PlayerCharacter[];
  busy: boolean;
  onClose: () => void;
  onBuy: (playerId: PlayerId, itemId: string) => void;
  onSell: (playerId: PlayerId, itemName: string) => void;
  onForge: (playerId: PlayerId, kind: 'weapon' | 'armor', attackId?: string) => void;
}

const kindIcon = {
  weapon: <Sword size={14} />,
  armor: <Shield size={14} />,
  potion: <Flask size={14} />,
  gear: <Package size={14} />,
} as const;

const kindLabel = { weapon: '武器', armor: '護甲', potion: '藥劑', gear: '雜項' } as const;
const kindLabelEn = { weapon: 'Weapon', armor: 'Armor', potion: 'Potion', gear: 'Gear' } as const;

// Equipment merchant: buy from the fixed catalog, sell carried items back.
// Available out of combat; the DM narrates who the merchant is in the story.
export function ShopModal({ players, busy, onClose, onBuy, onSell, onForge }: ShopModalProps) {
  const { lang, tz } = useI18n();
  const [items, setItems] = useState<ShopItem[]>([]);
  const [loadError, setLoadError] = useState('');
  const [activeId, setActiveId] = useState<PlayerId>(players[0]?.id as PlayerId);

  useEffect(() => {
    let cancelled = false;
    shopCatalog()
      .then(({ items: list }) => { if (!cancelled) setItems(list); })
      .catch((caught) => { if (!cancelled) setLoadError(caught instanceof Error ? caught.message : String(caught)); });
    return () => { cancelled = true; };
  }, []);

  const active = useMemo(() => players.find((player) => player.id === activeId) || players[0], [players, activeId]);
  const sellPriceOf = (name: string) => {
    const item = items.find((entry) => entry.name === name);
    return item ? Math.max(1, Math.floor(item.price / 2)) : 5;
  };

  if (!active) return null;

  return (
    <motion.div className="shop-backdrop" role="presentation" initial={{ opacity: 0 }} animate={{ opacity: 1 }} onMouseDown={onClose}>
      <motion.div
        className="shop-modal"
        role="dialog"
        aria-modal="true"
        aria-label={tz('裝備商店')}
        initial={{ opacity: 0, y: 24, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ type: 'spring', stiffness: 280, damping: 26 }}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="shop-head">
          <Storefront size={22} weight="duotone" />
          <div>
            <p className="eyebrow">Equipment merchant</p>
            <h2>{tz('裝備商店')}</h2>
          </div>
          <button type="button" className="shop-close" onClick={onClose} aria-label={tz('關閉商店')}><X size={18} /></button>
        </header>

        <div className="shop-players" role="tablist" aria-label={tz('選擇買家')}>
          {players.map((player) => (
            <button
              key={player.id}
              type="button"
              role="tab"
              aria-selected={player.id === active.id}
              className={player.id === active.id ? 'active' : ''}
              onClick={() => setActiveId(player.id)}
            >
              {player.name}
              <span className="shop-gold"><Coins size={13} weight="fill" />{player.gold ?? 0} gp</span>
            </button>
          ))}
        </div>

        {loadError && <p className="shop-error" role="alert">{loadError}</p>}

        <div className="shop-columns">
          <section aria-label={tz('商店目錄')}>
            <h3>{tz('商店目錄')}</h3>
            <div className="shop-list">
              {items.map((item) => {
                const affordable = (active.gold ?? 0) >= item.price;
                return (
                  <article key={item.id} className="shop-item">
                    <span className={`shop-kind shop-kind-${item.kind}`}>{kindIcon[item.kind]}{lang === 'en' ? kindLabelEn[item.kind] : kindLabel[item.kind]}</span>
                    <strong>{item.name}</strong>
                    <small>{item.note}</small>
                    <button type="button" disabled={busy || !affordable} onClick={() => onBuy(active.id, item.id)}>
                      {affordable ? `${tz('購買')} ${item.price} gp` : `${tz('需')} ${item.price} gp`}
                    </button>
                  </article>
                );
              })}
            </div>
          </section>
          <section aria-label={tz('鍛造商')}>
            <h3><Hammer size={14} /> {lang === 'en' ? `Forge (upgrade cap +${FORGE_CAP})` : `鍛造商（強化上限 +${FORGE_CAP}）`}</h3>
            <div className="shop-list">
              {active.attacks.map((attack) => {
                const level = attack.upgradeLevel || 0;
                const maxed = level >= FORGE_CAP;
                const cost = forgeWeaponCost(level + 1);
                const affordable = (active.gold ?? 0) >= cost;
                return (
                  <article key={attack.id} className="shop-item">
                    <span className="shop-kind shop-kind-weapon"><Sword size={14} />{tz('武器')}</span>
                    <strong>{attack.name}{level > 0 ? ` +${level}` : ''}</strong>
                    <small>{tz('命中')} +{attack.attackBonus}・{attack.damage}{(attack.attacksPerAction || 1) > 1 ? (lang === 'en' ? `・${attack.attacksPerAction} attacks per action` : `・每動作 ${attack.attacksPerAction} 擊`) : ''}</small>
                    <button type="button" disabled={busy || maxed || !affordable} onClick={() => onForge(active.id, 'weapon', attack.id)}>
                      {maxed ? tz('已達上限') : affordable ? `${tz('強化')} ${cost} gp` : `${tz('需')} ${cost} gp`}
                    </button>
                  </article>
                );
              })}
              {(() => {
                const level = active.armorUpgrade || 0;
                const maxed = level >= FORGE_CAP;
                const cost = forgeArmorCost(level + 1);
                const affordable = (active.gold ?? 0) >= cost;
                return (
                  <article className="shop-item">
                    <span className="shop-kind shop-kind-armor"><Shield size={14} />{tz('護甲')}</span>
                    <strong>{tz('護甲')}{level > 0 ? ` +${level}` : ''}</strong>
                    <small>{lang === 'en' ? `Current AC ${active.ac}; each upgrade adds +1 AC` : `目前 AC ${active.ac}，每級強化 +1 AC`}</small>
                    <button type="button" disabled={busy || maxed || !affordable} onClick={() => onForge(active.id, 'armor')}>
                      {maxed ? tz('已達上限') : affordable ? `${tz('強化')} ${cost} gp` : `${tz('需')} ${cost} gp`}
                    </button>
                  </article>
                );
              })()}
            </div>
          </section>
          <section aria-label={tz('身上裝備')}>
            <h3>{lang === 'en' ? `${active.name} inventory` : `${active.name}的行囊`}</h3>
            {active.equipment.length === 0 && <p className="shop-empty">{tz('行囊空空如也。')}</p>}
            <div className="shop-list shop-inventory">
              {active.equipment.map((name, index) => (
                <article key={`${name}-${index}`} className="shop-item">
                  <strong>{name}</strong>
                  <button type="button" disabled={busy} onClick={() => onSell(active.id, name)}>
                    {tz('賣出')} +{sellPriceOf(name)} gp
                  </button>
                </article>
              ))}
            </div>
          </section>
        </div>
      </motion.div>
    </motion.div>
  );
}
