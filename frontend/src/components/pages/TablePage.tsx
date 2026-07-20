import { motion } from 'framer-motion';
import {
  ArrowClockwise,
  Compass,
  Heartbeat,
  Lightbulb,
  LockKey,
  MapTrifold,
  Plugs,
  ShieldWarning,
  Storefront,
  Sword,
  XCircle,
} from '@phosphor-icons/react';
import type { ContextualTip } from '../../app/app-utils';
import type { SceneSlotInfo } from '../../api';
import type { DiceAnimationState } from '../../hooks/useDiceAnimation';
import type {
  AiStatus,
  Campaign,
  CampaignSettings,
  CharacterSpell,
  MessageAudience,
  Page,
  PlayerCharacter,
  PlayerId,
  RequiredCheck,
  RestType,
  SceneImage,
  StoryEntry,
} from '../../types';
import type { RevisionChatLine } from '../StoryRevisionPanel';
import type { StageClearInfo } from '../StageClearModal';
import { CharacterPanel } from '../CharacterPanel';
import { CombatTracker } from '../CombatTracker';
import { DiceTray } from '../DiceTray';
import { MagneticButton } from '../MagneticButton';
import { PartyWipeModal } from '../PartyWipeModal';
import { SceneVisual } from '../SceneVisual';
import { ShopModal } from '../ShopModal';
import { SpellCastModal } from '../SpellCastModal';
import { StageClearModal } from '../StageClearModal';
import { StoryFeed } from '../StoryFeed';
import { StoryRevisionPanel } from '../StoryRevisionPanel';

interface SpellModalState {
  playerId: PlayerId;
  spell: CharacterSpell;
}

interface TablePageProps {
  campaign: Campaign;
  settings: CampaignSettings;
  status: AiStatus | null;
  viewer: MessageAudience;
  sceneImage: SceneImage | null;
  sceneSlots: SceneSlotInfo[];
  generatingSlotId: string;
  imageLoading: boolean;
  imageError: string;
  canGenerateImages: boolean;
  contextualTip?: ContextualTip;
  notice: string;
  error: string;
  demoMode: boolean;
  needsDmConnect: boolean;
  canConnectDm: boolean;
  dmLabel: string;
  connecting: boolean;
  loading: boolean;
  retryAvailable: boolean;
  activeRequiredCheck: RequiredCheck | null;
  currentCombatantName?: string;
  diceAnimation: DiceAnimationState;
  revisionOpen: boolean;
  revising: boolean;
  latestDm?: StoryEntry;
  revisionChat: RevisionChatLine[];
  partyWiped: boolean;
  stageClear: StageClearInfo | null;
  shopOpen: boolean;
  shopBusy: boolean;
  spellModal: SpellModalState | null;
  onPageChange: (page: Page) => void;
  onViewerChange: (viewer: MessageAudience) => void;
  onSelectSceneImage: (image: SceneImage) => void;
  onGenerateImage: (narration?: string, imagePrompt?: string, scene?: string, slotId?: string) => void;
  onDismissTip: (id: string) => void;
  onDismissNotice: () => void;
  onEnableDemo: () => void;
  onConnectDm: () => void;
  onDismissError: () => void;
  onRetryLastTurn: () => void;
  onRevive: (targetId: PlayerId, rescuerId: PlayerId) => void;
  onOpenShop: () => void;
  onToggleRevision: () => void;
  onCloseRevision: () => void;
  onSubmitRevision: (note: string) => void;
  onDiceRoll: (success: boolean, total: number) => void;
  onRequiredRoll: (natural: number, success: boolean) => void;
  onCampaign: (view: Campaign) => void;
  onEndCombat: (final?: boolean) => void;
  onOpenSpellCast: (playerId: PlayerId, spell: CharacterSpell, asRitual?: boolean, targetId?: string) => void;
  onResourceChange: (playerId: PlayerId, resourceId: string, delta: number) => void;
  onUseItem: (playerId: PlayerId, itemName: string) => void;
  onSubmitAction: (playerId: PlayerId, text: string) => void;
  onUnlockAction: (playerId: PlayerId) => void;
  onRest: (playerId: PlayerId, type: RestType) => void;
  onGeneratePortrait: (player: PlayerCharacter, appearance: string) => Promise<void>;
  onRetryCombat: () => void;
  onClearStage: () => void;
  onCloseShop: () => void;
  onShopBuy: (playerId: PlayerId, itemId: string) => void;
  onShopSell: (playerId: PlayerId, itemName: string) => void;
  onShopForge: (playerId: PlayerId, kind: 'weapon' | 'armor', attackId?: string) => void;
  onCloseSpellModal: () => void;
  onCastSpell: (playerId: PlayerId, spell: CharacterSpell, asRitual: boolean, targetId: string) => void;
}

