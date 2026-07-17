import { AnimatePresence, motion } from 'framer-motion';
import { ArrowClockwise, ImageSquare, MagicWand } from '@phosphor-icons/react';
import type { SceneImage } from '../types';
import { MagneticButton } from './MagneticButton';

interface SceneVisualProps {
  image?: SceneImage | null;
  images?: SceneImage[];
  scene: string;
  loading: boolean;
  error: string;
  canGenerate: boolean;
  onGenerate: () => void;
  onSelect: (image: SceneImage) => void;
}

export function SceneVisual({ image, images = [], scene, loading, error, canGenerate, onGenerate, onSelect }: SceneVisualProps) {
  const gallery = images.length > 0 ? images : image ? [image] : [];

  return (
    <section className="scene-visual" aria-label="場景插圖">
      <AnimatePresence mode="wait">
        {image ? (
          <motion.img
            key={image.url}
            src={image.url}
            alt={`${image.scene}的 AI 場景插圖`}
            initial={{ opacity: 0, scale: 1.025 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: .55, ease: [0.16, 1, 0.3, 1] }}
          />
        ) : (
          <motion.div key="empty" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="scene-empty">
            <div className="scene-glyph"><ImageSquare size={28} /></div>
            <strong>讓這個場景顯影</strong>
            <span>依據最新 DM 敘事生成一張原創環境插圖</span>
          </motion.div>
        )}
      </AnimatePresence>

      {loading && (
        <div className="image-loading" aria-live="polite">
          <div className="image-scan" />
          <MagicWand size={22} />
          <strong>場景正在顯影</strong>
          <span>通常需要數十秒，文字遊戲仍可繼續</span>
        </div>
      )}

      <div className="scene-caption">
        <div>
          <span>{image ? image.scene : scene}</span>
          <small>{image ? `${image.model}／${image.createdAt}` : '尚未生成插圖'}</small>
        </div>
        <MagneticButton variant="quiet" disabled={loading || !canGenerate} onClick={onGenerate}>
          {image ? <ArrowClockwise size={16} /> : <MagicWand size={16} />}
          {image ? '重新生成' : '生成場景'}
        </MagneticButton>
      </div>

      {gallery.length > 1 && (
        <div className="scene-gallery" aria-label="過去場景圖片">
          {gallery.map((entry, index) => {
            const selected = entry.url === image?.url;
            const label = entry.scene || `場景 ${index + 1}`;
            return (
              <div
                key={`${entry.url}-${index}`}
                className={`scene-gallery-item${selected ? ' selected' : ''}`}
              >
                <button
                  type="button"
                  className={selected ? 'selected' : undefined}
                  onClick={() => onSelect(entry)}
                  title={`${label}${entry.createdAt ? ` · ${entry.createdAt}` : ''}`}
                  aria-pressed={selected}
                >
                  <span className="scene-gallery-thumb" aria-hidden="true">
                    {entry.url ? (
                      <img src={entry.url} alt="" loading="lazy" decoding="async" />
                    ) : (
                      <span className="scene-gallery-thumb-empty">
                        <ImageSquare size={18} />
                      </span>
                    )}
                  </span>
                  <span className="scene-gallery-label">{label}</span>
                </button>
              </div>
            );
          })}
        </div>
      )}

      {error && <p className="image-error">{error}</p>}
      {!canGenerate && !error && <p className="image-helper">先以 codex login 登入，或在設定改用本地 SD Forge，即可生成圖片。</p>}
    </section>
  );
}
