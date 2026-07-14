import { AnimatePresence, motion } from 'framer-motion';
import { HourglassMedium, Scroll } from '@phosphor-icons/react';
import type { PlayerCharacter, StoryEntry } from '../types';

interface StoryFeedProps {
  story: StoryEntry[];
  players: PlayerCharacter[];
  loading: boolean;
}

export function StoryFeed({ story, players, loading }: StoryFeedProps) {
  function speakerName(entry: StoryEntry) {
    if (entry.speaker === 'dm') return '地城主';
    if (entry.speaker === 'system') return '紀錄';
    return players.find((player) => player.id === entry.speaker)?.name || '冒險者';
  }

  return (
    <section className="story-panel" aria-label="冒險敘事">
      <div className="section-heading">
        <div>
          <p className="eyebrow">即時敘事</p>
          <h2>禮拜堂內部</h2>
        </div>
        <Scroll size={20} aria-hidden="true" />
      </div>

      <div className="story-feed" aria-live="polite">
        <AnimatePresence initial={false}>
          {story.map((entry, index) => (
            <motion.article
              layout
              key={entry.id}
              initial={{ opacity: 0, y: 14 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              transition={{ type: 'spring', stiffness: 100, damping: 20, delay: Math.min(index * 0.03, 0.18) }}
              className={`story-entry ${entry.speaker.startsWith('player') ? 'story-player' : `story-${entry.speaker}`}`}
            >
              <div className="story-meta">
                <span>{speakerName(entry)}</span>
                <time>{entry.time}</time>
              </div>
              <p>{entry.text}</p>
            </motion.article>
          ))}
          {loading && (
            <motion.div
              key="loading"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              className="dm-loading"
            >
              <HourglassMedium size={18} className="hourglass" />
              <div>
                <span>地城主正在裁定</span>
                <div className="loading-lines"><i /><i /><i /></div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </section>
  );
}
