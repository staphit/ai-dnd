import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { createLevel3Character } from '../rules/characters';
import { ActionComposer } from './ActionComposer';
import { DiceTray } from './DiceTray';

describe('player action flow', () => {
  it('allows an empty action and converts it into an explicit no-action choice', () => {
    const onSubmit = vi.fn();
    render(<ActionComposer player="player1" name="艾拉" className="遊俠" disabled={false} partySize={2} onSubmit={onSubmit} onUnlock={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /鎖定行動/ }));
    expect(onSubmit).toHaveBeenCalledWith('player1', '本回合不行動，保持警戒。');
  });

  it('allows a locked action to be unlocked before the party advances', () => {
    const onUnlock = vi.fn();
    render(<ActionComposer player="player1" name="艾拉" className="遊俠" pending="檢查門鎖" disabled={false} partySize={2} onSubmit={vi.fn()} onUnlock={onUnlock} />);
    fireEvent.click(screen.getByRole('button', { name: /修改行動/ }));
    expect(onUnlock).toHaveBeenCalledWith('player1');
  });

  it('shows only the required d20 check rather than a permanent manual dice tray', () => {
    const player = createLevel3Character('player1', '艾拉', '遊俠');
    render(<DiceTray players={[player]} requiredCheck={{ character: '艾拉', ability: '敏捷', skill: '隱匿', dc: 14, reason: '避開守衛。' }} onResult={vi.fn()} onRequiredRoll={vi.fn()} />);
    expect(screen.getByText('現在擲 d20')).toBeVisible();
    expect(screen.getByText(/總值需達到 DC 14/)).toBeVisible();
    expect(screen.queryByText('d4')).not.toBeInTheDocument();
    expect(screen.queryByText('自訂')).not.toBeInTheDocument();
  });
});
