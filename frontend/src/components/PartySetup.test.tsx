import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { PartySetup } from './PartySetup';
import { createLevel3Character } from '../rules/characters';

const initialPlayers = [
  createLevel3Character('player1', '預設一', '戰士'),
  createLevel3Character('player2', '預設二', '牧師'),
];

describe('PartySetup', () => {
  it('creates a three-player party with the selected names and classes', async () => {
    const user = userEvent.setup();
    const onComplete = vi.fn();
    render(<PartySetup initialTitle="測試戰役" initialPlayers={initialPlayers} onComplete={onComplete} />);

    await user.click(screen.getByRole('button', { name: '3人' }));
    await user.clear(screen.getByLabelText('玩家 1 角色名稱'));
    await user.type(screen.getByLabelText('玩家 1 角色名稱'), '阿莎');
    await user.selectOptions(screen.getByLabelText('玩家 1 職業'), '法師');
    await user.clear(screen.getByLabelText('玩家 2 角色名稱'));
    await user.type(screen.getByLabelText('玩家 2 角色名稱'), '布蘭');
    await user.selectOptions(screen.getByLabelText('玩家 2 職業'), '野蠻人');
    await user.clear(screen.getByLabelText('玩家 3 角色名稱'));
    await user.type(screen.getByLabelText('玩家 3 角色名稱'), '希雅');
    await user.selectOptions(screen.getByLabelText('玩家 3 職業'), '德魯伊');
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));

    expect(onComplete).toHaveBeenCalledOnce();
    const setup = onComplete.mock.calls[0][0];
    expect(setup.players.map((player: { id: string; name: string; className: string }) => [player.id, player.name, player.className])).toEqual([
      ['player1', '阿莎', '法師'],
      ['player2', '布蘭', '野蠻人'],
      ['player3', '希雅', '德魯伊'],
    ]);
    expect(setup.players[0].spellcasting.slots).toHaveLength(2);
    expect(setup.players[2].resources.some((entry: { id: string }) => entry.id === 'wild_shape')).toBe(true);
  });

  it('uses the selected story preset when the adventure starts', async () => {
    const user = userEvent.setup();
    const onComplete = vi.fn();
    render(<PartySetup initialTitle='灰燼王冠' initialPlayers={initialPlayers} onComplete={onComplete} />);

    await user.click(screen.getByRole('button', { name: /血月特快車/ }));
    expect(screen.getByDisplayValue('血月特快車')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));

    expect(onComplete).toHaveBeenCalledWith(expect.objectContaining({
      title: '血月特快車',
      storyId: 'blood-moon-express',
    }));
  });

  it('rejects duplicate character names', async () => {
    const user = userEvent.setup();
    const onComplete = vi.fn();
    render(<PartySetup initialTitle="測試戰役" initialPlayers={initialPlayers} onComplete={onComplete} />);
    await user.clear(screen.getByLabelText('玩家 2 角色名稱'));
    await user.type(screen.getByLabelText('玩家 2 角色名稱'), '預設一');
    await user.click(screen.getByRole('button', { name: /開始冒險/ }));
    expect(screen.getByRole('alert')).toHaveTextContent('角色名稱不能重複');
    expect(onComplete).not.toHaveBeenCalled();
  });
});
