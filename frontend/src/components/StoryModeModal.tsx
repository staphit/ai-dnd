import { useEffect } from 'react';
import { motion } from 'framer-motion';
import { BookOpenText, Sparkle, X } from '@phosphor-icons/react';

interface StoryModeModalProps {
  busy: boolean;
  onPick: (mode: 'scripted' | 'freeform') => void;
  onClose: () => void;
}

// Shown before creating a campaign whose preset ships a hand-written script
// module: the duet chooses between following the fixed script (recommended)
// and letting the AI DM improvise freely.
export function StoryModeModal({ busy, onPick, onClose }: StoryModeModalProps) {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose]);
  return (
    <motion.div
      className="story-mode-backdrop"
      role="presentation"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      onMouseDown={onClose}
    >
      <motion.div
        className="story-mode-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="story-mode-title"
        initial={{ opacity: 0, y: 24, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ type: 'spring', stiffness: 280, damping: 26 }}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <button type="button" className="story-mode-close" onClick={onClose} aria-label="關閉故事模式選擇"><X size={18} /></button>
        <div className="story-mode-sigil" aria-hidden="true"><BookOpenText size={28} weight="duotone" /></div>
        <h2 id="story-mode-title">要怎麼進行這個故事？</h2>
        <p>這個劇本備有手寫的既定路線，也可以完全交給 AI 地城主即興發揮。</p>
        <div className="story-mode-actions">
          <button type="button" className="story-mode-scripted" disabled={busy} onClick={() => onPick('scripted')}>
            <strong>既定劇本<i className="story-mode-badge">推薦</i></strong>
            <small>依手寫劇本推進，每一幕提供固定選項，通往光明或沉沒結局</small>
          </button>
          <button type="button" className="story-mode-freeform" disabled={busy} onClick={() => onPick('freeform')}>
            <strong><Sparkle size={14} weight="fill" aria-hidden="true" />AI 自由走向</strong>
            <small>由 AI 地城主即興敘事，可自由輸入任何行動</small>
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}