export function TablePage(props: TablePageProps) {
  const {
    campaign,
    settings,
    viewer,
    sceneImage,
    sceneSlots,
    generatingSlotId,
    imageLoading,
    imageError,
    canGenerateImages,
    activeRequiredCheck,
    diceAnimation,
  } = props;
  return (
    <motion.main key="table" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="table-layout">
      <div className="table-main">
        <div className="table-preamble">
          <SceneVisual
            image={sceneImage}
            images={settings.sceneImages || []}
            slots={sceneSlots}
            generatingSlotId={generatingSlotId}
            scene={campaign.scene}
            loading={imageLoading}
            error={imageError}
            canGenerate={canGenerateImages}
            onGenerate={() => props.onGenerateImage()}
            onSelect={props.onSelectSceneImage}
            onGenerateSlot={(slotId) => props.onGenerateImage(undefined, undefined, undefined, slotId)}
          />
        </div>

        <div className="viewer-switch">
          <LockKey />
          <span>訊息視角</span>
          <select value={viewer} onChange={(event) => props.onViewerChange(event.target.value as MessageAudience)}>
            <option value="public">公開訊息</option>
            {campaign.players.map((player) => <option key={player.id} value={player.id}>{player.name} 的私密訊息</option>)}
          </select>
        </div>

        <TableContext {...props} />

        <div className={`story-workspace ${props.revisionOpen ? 'story-workspace-revision' : ''}`}>
          <div className="story-workspace-main">
            <StoryFeed
              story={campaign.story}
              players={campaign.players}
              loading={props.loading}
              viewer={viewer}
              combatActive={campaign.combat?.active === true}
              checkPending={Boolean(activeRequiredCheck)}
              diceRolling={diceAnimation.rolling}
              diceOutcome={diceAnimation.outcome}
            />
            <div className="story-revision-toggle-row">
              <button type="button" className={`story-revision-toggle ${props.revisionOpen ? 'active' : ''}`} disabled={!props.latestDm || props.loading || props.revising} onClick={props.onToggleRevision}>
                {props.revisionOpen ? '關閉敘事修正' : '修正上一則 DM 敘事'}
              </button>
            </div>
          </div>
          <StoryRevisionPanel
            open={props.revisionOpen}
            onClose={props.onCloseRevision}
            loading={props.revising}
            disabled={props.loading || props.needsDmConnect || !props.latestDm}
            disabledReason={props.needsDmConnect ? `請先連線 ${props.dmLabel}` : !props.latestDm ? '尚無公開 DM 敘事' : undefined}
            previousDraft={props.latestDm?.text || ''}
            chat={props.revisionChat}
            onSubmit={props.onSubmitRevision}
          />
        </div>

        {activeRequiredCheck && (
          <DiceTray
            players={campaign.players}
            requiredCheck={activeRequiredCheck}
            onRoll={(roll) => props.onDiceRoll(roll.success, roll.total)}
            onRequiredRoll={(roll) => props.onRequiredRoll(roll.natural, roll.success)}
          />
        )}
        {campaign.combat?.active && campaign.id && (
          <section className="inline-combat">
            <div className="section-heading"><div><p className="eyebrow">戰鬥進行中</p><h2>先攻與回合操作</h2></div></div>
            <CombatTracker
              campaignId={campaign.id}
              players={campaign.players}
              combat={campaign.combat}
              onView={props.onCampaign}
              onEnd={() => props.onEndCombat()}
              onCastSpell={(playerId, spell) => props.onOpenSpellCast(playerId, spell)}
              onUseResource={(playerId, resourceId) => props.onResourceChange(playerId, resourceId, -1)}
              onUseItem={props.onUseItem}
            />
          </section>
        )}
        <PlayerComposers {...props} />
        <TableOverlays {...props} />
      </div>
    </motion.main>
  );
}

