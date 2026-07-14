import { AnimatePresence, motion } from 'framer-motion';
import { HourglassMedium, LockKey, Scroll } from '@phosphor-icons/react';
import type { MessageAudience, PlayerCharacter, StoryEntry } from '../types';

interface StoryFeedProps {
  story: StoryEntry[];
  players: PlayerCharacter[];
  loading: boolean;
  viewer?: MessageAudience;
}

export function StoryFeed({ story, players, loading, viewer = 'public' }: StoryFeedProps) {
  function speakerName(entry: StoryEntry) {
    if (entry.speaker === 'dm') return '地城主';
    if (entry.speaker === 'system') return '紀錄';
    return players.find((player) => player.id === entry.speaker)?.name || '冒險者';
  }
  const visibleStory = story.filter((entry) => !entry.audience || entry.audience === 'public' || entry.audience === viewer);

  return (
    <section className="story-panel" aria-label="冒險敘事">
      <div className="section-heading"><div><p className="eyebrow">即時敘事</p><h2>{viewer === 'public' ? '公開頻道' : `${players.find((player) => player.id === viewer)?.name || '玩家'}的私密頻道`}</h2></div>{viewer === 'public' ? <Scroll size={20} /> : <LockKey size={20} />}</div>
      <div className="story-feed" aria-live="polite">
        <AnimatePresence initial={false}>
          {visibleStory.map((entry, index) => (
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
