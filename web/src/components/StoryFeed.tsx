import { useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { ClockCounterClockwise, HourglassMedium, LockKey, Scroll } from '@phosphor-icons/react';
import type { MessageAudience, PlayerCharacter, StoryEntry } from '../types';

interface StoryFeedProps {
  story: StoryEntry[];
  players: PlayerCharacter[];
  loading: boolean;
  viewer?: MessageAudience;
}

export function StoryFeed({ story, players, loading, viewer = 'public' }: StoryFeedProps) {
  const [showHistory, setShowHistory] = useState(false);
  function speakerName(entry: StoryEntry) {
    if (entry.speaker === 'dm') return '地城主';
    if (entry.speaker === 'system') return '紀錄';
    return players.find((player) => player.id === entry.speaker)?.name || '冒險者';
  }
  const visibleStory = story.filter((entry) => !entry.audience || entry.audience === 'public' || entry.audience === viewer);
  let latestPublicDmIndex = -1;
  for (let index = visibleStory.length - 1; index >= 0; index -= 1) {
    const entry = visibleStory[index];
    if (entry.speaker === 'dm' && (!entry.audience || entry.audience === 'public')) { latestPublicDmIndex = index; break; }
  }
  const latestStory = latestPublicDmIndex >= 0 ? visibleStory.slice(latestPublicDmIndex) : visibleStory.slice(-1);
  const displayedStory = showHistory ? visibleStory : latestStory;
  const hiddenCount = Math.max(0, visibleStory.length - latestStory.length);

  return (
    <section className="story-panel" aria-label="冒險敘事">
      <div className="section-heading"><div><p className="eyebrow">最新對話</p><h2>{viewer === 'public' ? '公開頻道' : `${players.find((player) => player.id === viewer)?.name || '玩家'}的私密頻道`}</h2></div><div className="story-heading-actions">{hiddenCount > 0 && <button type="button" aria-expanded={showHistory} onClick={() => setShowHistory((value) => !value)}><ClockCounterClockwise />{showHistory ? '收起歷史' : `歷史對話（${hiddenCount}）`}</button>}{viewer === 'public' ? <Scroll size={20} /> : <LockKey size={20} />}</div></div>
      <div className="story-feed" aria-live="polite">
        <AnimatePresence initial={false}>
          {displayedStory.map((entry, index) => (
            <motion.article layout key={entry.id} initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} transition={{ delay: Math.min(index * .03, .18) }} className={`story-entry ${entry.speaker.startsWith('player') ? 'story-player' : `story-${entry.speaker}`} ${entry.audience && entry.audience !== 'public' ? 'story-private' : ''}`}>
              <div className="story-meta"><span>{speakerName(entry)}{entry.audience && entry.audience !== 'public' ? '／私密' : ''}</span><time>{entry.time}</time></div><p>{entry.text}</p>
            </motion.article>
          ))}
          {loading && <motion.div key="loading" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="dm-loading"><HourglassMedium size={18} className="hourglass" /><div><span>地城主正在裁定</span><div className="loading-lines"><i /><i /><i /></div></div></motion.div>}
        </AnimatePresence>
      </div>
    </section>
  );
}
