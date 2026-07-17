import { lazy, Suspense, useEffect, useState, type ReactNode } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { ArrowsOutSimple, ClockCounterClockwise, HourglassMedium, LockKey, Scroll } from '@phosphor-icons/react';
import type { MessageAudience, PlayerCharacter, StoryEntry } from '../types';
import {
  clampDmAvatarScale,
  DM_AVATAR_SCALE_DEFAULT,
  DM_AVATAR_SCALE_MAX,
  DM_AVATAR_SCALE_MIN,
  loadStoredDmScale,
  saveStoredDmScale,
} from './dmAvatarScale';
import { formatStoryText } from '../formatStoryText';

const DMTable = lazy(() => import('./DMTable').then((m) => ({ default: m.DMTable })));

interface StoryFeedProps {
  story: StoryEntry[];
  players: PlayerCharacter[];
  loading: boolean;
  viewer?: MessageAudience;
  combatActive?: boolean;
  /** Required check is on screen (dice glow). */
  checkPending?: boolean;
  /** Player just clicked roll — play table dice tumble. */
  diceRolling?: boolean;
  /** Known d20 outcome for DM cheer / fail clips. */
  diceOutcome?: 'success' | 'fail' | null;
  /** Connection / setup notices shown above dialogue (not on the scene image). */
  dialogueNotices?: ReactNode;
}

const SPEAK_MS = 4500;

