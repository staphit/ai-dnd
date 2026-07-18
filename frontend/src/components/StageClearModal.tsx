import { useEffect } from 'react';
import { motion } from 'framer-motion';
import { Trophy, X } from '@phosphor-icons/react';
import { useI18n } from '../i18n';

export interface StageClearInfo {
  cleared: string;
  next: string;
  title: string;
}

interface StageClearModalProps {
  info: StageClearInfo;
  onClose: () => void;
}

// Success popup for a completed act (前期/中期/後期): announced here instead
// of a journal line so the milestone can't be missed in the scrollback.
export function StageClearModal({ info, onClose }: StageClearModalProps) {
  const { lang, tz } = useI18n();
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  const entersEnding = info.next === '結局';
  return (
    <motion.div className="stage-clear-backdrop" role="presentation" initial={{ opacity: 0 }} animate={{ opacity: 1 }} onMouseDown={onClose}>
      <motion.div
        className="stage-clear-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="stage-clear-title"
        initial={{ opacity: 0, y: 20, scale: 0.96 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ type: 'spring', stiffness: 300, damping: 24 }}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <button type="button" className="stage-clear-close" onClick={onClose} aria-label={tz('關閉任務達成通知')}><X size={16} /></button>
        <div className="stage-clear-sigil" aria-hidden="true"><Trophy size={30} weight="duotone" /></div>
        <p className="eyebrow">{tz('任務達成')}</p>
        <h2 id="stage-clear-title">{lang === 'en' ? `${info.cleared} objective complete` : `${info.cleared}目標完成`}</h2>
        <p className="stage-clear-copy">
          {entersEnding ? tz('故事推向結局') : lang === 'en' ? `The story enters ${info.next}` : `故事進入${info.next}`}
          {info.title ? `——「${info.title}」` : ''}
        </p>
        <button type="button" className="stage-clear-continue" onClick={onClose}>{tz('繼續冒險')}</button>
      </motion.div>
    </motion.div>
  );
}
