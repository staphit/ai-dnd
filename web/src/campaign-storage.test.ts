import { beforeEach, describe, expect, it } from 'vitest';
import { initialCampaign } from './data';
import { importCampaign, listCampaigns, loadActiveCampaign } from './campaign-storage';

describe('campaign vault', () => {
  beforeEach(() => localStorage.clear());

  it('copies the legacy save without deleting or changing it', () => {
    const legacy = { ...initialCampaign, setupComplete: true, title: '正在玩的舊戰役', round: 9 };
    const raw = JSON.stringify(legacy);
    localStorage.setItem('dnd-duet-web-v1', raw);
    const loaded = loadActiveCampaign(initialCampaign);
    expect(loaded.title).toBe('正在玩的舊戰役');
    expect(loaded.round).toBe(9);
    expect(localStorage.getItem('dnd-duet-web-v1')).toBe(raw);
    expect(localStorage.getItem('dnd-duet-web-v2-vault')).toContain('正在玩的舊戰役');
  });

  it('imports a campaign without switching the active campaign', () => {
    const active = loadActiveCampaign(initialCampaign);
    importCampaign(JSON.stringify({ format: 'dnd-duet-campaign', campaign: { ...initialCampaign, title: '匯入戰役' } }), initialCampaign);
    expect(loadActiveCampaign(initialCampaign).id).toBe(active.id);
    expect(listCampaigns(initialCampaign).map((entry) => entry.title)).toContain('匯入戰役');
  });
});