export function StoryFeed({
  story,
  players,
  loading,
  viewer = 'public',
  combatActive = false,
  checkPending = false,
  diceRolling = false,
  diceOutcome = null,
  dialogueNotices,
}: StoryFeedProps) {
  const [showHistory, setShowHistory] = useState(false);
  const [speaking, setSpeaking] = useState(false);
  const [dmScale, setDmScale] = useState(loadStoredDmScale);

  // When a new public DM line arrives (and not thinking), play a short "talk" pose.
  const latestPublicDmId = (() => {
    for (let i = story.length - 1; i >= 0; i -= 1) {
      const entry = story[i];
      if (entry.speaker === 'dm' && (!entry.audience || entry.audience === 'public')) return entry.id;
    }
    return '';
  })();

  useEffect(() => {
    if (!latestPublicDmId || loading) return;
    setSpeaking(true);
    const t = window.setTimeout(() => setSpeaking(false), SPEAK_MS);
    return () => window.clearTimeout(t);
  }, [latestPublicDmId, loading]);

  function speakerName(entry: StoryEntry) {
    if (entry.speaker === 'dm') return '地城主';
    if (entry.speaker === 'system') return '紀錄';
    return players.find((player) => player.id === entry.speaker)?.name || '冒險者';
  }

  const visibleStory = story.filter((entry) => !entry.audience || entry.audience === 'public' || entry.audience === viewer);
  let latestPublicDmIndex = -1;
  for (let index = visibleStory.length - 1; index >= 0; index -= 1) {
    const entry = visibleStory[index];
    if (entry.speaker === 'dm' && (!entry.audience || entry.audience === 'public')) {
      latestPublicDmIndex = index;
      break;
    }
  }
  const latestStory = latestPublicDmIndex >= 0 ? visibleStory.slice(latestPublicDmIndex) : visibleStory.slice(-1);
  const displayedStory = showHistory ? visibleStory : latestStory;
  const hiddenCount = Math.max(0, visibleStory.length - latestStory.length);

  return (
    <section className="story-panel" aria-label="冒險敘事">
      <div className="section-heading">
        <div>
          <p className="eyebrow">最新對話</p>
          <h2>{viewer === 'public' ? '公開頻道' : `${players.find((player) => player.id === viewer)?.name || '玩家'}的私密頻道`}</h2>
        </div>
        <div className="story-heading-actions">
          {hiddenCount > 0 && (
            <button type="button" aria-expanded={showHistory} onClick={() => setShowHistory((value) => !value)}>
              <ClockCounterClockwise />
              {showHistory ? '收起歷史' : `歷史對話（${hiddenCount}）`}
            </button>
          )}
          {viewer === 'public' ? <Scroll size={20} /> : <LockKey size={20} />}
        </div>
      </div>

      <div className="story-with-dm">
        <div className="dm-portrait-column">
          <div className="dm-portrait-frame">
            <Suspense fallback={<div className="dm-portrait dm-portrait-fallback" aria-hidden="true" />}>
              <DMTable
                speaking={speaking && !loading && !diceRolling}
                thinking={loading}
                combatActive={combatActive}
                checkPending={checkPending && !diceRolling}
                rolling={diceRolling}
                rollOutcome={diceOutcome}
                avatarScale={dmScale}
              />
            </Suspense>
          </div>
          <label className="dm-scale-control">
            <span className="dm-scale-label">
              <ArrowsOutSimple size={14} weight="bold" />
              DM 大小
              <strong>{Math.round((dmScale / DM_AVATAR_SCALE_DEFAULT) * 100)}%</strong>
            </span>
            <input
              type="range"
              min={DM_AVATAR_SCALE_MIN}
              max={DM_AVATAR_SCALE_MAX}
              step={0.01}
              value={dmScale}
              aria-valuemin={DM_AVATAR_SCALE_MIN}
              aria-valuemax={DM_AVATAR_SCALE_MAX}
              aria-valuenow={dmScale}
              aria-label="地城主模型大小"
              onInput={(event) => {
                const next = clampDmAvatarScale(Number((event.target as HTMLInputElement).value));
                setDmScale(next);
              }}
              onChange={(event) => {
                const next = clampDmAvatarScale(Number(event.target.value));
                setDmScale(next);
                saveStoredDmScale(next);
              }}
            />
            <div className="dm-scale-actions">
              <button
                type="button"
                className="dm-scale-btn"
                onClick={() => {
                  const next = clampDmAvatarScale(dmScale - 0.05);
                  setDmScale(next);
                  saveStoredDmScale(next);
                }}
              >
                縮小
              </button>
              <button
                type="button"
                className="dm-scale-btn"
                onClick={() => {
                  const next = clampDmAvatarScale(dmScale + 0.05);
                  setDmScale(next);
                  saveStoredDmScale(next);
                }}
              >
                放大
              </button>
              <button
                type="button"
                className="dm-scale-reset"
                onClick={() => {
                  setDmScale(DM_AVATAR_SCALE_DEFAULT);
                  saveStoredDmScale(DM_AVATAR_SCALE_DEFAULT);
                }}
              >
                重設
              </button>
            </div>
          </label>
        </div>

        <div className="story-feed" aria-live="polite">
          <p className="story-view-hint">
            肖像操作：左鍵旋轉・右鍵平移・滾輪縮放・雙擊重設視角
          </p>
          {dialogueNotices ? <div className="story-dialogue-notices">{dialogueNotices}</div> : null}
          <AnimatePresence initial={false}>
            {displayedStory.map((entry, index) => (
              <motion.article
                layout
                key={entry.id}
                initial={{ opacity: 0, y: 14 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                transition={{ delay: Math.min(index * 0.03, 0.18) }}
                className={`story-entry ${entry.speaker.startsWith('player') ? 'story-player' : `story-${entry.speaker}`} ${entry.audience && entry.audience !== 'public' ? 'story-private' : ''}`}
              >
                <div className="story-meta">
                  <span>
                    {speakerName(entry)}
                    {entry.audience && entry.audience !== 'public' ? '／私密' : ''}
                  </span>
                  <time>{entry.time}</time>
                </div>
                <p className="story-prose">{entry.speaker === 'dm' || entry.speaker === 'system' ? formatStoryText(entry.text) : entry.text}</p>
              </motion.article>
            ))}
            {loading && (
              <motion.div key="loading" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="dm-loading">
                <HourglassMedium size={18} className="hourglass" />
                <div>
                  <span>地城主正在裁定</span>
                  <div className="loading-lines">
                    <i />
                    <i />
                    <i />
                  </div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>
    </section>
  );
}
