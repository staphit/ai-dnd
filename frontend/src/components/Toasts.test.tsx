import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastBell, ToastStack, useToasts } from './Toasts';

// Minimal harness mirroring how App owns the store and hands plain props to
// the stack (top-right toasts) and the bell (Topbar history).
function Harness() {
  const { toasts, history, push, dismiss, clearHistory } = useToasts();
  return (
    <>
      <button type="button" onClick={() => push('error', '伺服器錯誤（HTTP 500）')}>fire-error</button>
      <button type="button" onClick={() => push('info', '【行動駁回】艾拉：法術位已耗盡。')}>fire-info</button>
      <ToastStack toasts={toasts} onDismiss={dismiss} />
      <ToastBell history={history} onClear={clearHistory} />
    </>
  );
}

describe('toast notifications', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('renders a toast and auto-dismisses it after 10 seconds', () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole('button', { name: 'fire-error' }));
    expect(screen.getByText('伺服器錯誤（HTTP 500）')).toBeVisible();

    act(() => { vi.advanceTimersByTime(9000); });
    expect(screen.getByText('伺服器錯誤（HTTP 500）')).toBeVisible();
    act(() => { vi.advanceTimersByTime(1100); });
    expect(screen.queryByText('伺服器錯誤（HTTP 500）')).not.toBeInTheDocument();
  });

  it('supports manual dismissal via the close button', () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole('button', { name: 'fire-info' }));
    expect(screen.getByText('【行動駁回】艾拉：法術位已耗盡。')).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: '關閉通知' }));
    expect(screen.queryByText('【行動駁回】艾拉：法術位已耗盡。')).not.toBeInTheDocument();
  });

  it('keeps dismissed toasts in the bell history, newest first, with a clear-all', () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole('button', { name: 'fire-error' }));
    fireEvent.click(screen.getByRole('button', { name: 'fire-info' }));
    act(() => { vi.advanceTimersByTime(10100); });
    // Both toasts auto-dismissed; the badge still counts the session history.
    expect(screen.queryByText('伺服器錯誤（HTTP 500）')).not.toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '通知紀錄' }));
    const items = screen.getAllByRole('listitem');
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent('【行動駁回】艾拉：法術位已耗盡。');
    expect(items[0]).toHaveTextContent(/\d{2}:\d{2}/);
    expect(items[1]).toHaveTextContent('伺服器錯誤（HTTP 500）');

    fireEvent.click(screen.getByRole('button', { name: '全部清除' }));
    expect(screen.getByText('目前沒有通知。')).toBeVisible();
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument();
  });
});
