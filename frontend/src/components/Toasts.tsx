import { useCallback, useRef, useState } from 'react';
import { Bell, Info, Warning, X } from '@phosphor-icons/react';
import { useI18n } from '../i18n';

export type ToastKind = 'error' | 'info';

export interface ToastMessage {
  id: string;
  kind: ToastKind;
  text: string;
  /** HH:mm — same 24-hour zh-TW format as the story-entry timestamps. */
  time: string;
}

const AUTO_DISMISS_MS = 10000;
const HISTORY_CAP = 50;

function stamp() {
  return new Intl.DateTimeFormat('zh-TW', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date());
}

// Tiny self-contained toast store: one hook instance owned by App; the stack
// and the bell receive plain props. No context, no libraries.
export function useToasts() {
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  // Session history, newest first, capped — survives dismissal, not reloads.
  const [history, setHistory] = useState<ToastMessage[]>([]);
  const counterRef = useRef(0);

  const dismiss = useCallback((id: string) => {
    setToasts((current) => current.filter((entry) => entry.id !== id));
  }, []);

  const push = useCallback((kind: ToastKind, text: string) => {
    const message = text.trim();
    if (!message) return;
    counterRef.current += 1;
    const entry: ToastMessage = { id: `toast-${counterRef.current}-${Date.now()}`, kind, text: message, time: stamp() };
    setToasts((current) => [...current, entry]);
    setHistory((current) => [entry, ...current].slice(0, HISTORY_CAP));
    window.setTimeout(() => dismiss(entry.id), AUTO_DISMISS_MS);
  }, [dismiss]);

  const clearHistory = useCallback(() => setHistory([]), []);

  return { toasts, history, push, dismiss, clearHistory };
}

interface ToastLayerProps {
  toasts: ToastMessage[];
  history: ToastMessage[];
  onDismiss: (id: string) => void;
  onClear: () => void;
}

// Viewport-fixed top-right overlay: the bell (with its history) floats above
// the page next to the live toasts, so notifications never require scrolling.
export function ToastLayer({ toasts, history, onDismiss, onClear }: ToastLayerProps) {
  return (
    <div className="toast-layer">
      <ToastBell history={history} onClear={onClear} />
      <ToastStack toasts={toasts} onDismiss={onDismiss} />
    </div>
  );
}

interface ToastStackProps {
  toasts: ToastMessage[];
  onDismiss: (id: string) => void;
}

// Toast column inside the fixed layer; each toast auto-dismisses after 10
// seconds and can be closed manually.
export function ToastStack({ toasts, onDismiss }: ToastStackProps) {
  const { tz } = useI18n();
  if (toasts.length === 0) return null;
  return (
    <div className="toast-stack" role="status" aria-live="polite" aria-label={tz('通知')}>
      {toasts.map((toast) => (
        <div key={toast.id} className={`toast toast-${toast.kind}`}>
          {toast.kind === 'error' ? <Warning size={16} weight="fill" aria-hidden="true" /> : <Info size={16} weight="fill" aria-hidden="true" />}
          <p>{toast.text}</p>
          <button type="button" aria-label={tz('關閉通知')} onClick={() => onDismiss(toast.id)}><X size={14} /></button>
        </div>
      ))}
    </div>
  );
}

interface ToastBellProps {
  history: ToastMessage[];
  onClear: () => void;
}

// Bell with a badge count; clicking opens the session notification history
// (newest first) with a clear-all button.
export function ToastBell({ history, onClear }: ToastBellProps) {
  const { tz } = useI18n();
  const [open, setOpen] = useState(false);
  return (
    <div className="toast-bell-wrap">
      <button type="button" className="toast-bell" aria-label={tz('通知紀錄')} aria-expanded={open} onClick={() => setOpen((value) => !value)}>
        <Bell size={18} />
        {history.length > 0 && <span className="toast-bell-badge">{history.length > 99 ? '99+' : history.length}</span>}
      </button>
      {open && (
        <div className="toast-history" role="region" aria-label={tz('通知紀錄清單')}>
          <header>
            <strong>{tz('通知紀錄')}</strong>
            <button type="button" className="toast-history-clear" disabled={history.length === 0} onClick={onClear}>{tz('全部清除')}</button>
          </header>
          {history.length === 0 ? (
            <p className="toast-history-empty">{tz('目前沒有通知。')}</p>
          ) : (
            <ul>
              {history.map((entry) => (
                <li key={entry.id} className={`toast-history-${entry.kind}`}>
                  <small>{entry.time}</small>
                  <span>{entry.text}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
