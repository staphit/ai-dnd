import { beforeEach, describe, expect, it } from 'vitest';
import { getActiveCampaignId, readLegacyVault, setActiveCampaignId } from './campaign-storage';
import { makeCampaign } from './test/fixtures';

describe('legacy vault reader', () => {
  beforeEach(() => localStorage.clear());

  it('returns the completed campaigns from the v2 vault without touching it', () => {
    const played = makeCampaign({ id: 'campaign-legacy-1', title: '正在玩的舊戰役', round: 9 });
    const pristine = makeCampaign({ id: 'campaign-legacy-2', title: '從未開團', setupComplete: false });
    const raw = JSON.stringify({ version: 2, activeId: played.id, campaigns: { [played.id!]: played, [pristine.id!]: pristine } });
    localStorage.setItem('dnd-duet-web-v2-vault', raw);

    const campaigns = readLegacyVault();
    expect(campaigns.map((entry) => entry.title)).toEqual(['正在玩的舊戰役']);
    expect(campaigns[0].id).toBe('campaign-legacy-1');
    expect(localStorage.getItem('dnd-duet-web-v2-vault')).toBe(raw);
  });

  it('falls back to the v1 single-campaign slot', () => {
    const legacy = makeCampaign({ id: undefined, title: 'V1 舊戰役' });
    localStorage.setItem('dnd-duet-web-v1', JSON.stringify(legacy));
    expect(readLegacyVault().map((entry) => entry.title)).toEqual(['V1 舊戰役']);
  });

  it('returns an empty list when no legacy data exists', () => {
    expect(readLegacyVault()).toEqual([]);
    localStorage.setItem('dnd-duet-web-v2-vault', '{broken json');
    expect(readLegacyVault()).toEqual([]);
  });
});

describe('active campaign id', () => {
  beforeEach(() => localStorage.clear());

  it('stores and clears the last-active id', () => {
    expect(getActiveCampaignId()).toBe('');
    setActiveCampaignId('campaign-abc');
    expect(getActiveCampaignId()).toBe('campaign-abc');
    expect(localStorage.getItem('dnd-duet-active-id')).toBe('campaign-abc');
    setActiveCampaignId('');
    expect(getActiveCampaignId()).toBe('');
  });
});