function TableContext(props: TablePageProps) {
  const { campaign, contextualTip } = props;
  return (
    <div className="table-preamble table-context">
      {contextualTip && (
        <aside className="novice-tip novice-tip-inline" aria-label="新手提示">
          <Lightbulb size={20} />
          <div><strong>{contextualTip.title}</strong><span>{contextualTip.text}</span></div>
          {contextualTip.page && <button type="button" className="tip-action" onClick={() => props.onPageChange(contextualTip.page!)}>前往查看</button>}
          <button type="button" className="tip-dismiss" aria-label="關閉提示" onClick={() => props.onDismissTip(contextualTip.id)}><XCircle /></button>
        </aside>
      )}
      {props.notice && <div className="notice-banner"><span>{props.notice}</span><button type="button" onClick={props.onDismissNotice}><XCircle /></button></div>}
      {props.status && !props.status.connected && !props.demoMode && (
        <div className="model-notice">
          <ShieldWarning size={20} />
          <div><strong>本機 AI 尚未連線</strong><span>請先執行 codex login，或使用示範 DM。</span></div>
          <MagneticButton variant="quiet" onClick={props.onEnableDemo}>使用示範 DM</MagneticButton>
        </div>
      )}
      {props.needsDmConnect && props.canConnectDm && (
        <div className="model-notice">
          <Plugs size={20} />
          <div><strong>需要連線 {props.dmLabel}</strong><span>每個故事各自一條 DM 連線；「{campaign.title}」要先連線後 DM 才會裁定。切換故事、切換資料源或連線中斷後都需要重新連線。</span></div>
          <MagneticButton onClick={props.onConnectDm} disabled={props.connecting}>{props.connecting ? '連線中…' : `連線 ${props.dmLabel}`}</MagneticButton>
        </div>
      )}
      {props.error && (
        <div className="error-banner" role="alert">
          <XCircle size={19} />
          <div><strong>操作中斷</strong><span>{props.error}</span></div>
          {props.retryAvailable && !props.needsDmConnect && !props.loading && <button type="button" className="retry-turn" onClick={props.onRetryLastTurn}><ArrowClockwise />重試上一步</button>}
          <button type="button" onClick={props.onDismissError}><XCircle /></button>
        </div>
      )}
      <ReviveNotice {...props} />
      <SceneStrip {...props} />
      <ObjectiveCard campaign={campaign} />
    </div>
  );
}

function ReviveNotice(props: TablePageProps) {
  const { campaign } = props;
  if (campaign.combat?.active || !campaign.id || !campaign.players.some((player) => player.hp === 0)) return null;
  return (
    <div className="model-notice revive-notice">
      <Heartbeat size={20} />
      <div><strong>有隊友倒地</strong><span>救援會消耗 1 點探索行動時間；倒地者最多花費 2 顆生命骰回復生命後重新站起。</span></div>
      {campaign.players.filter((player) => player.hp === 0).map((target) => {
        const rescuer = campaign.players.find((player) => player.hp > 0 && player.id !== target.id);
        return rescuer ? <MagneticButton key={target.id} variant="quiet" disabled={props.loading} onClick={() => props.onRevive(target.id, rescuer.id)}>{rescuer.name}救援{target.name}</MagneticButton> : null;
      })}
    </div>
  );
}

function SceneStrip(props: TablePageProps) {
  const { campaign } = props;
  return (
    <section className="scene-strip" aria-label="場景與遊戲狀態">
      <MapTrifold size={21} />
      <div><span>目前場景</span><strong>{campaign.scene}</strong></div>
      <div className={`game-state ${campaign.combat?.active ? 'game-state-combat' : 'game-state-exploration'}`}>
        {campaign.combat?.active ? <Sword size={17} weight="fill" /> : <Compass size={17} />}
        <div><span>遊戲狀態</span><strong>{campaign.combat?.active ? `戰鬥中${props.currentCombatantName ? `・${props.currentCombatantName} 行動` : ''}` : '探索中'}</strong></div>
      </div>
      <div className="round-mark"><span>{campaign.combat?.active ? '戰鬥輪' : '探索回合'}</span><strong>{String(campaign.combat?.active ? campaign.combat.round : campaign.round).padStart(2, '0')}</strong></div>
      <button type="button" className="shop-open" disabled={Boolean(campaign.combat?.active)} title={campaign.combat?.active ? '戰鬥中無法交易' : '向裝備商買賣裝備'} onClick={props.onOpenShop}><Storefront size={16} />裝備商店</button>
    </section>
  );
}

