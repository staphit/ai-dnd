import { useEffect, useMemo, useRef, useState } from 'react';
import { ArrowClockwise, Crosshair, Lightning, MagicWand, Plus, Shield, Skull, Sword, X } from '@phosphor-icons/react';
import type { Campaign, CharacterSpell, CombatState, PlayerCharacter, PlayerId } from '../types';
import { combatAttack, combatEndTurn, combatEnemyTurn, combatStart, revive, type EnemySpec } from '../api';

interface CombatTrackerProps {
  campaignId: string;
  players: PlayerCharacter[];
  combat?: CombatState;
  // Every combat endpoint returns the full server view; the parent adopts it.
  onView: (view: Campaign) => void;
  // 結束戰鬥並敘述: the parent runs conclude + the DM narration turn.
  onEnd: () => void;
  /** Open spell-cast modal for a party member (combat or exploration). */
  onCastSpell?: (playerId: PlayerId, spell: CharacterSpell) => void;
  /** Spend one use of a class resource (回氣、動作如潮…) from the combat menu. */
  onUseResource?: (playerId: PlayerId, resourceId: string) => void;
}

const emptyEnemy: EnemySpec = { name: '骸骨守衛', ac: 13, hp: 13, initiativeBonus: 2, attackBonus: 4, damage: '1d6+2', damageType: '穿刺' };

