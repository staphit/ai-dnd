import { render, screen, waitFor } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { StoryFeed } from './StoryFeed';
import { makePlayer } from '../test/fixtures';

const players = [makePlayer('player1', '艾拉', { className: '遊俠' }), makePlayer('player2', '米拉', { className: '法師' })];
const story = [
  { id: 'public', speaker: 'dm' as const, text: '所有人都看見門打開。', time: '10:00', audience: 'public' as const },
  { id: 'private-1', speaker: 'dm' as const, text: '艾拉注意到牆上的暗號。', time: '10:01', audience: 'player1' as const },
  { id: 'private-2', speaker: 'dm' as const, text: '米拉感覺到魔法。', time: '10:01', audience: 'player2' as const },
];

describe('StoryFeed privacy', () => {
  it('hides every private message in the public view', () => {
    render(<StoryFeed story={story} players={players} loading={false} viewer="public" />);
    expect(screen.getByText('所有人都看見門打開。')).toBeVisible();
    expect(screen.queryByText('艾拉注意到牆上的暗號。')).not.toBeInTheDocument();
    expect(screen.queryByText('米拉感覺到魔法。')).not.toBeInTheDocument();
  });

  it('shows only the selected player private message', () => {
    render(<StoryFeed story={story} players={players} loading={false} viewer="player1" />);
    expect(screen.getByText('艾拉注意到牆上的暗號。')).toBeVisible();
    expect(screen.queryByText('米拉感覺到魔法。')).not.toBeInTheDocument();
  });

  it('keeps older rounds behind the history button', async () => {
    render(<StoryFeed story={[
      { id: 'old', speaker: 'dm', text: '舊場景。', time: '09:00', audience: 'public' },
      { id: 'action', speaker: 'player1', text: '向前走。', time: '09:01', audience: 'public' },
      { id: 'latest', speaker: 'dm', text: '最新場景。', time: '09:02', audience: 'public' },
    ]} players={players} loading={false} viewer="public" />);
    expect(screen.getByText('最新場景。')).toBeVisible();
    expect(screen.queryByText('舊場景。')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /歷史對話/ }));
    await waitFor(() => expect(screen.getByText('舊場景。')).toBeVisible());
  });
});

describe('StoryFeed scripted choice chip', () => {
  it('lifts the 【選擇】 lead line into a chip inside one grid cell', () => {
    const { container } = render(<StoryFeed story={[
      { id: 'chip', speaker: 'dm', text: '【選擇】沿著泥痕走向祭壇\n祭壇前的泥痕突然中斷。', time: '10:02', audience: 'public' },
    ]} players={players} loading={false} viewer="public" />);
    expect(screen.getByText('▸ 選擇：沿著泥痕走向祭壇')).toBeVisible();
    expect(screen.getByText('祭壇前的泥痕突然中斷。')).toBeVisible();
    // Chip and prose share one wrapper so the two-column entry grid keeps
    // meta | body alignment (regression: prose squeezed into the meta column).
    const body = container.querySelector('.story-body');
    expect(body).not.toBeNull();
    expect(body!.querySelector('.story-choice-chip')).not.toBeNull();
    expect(body!.querySelector('.story-prose')).not.toBeNull();
  });
});
