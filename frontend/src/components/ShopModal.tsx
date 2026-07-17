import { useEffect, useMemo, useState } from 'react';
import { motion } from 'framer-motion';
import { Coins, Flask, Package, Shield, Storefront, Sword, X } from '@phosphor-icons/react';
import type { PlayerCharacter, PlayerId } from '../types';
import { shopCatalog, type ShopItem } from '../api';

interface ShopModalProps {
  players: PlayerCharacter[];
  busy: boolean;
  onClose: () => void;
  onBuy: (playerId: PlayerId, itemId: string) => void;
  onSell: (playerId: PlayerId, itemName: string) => void;
}

const kindIcon = {
  weapon: <Sword size={14} />,
  armor: <Shield size={14} />,
  potion: <Flask size={14} />,
  gear: <Package size={14} />,
} as const;

const kindLabel = { weapon: '武器', armor: '護甲', potion: '藥劑', gear: '雜項' } as const;

// Equipment merchant: buy from the fixed catalog, sell carried items back.
// Available out of combat; the DM narrates who the merchant is in the story.
export function ShopModal({ players, busy, onClose, onBuy, onSell }: ShopModalProps) {
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
        aria-label="裝備商店"
        initial={{ opacity: 0, y: 24, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ type: 'spring', stiffness: 280, damping: 26 }}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="shop-head">
          <Storefront size={22} weight="duotone" />
          <div>
            <p className="eyebrow">Equipment merchant</p>
            <h2>裝備商店</h2>
          </div>
          <button type="button" className="shop-close" onClick={onClose} aria-label="關閉商店"><X size={18} /></button>
        </header>

        <div className="shop-players" role="tablist" aria-label="選擇買家">
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
          <section aria-label="商店目錄">
            <h3>商店目錄</h3>
            <div className="shop-list">
              {items.map((item) => {
                const affordable = (active.gold ?? 0) >= item.price;
                return (
                  <article key={item.id} className="shop-item">
                    <span className={`shop-kind shop-kind-${item.kind}`}>{kindIcon[item.kind]}{kindLabel[item.kind]}</span>
                    <strong>{item.name}</strong>
                    <small>{item.note}</small>
                    <button type="button" disabled={busy || !affordable} onClick={() => onBuy(active.id, item.id)}>
                      {affordable ? `購買 ${item.price} gp` : `需 ${item.price} gp`}
                    </button>
                  </article>
                );
              })}
            </div>
          </section>
          <section aria-label="身上裝備">
            <h3>{active.name}的行囊</h3>
            {active.equipment.length === 0 && <p className="shop-empty">行囊空空如也。</p>}
            <div className="shop-list shop-inventory">
              {active.equipment.map((name, index) => (
                <article key={`${name}-${index}`} className="shop-item">
                  <strong>{name}</strong>
                  <button type="button" disabled={busy} onClick={() => onSell(active.id, name)}>
                    賣出 +{sellPriceOf(name)} gp
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
