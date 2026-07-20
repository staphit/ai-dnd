import { motion } from 'framer-motion';
import { storySpeakerLabel } from '../../app/app-utils';
import type { Campaign, PlayerId, StoryEntry } from '../../types';
import { MagneticButton } from '../MagneticButton';

interface JournalPageProps {
  campaign: Campaign;
  story: StoryEntry[];
  novelBusy: boolean;
  novelPlayerId: PlayerId | '';
  onNovelPlayerChange: (playerId: PlayerId) => void;
  onExportNovel: (playerId: PlayerId) => void;
}

export function JournalPage({
  campaign,
  story,
  novelBusy,
  novelPlayerId,
  onNovelPlayerChange,
  onExportNovel,
}: JournalPageProps) {
  const narrator = (novelPlayerId || campaign.players[0]?.id || '') as PlayerId;
  return (
    <motion.main key="journal" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page">
      <div className="page-intro">
        <p className="eyebrow">戰役記憶</p>
        <h2>{campaign.title}</h2>
        <p>公開與私密訊息都保存在伺服器戰役資料庫與匯出檔中。</p>
      </div>
      {campaign.id && campaign.players.length > 0 && (
        <section className="novel-export" aria-label="輸出劇本">
          <div>
            <strong>輸出第一人稱劇本</strong>
            <span>AI 會把整場冒險改寫成所選角色視角的小說（含對話），下載為 txt。故事完結後輸出最完整。</span>
          </div>
          <select value={narrator} onChange={(event) => onNovelPlayerChange(event.target.value as PlayerId)}>
            {campaign.players.map((player) => <option key={player.id} value={player.id}>{player.name}的視角</option>)}
          </select>
          <MagneticButton disabled={novelBusy} onClick={() => onExportNovel(narrator)}>
            {novelBusy ? 'AI 撰寫中（約 1–2 分鐘）…' : '輸出劇本 TXT'}
          </MagneticButton>
        </section>
      )}
      <div className="journal-list">
        {story.map((entry, index) => (
          <article key={entry.id} className={entry.audience && entry.audience !== 'public' ? 'journal-private' : ''}>
            <span>{String(index + 1).padStart(2, '0')}</span>
            <div>
              <small>{entry.time}／{storySpeakerLabel(entry, campaign.players)}</small>
              <p>{entry.text}</p>
            </div>
          </article>
        ))}
      </div>
    </motion.main>
  );
}
