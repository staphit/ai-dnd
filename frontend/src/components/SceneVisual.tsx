import { AnimatePresence, motion } from 'framer-motion';
import { ArrowClockwise, HourglassMedium, ImageSquare, MagicWand } from '@phosphor-icons/react';
import type { SceneImage } from '../types';
import type { SceneSlotInfo } from '../api';
import { MagneticButton } from './MagneticButton';
import { useI18n, type Language } from '../i18n';

interface SceneVisualProps {
  image?: SceneImage | null;
  images?: SceneImage[];
  /** One slot per DM beat (story order): prompt captured at turn time; imageUrl empty until generated. */
  slots?: SceneSlotInfo[];
  /** Slot currently rendering, so its tile shows a spinner instead of 生成. */
  generatingSlotId?: string;
  scene: string;
  loading: boolean;
  error: string;
  canGenerate: boolean;
  onGenerate: () => void;
  onSelect: (image: SceneImage) => void;
  onGenerateSlot?: (slotId: string) => void;
}

function slotTime(createdAt: number) {
  return new Intl.DateTimeFormat('zh-TW', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(createdAt));
}

// Generation costs time (and possibly quota); a stray click should not fire it.
function confirmGenerate(label: string, regenerate: boolean, lang: Language) {
  if (lang === 'en') {
    return window.confirm(
      regenerate
        ? `Regenerate the scene art for 「${label}」? The current image for this beat will be replaced.`
        : `Generate scene art for 「${label}」? It usually takes tens of seconds; the game continues meanwhile.`,
    );
  }
  return window.confirm(
    regenerate
      ? `要重新生成「${label}」的場景圖嗎？此幕現有圖片會被新圖取代。`
      : `要生成「${label}」的場景圖嗎？通常需要數十秒，期間遊戲可繼續進行。`,
  );
}

export function SceneVisual({ image, images = [], slots = [], generatingSlotId = '', scene, loading, error, canGenerate, onGenerate, onSelect, onGenerateSlot }: SceneVisualProps) {
  const { lang, tz } = useI18n();
  const gallery = images.length > 0 ? images : image ? [image] : [];
  // Prompt recorded for the beat the main image belongs to.
  const selectedSlot = image ? slots.find((slot) => slot.imageUrl && slot.imageUrl === image.url) : undefined;

  return (
    <section className="scene-visual" aria-label={tz('場景插圖')}>
      <AnimatePresence mode="wait">
        {image ? (
          <motion.img
            key={image.url}
            src={image.url}
            alt={lang === 'en' ? `AI scene art of ${image.scene}` : `${image.scene}的 AI 場景插圖`}
            initial={{ opacity: 0, scale: 1.025 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: .55, ease: [0.16, 1, 0.3, 1] }}
          />
        ) : (
          <motion.div key="empty" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="scene-empty">
            <div className="scene-glyph"><ImageSquare size={28} /></div>
            <strong>{tz('讓這個場景顯影')}</strong>
            <span>{tz('依據最新 DM 敘事生成一張原創環境插圖')}</span>
          </motion.div>
        )}
      </AnimatePresence>

      {loading && (
        <div className="image-loading" aria-live="polite">
          <div className="image-scan" />
          <MagicWand size={22} />
          <strong>{tz('場景正在顯影')}</strong>
          <span>{tz('通常需要數十秒，文字遊戲仍可繼續')}</span>
        </div>
      )}

      <div className="scene-caption">
        <div>
          <span>{image ? image.scene : scene}</span>
          <small>{image ? `${image.model}／${image.createdAt}` : tz('尚未生成插圖')}</small>
          {selectedSlot?.imagePrompt && (
            <small className="scene-caption-prompt" title={selectedSlot.imagePrompt}>
              prompt：{selectedSlot.imagePrompt}
            </small>
          )}
        </div>
        <MagneticButton
          variant="quiet"
          disabled={loading || !canGenerate}
          onClick={() => {
            if (!confirmGenerate(image ? image.scene : scene, Boolean(image), lang)) return;
            if (selectedSlot && onGenerateSlot) onGenerateSlot(selectedSlot.id);
            else onGenerate();
          }}
        >
          {image ? <ArrowClockwise size={16} /> : <MagicWand size={16} />}
          {image ? tz('重新生成') : tz('生成場景')}
        </MagneticButton>
      </div>

      {slots.length > 0 && (
        <div className="scene-gallery scene-slot-row" aria-label={tz('各回合場景圖（依劇情順序）')}>
          {slots.map((slot) => {
            const selected = Boolean(slot.imageUrl) && slot.imageUrl === image?.url;
            const rendering = slot.id === generatingSlotId && loading;
            const label = slot.scene || tz('場景');
            const tooltip = slot.imagePrompt ? `${label}・${slotTime(slot.createdAt)}\nprompt：${slot.imagePrompt}` : `${label}・${slotTime(slot.createdAt)}`;
            return (
              <div key={slot.id} className={`scene-gallery-item${selected ? ' selected' : ''}${slot.imageUrl ? '' : ' scene-slot-pending'}`}>
                {slot.imageUrl ? (
                  <button
                    type="button"
                    className={selected ? 'selected' : undefined}
                    onClick={() => onSelect({ url: slot.imageUrl, scene: slot.scene, createdAt: slotTime(slot.createdAt), model: slot.imageModel || 'Image' })}
                    title={tooltip}
                    aria-pressed={selected}
                  >
                    <span className="scene-gallery-thumb" aria-hidden="true">
                      <img src={slot.imageUrl} alt="" loading="lazy" decoding="async" />
                    </span>
                    <span className="scene-gallery-label">{label}</span>
                  </button>
                ) : (
                  <button
                    type="button"
                    className="scene-slot-generate"
                    disabled={loading || !canGenerate || !onGenerateSlot}
                    onClick={() => { if (confirmGenerate(label, false, lang)) onGenerateSlot?.(slot.id); }}
                    title={tooltip}
                  >
                    <span className="scene-gallery-thumb scene-gallery-thumb-empty" aria-hidden="true">
                      {rendering ? <HourglassMedium size={18} className="hourglass" /> : <MagicWand size={18} />}
                    </span>
                    <span className="scene-gallery-label">{rendering ? tz('生成中…') : `${tz('生成・')}${label}`}</span>
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}

      {slots.length === 0 && gallery.length > 1 && (
        <div className="scene-gallery" aria-label={tz('過去場景圖片')}>
          {gallery.map((entry, index) => {
            const selected = entry.url === image?.url;
            const label = entry.scene || `${tz('場景')} ${index + 1}`;
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
      {!canGenerate && !error && <p className="image-helper">{tz('先以 codex login 登入，或在設定改用本地 SD Forge，即可生成圖片。')}</p>}
    </section>
  );
}
