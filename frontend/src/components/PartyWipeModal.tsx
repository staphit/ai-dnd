import { motion } from 'framer-motion';
import { ArrowCounterClockwise, Skull } from '@phosphor-icons/react';
import { useI18n } from '../i18n';

interface PartyWipeModalProps {
  busy: boolean;
  onEndStory: () => void;
  onRetry: () => void;
}

// Shown when every party combatant is defeated while combat is still active:
// the duet chooses between a narrated final chapter and replaying the fight
// from its opening snapshot.
export function PartyWipeModal({ busy, onEndStory, onRetry }: PartyWipeModalProps) {
  const { tz } = useI18n();
  return (
    <motion.div className="party-wipe-backdrop" role="presentation" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
      <motion.div
        className="party-wipe-modal"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="party-wipe-title"
        initial={{ opacity: 0, y: 24, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ type: 'spring', stiffness: 280, damping: 26 }}
      >
        <div className="party-wipe-sigil" aria-hidden="true"><Skull size={30} weight="fill" /></div>
        <h2 id="party-wipe-title">{tz('全隊倒地')}</h2>
        <p>{tz('所有冒險者都失去了戰鬥能力。這場故事要在此劃下句點，還是讓命運倒轉、重新面對這場戰鬥？')}</p>
        <div className="party-wipe-actions">
          <button type="button" className="party-wipe-retry" onClick={onRetry} disabled={busy}>
            <ArrowCounterClockwise size={17} />
            {tz('戰鬥重來')}
            <small>{tz('回到本場戰鬥開始時的狀態')}</small>
          </button>
          <button type="button" className="party-wipe-end" onClick={onEndStory} disabled={busy}>
            <Skull size={17} />
            {tz('結束故事')}
            <small>{tz('由地城主寫下這個冒險的終章')}</small>
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}