function ObjectiveCard({ campaign }: { campaign: Campaign }) {
  const arc = campaign.storyArc;
  const phase = arc && !arc.ended ? arc.phases[arc.current] : undefined;
  const pct = phase ? Math.min(100, Math.round((campaign.round / Math.max(1, phase.deadlineRound)) * 100)) : 0;
  return (
    <section className="objective objective-preamble" aria-label="任務摘要">
      <p className="eyebrow">任務摘要</p>
      {arc?.ended && <div className="arc-progress arc-ended">劇本三階段已完成，故事進入尾聲</div>}
      {phase && (
        <div className={`arc-progress ${campaign.round > phase.deadlineRound ? 'arc-overdue' : ''}`} aria-label="劇本進度">
          <span className="arc-stage">{phase.stage}</span><div className="arc-track" role="presentation"><i style={{ width: `${pct}%` }} /></div><span className="arc-rounds">第 {campaign.round}／{phase.deadlineRound} 回合</span><span className="arc-reward">限時獎勵 {phase.rewardXp} XP</span>
        </div>
      )}
      <strong>{campaign.objective}</strong>
      <p className="objective-context">{campaign.objectiveContext}</p>
      <div className="objective-stakes"><span>風險</span><p>{campaign.stakes}</p></div>
      <small>{campaign.combat?.active ? `戰鬥第 ${campaign.combat.round} 輪（戰鬥操作在對話下方）` : '探索進行中'}</small>
    </section>
  );
}

function PlayerComposers(props: TablePageProps) {
  const { campaign, settings } = props;
  const targets = [
    ...campaign.players.map((entry) => ({ id: entry.id, name: entry.name, side: 'party' as const })),
    ...(campaign.combat?.active ? campaign.combat.combatants.filter((entry) => entry.side === 'enemy' && !entry.defeated).map((entry) => ({ id: entry.id, name: entry.name, side: 'enemy' as const })) : []),
  ];
  return (
    <div className={`composer-grid party-${campaign.players.length}`}>
      {campaign.players.map((player) => (
        <section className="player-console" key={player.id} aria-label={`${player.name}玩家操作區`}>
          <CharacterPanel
            player={player}
            xp={campaign.xpProgress?.[player.id]}
            showStatHints={settings.showStatHints !== false}
            combatActive={campaign.combat?.active === true}
            pending={campaign.pending[player.id]}
            actionDisabled={props.loading || Boolean(props.activeRequiredCheck)}
            partySize={campaign.players.length}
            choices={(campaign.choices || []).filter((choice) => !choice.playerId || choice.playerId === player.id)}
            scripted={Boolean(campaign.script)}
            resourceSummary={player.spellcasting ? player.spellcasting.slots.map((slot) => `${slot.level}環 ${slot.current}/${slot.max}`).join('、') : player.resources.slice(0, 3).map((resource) => `${resource.name} ${resource.current}/${resource.max}`).join('、')}
            onSubmitAction={props.onSubmitAction}
            onUnlockAction={props.onUnlockAction}
            spellTargets={targets}
            onResourceChange={props.onResourceChange}
            onCastSpell={props.onOpenSpellCast}
            onRest={props.onRest}
            onGeneratePortrait={props.onGeneratePortrait}
          />
        </section>
      ))}
    </div>
  );
}

function TableOverlays(props: TablePageProps) {
  const { campaign, spellModal } = props;
  const caster = spellModal ? campaign.players.find((player) => player.id === spellModal.playerId) : undefined;
  const targets = [
    ...campaign.players.map((entry) => ({ id: entry.id, name: entry.name, side: 'party' as const })),
    ...(campaign.combat?.active ? campaign.combat.combatants.filter((entry) => entry.side === 'enemy' && !entry.defeated).map((entry) => ({ id: entry.id, name: entry.name, side: 'enemy' as const })) : []),
  ];
  return (
    <>
      {props.partyWiped && campaign.id && <PartyWipeModal busy={props.loading} onRetry={props.onRetryCombat} onEndStory={() => props.onEndCombat(true)} />}
      {props.stageClear && <StageClearModal info={props.stageClear} onClose={props.onClearStage} />}
      {props.shopOpen && campaign.id && <ShopModal players={campaign.players} busy={props.shopBusy} onClose={props.onCloseShop} onBuy={props.onShopBuy} onSell={props.onShopSell} onForge={props.onShopForge} />}
      {spellModal && caster && (
        <SpellCastModal open player={caster} spell={spellModal.spell} spellTargets={targets} onClose={props.onCloseSpellModal} onCast={(spell, asRitual, targetId) => props.onCastSpell(spellModal.playerId, spell, asRitual, targetId)} />
      )}
    </>
  );
}
