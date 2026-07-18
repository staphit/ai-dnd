import { useEffect, useRef, useState } from 'react';
import { ArrowCounterClockwise, ChatTeardropText, PaperPlaneRight, X } from '@phosphor-icons/react';

export interface RevisionChatLine {
  id: string;
  role: 'player' | 'system';
  text: string;
  time: string;
}

interface StoryRevisionPanelProps {
  open: boolean;
  onClose: () => void;
  loading: boolean;
  disabled: boolean;
  disabledReason?: string;
  previousDraft: string;
  chat: RevisionChatLine[];
  onSubmit: (note: string) => void;
}

export function StoryRevisionPanel({
  open,
  onClose,
  loading,
  disabled,
  disabledReason,
  previousDraft,
  chat,
  onSubmit,
}: StoryRevisionPanelProps) {
  const [draft, setDraft] = useState('');
  const endRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    endRef.current?.scrollIntoView({ block: 'end', behavior: 'smooth' });
  }, [open, chat.length, loading]);

  if (!open) return null;

  function submit() {
    const note = draft.trim();
    if (!note || loading || disabled) return;
    onSubmit(note);
    setDraft('');
  }

  return (
    <aside className="story-revision-panel" aria-label="修正 DM 對話">
      <header className="story-revision-head">
        <div>
          <p className="eyebrow">修正小窗</p>
          <strong>
            <ChatTeardropText size={16} weight="fill" />
            修正上一則 DM 對話
          </strong>
        </div>
        <button type="button" className="story-revision-close" aria-label="關閉修正小窗" onClick={onClose}>
          <X size={16} />
        </button>
      </header>

      <div className="story-revision-draft">
        <span>目前草稿</span>
        <p>{previousDraft || '尚無可修正的 DM 對話。'}</p>
      </div>

      <div className="story-revision-chat" aria-live="polite">
        {chat.length === 0 && (
          <p className="story-revision-empty">
            告訴 DM 哪裡不對（事實錯誤、語氣、遺漏、矛盾…），再按「重寫」。只改敘事，不重算 HP／XP。
          </p>
        )}
        {chat.map((line) => (
          <div key={line.id} className={`story-revision-line story-revision-${line.role}`}>
            <span>{line.role === 'player' ? '你' : '系統'}</span>
            <p>{line.text}</p>
            <time>{line.time}</time>
          </div>
        ))}
        {loading && (
          <div className="story-revision-line story-revision-system">
            <span>系統</span>
            <p>DM 正在依你的說明就地修正上一則對話…</p>
          </div>
        )}
        <div ref={endRef} />
      </div>

      {disabled && disabledReason && <p className="story-revision-disabled">{disabledReason}</p>}

      <form
        className="story-revision-form"
        onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}
      >
        <textarea
          value={draft}
          rows={3}
          disabled={loading || disabled}
          placeholder="例如：NPC 不該知道我們的名字；把氣氛改得更緊繃；別跳過搜查結果…"
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
              event.preventDefault();
              submit();
            }
          }}
        />
        <div className="story-revision-actions">
          <small>Ctrl+Enter 送出</small>
          <button type="submit" disabled={loading || disabled || !draft.trim()}>
            {loading ? <ArrowCounterClockwise size={15} className="hourglass" /> : <PaperPlaneRight size={15} weight="fill" />}
            {loading ? '重寫中…' : '依修正重寫'}
          </button>
        </div>
      </form>
    </aside>
  );
}