export function CombatTracker({ campaignId, players, combat, onView, onEnd, onCastSpell, onUseResource }: CombatTrackerProps) {
  const [enemies, setEnemies] = useState<EnemySpec[]>([]);
  const [draft, setDraft] = useState(emptyEnemy);
  const [targetId, setTargetId] = useState('');
  const [attackId, setAttackId] = useState('');
  const [spellId, setSpellId] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [enemyIntent, setEnemyIntent] = useState('');
  const busyRef = useRef(false);
  const enemyTurnFiredRef = useRef('');

  const current = combat?.active ? combat.combatants[combat.turnIndex] : undefined;
  const currentPlayer = players.find((player) => player.id === current?.playerId);
  const availableAttacks = currentPlayer?.attacks || [];
  const castableSpells = useMemo(() => {
    const list = currentPlayer?.spellcasting?.spells || [];
    return list.filter((spell) => spell.level === 0 || spell.prepared || spell.alwaysPrepared);
  }, [currentPlayer]);
  const validTargets = useMemo(() => combat?.combatants.filter((entry) => !entry.defeated && entry.id !== current?.id && entry.side !== current?.side) || [], [combat, current]);
  // Downed party members the current player can spend their action to revive.
  const downedAllies = useMemo(
    () => (current?.side === 'party'
      ? combat?.combatants.filter((entry) => entry.side === 'party' && entry.defeated && entry.playerId && entry.playerId !== current?.playerId) || []
      : []),
    [combat, current],
  );
  const currentEconomy = current ? combat?.turnEconomy?.[current.id] || { actionUsed: false, bonusActionUsed: false, reactionUsed: false } : undefined;
  const enemyTurnKey = combat?.active && current?.side === 'enemy' && !current.defeated
    ? `${combat.round}:${combat.turnIndex}:${current.id}`
    : '';

  async function run<T>(call: () => Promise<T>, apply: (result: T) => void) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setError('');
    try {
      apply(await call());
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  function addEnemy() {
    setEnemies((list) => [...list, { ...draft, name: draft.name.trim() || '未命名敵人', hp: Math.max(1, draft.hp) }]);
  }

  function begin() {
    void run(() => combatStart(campaignId, enemies), (view) => {
      onView(view);
      setEnemies([]);
      setEnemyIntent('');
    });
  }

  function attack() {
    const target = targetId || validTargets[0]?.id || '';
    if (!target) return;
    const chosenAttack = attackId || availableAttacks[0]?.id || '';
    void run(() => combatAttack(campaignId, { attackId: chosenAttack, targetId: target }), (result) => {
      onView(result.view);
      setTargetId('');
    });
  }

  function endTurn() {
    void run(() => combatEndTurn(campaignId), (result) => onView(result.view));
  }

  function enemyTurn() {
    if (enemyTurnKey) enemyTurnFiredRef.current = enemyTurnKey;
    void run(() => combatEnemyTurn(campaignId), (result) => {
      setEnemyIntent(result.intent);
      onView(result.view);
    });
  }

  // When the initiative order lands on an undefeated enemy, trigger its AI
  // turn automatically once (after a short beat); the button stays available
  // for manual retries if the request fails.
  useEffect(() => {
    if (!enemyTurnKey || enemyTurnFiredRef.current === enemyTurnKey) return;
    const timer = window.setTimeout(() => {
      if (enemyTurnFiredRef.current !== enemyTurnKey && !busyRef.current) enemyTurn();
    }, 600);
    return () => window.clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enemyTurnKey]);

  if (!combat?.active) {
    return (
      <section className="combat-console">
        <header><div><p className="eyebrow">Encounter setup</p><h2>建立戰鬥</h2></div><Sword size={24} /></header>
        <p className="muted-copy">玩家會自動加入。新增敵人後擲先攻；每次攻擊由伺服器判斷命中、重擊、傷害並記錄至故事。</p>
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
          {enemies.map((enemy, index) => <span key={`${enemy.name}-${index}`}>{enemy.name}／AC {enemy.ac}／HP {enemy.hp}<button type="button" onClick={() => setEnemies((list) => list.filter((_, entryIndex) => entryIndex !== index))}><X /></button></span>)}
        </div>
        {error && <p className="combat-error" role="alert">{error}</p>}
        <button type="button" className="primary-action" onClick={begin} disabled={busy || enemies.length === 0}><Crosshair />擲先攻並開始</button>
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
      {currentEconomy && <div className="turn-economy"><span className={currentEconomy.actionUsed ? 'used' : ''}>動作：{currentEconomy.actionUsed ? '已使用' : '可用'}</span><span className={currentEconomy.bonusActionUsed ? 'used' : ''}>附贈動作：{currentEconomy.bonusActionUsed ? '已使用' : '可用'}</span><span className={currentEconomy.reactionUsed ? 'used' : ''}>反應：{currentEconomy.reactionUsed ? '已使用' : '可用'}</span></div>}
      {enemyIntent && <p className="enemy-intent" role="status"><Skull size={16} weight="fill" />【敵方】{enemyIntent}</p>}
      {error && <p className="combat-error" role="alert">{error}</p>}
      <div className="combat-actions">
        {current?.side === 'enemy' ? (
          <button type="button" className="primary-action" onClick={enemyTurn} disabled={busy}><Skull />{busy ? '敵方行動結算中…' : '敵方行動'}</button>
        ) : (
          <>
            {availableAttacks.length > 0 && (
              <label>
                攻擊方式
                <select value={attackId || availableAttacks[0]?.id} onChange={(event) => setAttackId(event.target.value)}>
                  {availableAttacks.map((entry) => (
                    <option key={entry.id} value={entry.id}>{entry.name}／命中 +{entry.attackBonus}／{entry.damage}</option>
                  ))}
                </select>
              </label>
            )}
            <label>
              攻擊目標
              <select value={targetId} onChange={(event) => setTargetId(event.target.value)}>
                {validTargets.map((entry) => (
                  <option key={entry.id} value={entry.id}>{entry.name}（AC {entry.ac}）</option>
                ))}
              </select>
            </label>
            <button type="button" className="primary-action" onClick={attack} disabled={busy || !validTargets.length || currentEconomy?.actionUsed}>
              <Crosshair />攻擊（使用動作）
            </button>
            {downedAllies.length > 0 && currentPlayer && (
              <button
                type="button"
                className="revive-action"
                disabled={busy || currentEconomy?.actionUsed}
                onClick={() => {
                  const target = downedAllies[0];
                  if (!target.playerId) return;
                  void run(() => revive(campaignId, target.playerId!, currentPlayer.id), (view) => onView(view));
                }}
              >
                <Shield />救援 {downedAllies[0].name}（使用動作）
              </button>
            )}
            {onUseResource && currentPlayer && currentPlayer.resources.length > 0 && (
              <div className="combat-resources" aria-label="職業資源">
                {currentPlayer.resources.map((resource) => (
                  <button
                    key={resource.id}
                    type="button"
                    disabled={busy || resource.current === 0}
                    title={resource.description || resource.name}
                    onClick={() => onUseResource(currentPlayer.id, resource.id)}
                  >
                    <Lightning size={14} weight="fill" />
                    {resource.name} {resource.current}/{resource.max}
                  </button>
                ))}
              </div>
            )}
            {onCastSpell && currentPlayer && castableSpells.length > 0 && (
              <div className="combat-spell-cast">
                <label>
                  法術
                  <select value={spellId || castableSpells[0]?.id} onChange={(event) => setSpellId(event.target.value)}>
                    {castableSpells.map((spell) => (
                      <option key={spell.id} value={spell.id}>
                        {spell.name}（{spell.level === 0 ? '戲法' : `${spell.level} 環`}）
                      </option>
                    ))}
                  </select>
                </label>
                <button
                  type="button"
                  disabled={busy || currentEconomy?.actionUsed}
                  onClick={() => {
                    const id = spellId || castableSpells[0]?.id;
                    const spell = castableSpells.find((entry) => entry.id === id);
                    if (spell && currentPlayer) onCastSpell(currentPlayer.id, spell);
                  }}
                >
                  <MagicWand />施放法術
                </button>
              </div>
            )}
          </>
        )}
        <button type="button" onClick={endTurn} disabled={busy}><ArrowClockwise />結束回合</button>
        <button type="button" onClick={onEnd} disabled={busy}><X />結束戰鬥並敘述</button>
      </div>
    </section>
  );
}
