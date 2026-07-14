import type { Campaign, CampaignSummary } from './types';

const vaultKey = 'dnd-duet-web-v2-vault';
const legacyKey = 'dnd-duet-web-v1';

interface CampaignVault {
  version: 2;
  activeId: string;
  campaigns: Record<string, Campaign>;
}

function campaignId() {
  return `campaign-${Date.now()}-${crypto.randomUUID()}`;
}

function stamp(campaign: Campaign, id = campaign.id || campaignId()): Campaign {
  return {
    ...campaign,
    schemaVersion: 2,
    id,
    updatedAt: campaign.updatedAt || new Date().toISOString(),
    story: Array.isArray(campaign.story) ? campaign.story : [],
    pending: campaign.pending || {},
    players: Array.isArray(campaign.players) ? campaign.players : [],
  };
}

function parseCampaign(raw: string | null): Campaign | null {
  if (!raw) return null;
  try {
    const value = JSON.parse(raw) as Campaign;
    return value && typeof value === 'object' && typeof value.title === 'string' ? value : null;
  } catch {
    return null;
  }
}

function readVault(fallback: Campaign): CampaignVault {
  try {
    const parsed = JSON.parse(localStorage.getItem(vaultKey) || '') as CampaignVault;
    if (parsed?.version === 2 && parsed.activeId && parsed.campaigns?.[parsed.activeId]) return parsed;
  } catch {
    // Fall through to the non-destructive legacy migration below.
  }
  const legacy = parseCampaign(localStorage.getItem(legacyKey));
  const first = stamp(legacy || fallback);
  const vault: CampaignVault = { version: 2, activeId: first.id!, campaigns: { [first.id!]: first } };
  localStorage.setItem(vaultKey, JSON.stringify(vault));
  return vault;
}

function writeVault(vault: CampaignVault) {
  localStorage.setItem(vaultKey, JSON.stringify(vault));
}

export function loadActiveCampaign(fallback: Campaign): Campaign {
  const vault = readVault(fallback);
  return stamp(vault.campaigns[vault.activeId], vault.activeId);
}

export function saveActiveCampaign(campaign: Campaign, fallback: Campaign) {
  const vault = readVault(fallback);
  const id = campaign.id || vault.activeId;
  const saved = stamp({ ...campaign, updatedAt: new Date().toISOString() }, id);
  writeVault({ ...vault, activeId: id, campaigns: { ...vault.campaigns, [id]: saved } });
}

export function listCampaigns(fallback: Campaign): CampaignSummary[] {
  const vault = readVault(fallback);
  return Object.values(vault.campaigns)
    .map((campaign) => ({
      id: campaign.id!,
      title: campaign.title,
      updatedAt: campaign.updatedAt || new Date(0).toISOString(),
      round: campaign.round || 1,
    }))
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
}

export function activateCampaign(id: string, fallback: Campaign): Campaign | null {
  const vault = readVault(fallback);
  const campaign = vault.campaigns[id];
  if (!campaign) return null;
  writeVault({ ...vault, activeId: id });
  return stamp(campaign, id);
}

export function addCampaign(campaign: Campaign, fallback: Campaign, activate = false): Campaign {
  const vault = readVault(fallback);
  const id = campaignId();
  const saved = stamp({ ...campaign, updatedAt: new Date().toISOString() }, id);
  writeVault({
    ...vault,
    activeId: activate ? id : vault.activeId,
    campaigns: { ...vault.campaigns, [id]: saved },
  });
  return saved;
}

export function exportCampaign(campaign: Campaign): string {
  return JSON.stringify({ format: 'dnd-duet-campaign', version: 2, exportedAt: new Date().toISOString(), campaign }, null, 2);
}

export function importCampaign(raw: string, fallback: Campaign): Campaign {
  const parsed: unknown = JSON.parse(raw);
  const candidate = parsed && typeof parsed === 'object' && 'campaign' in parsed
    ? (parsed as { campaign?: Campaign }).campaign
    : parsed as Campaign;
  if (!candidate || typeof candidate.title !== 'string' || !Array.isArray(candidate.players) || !Array.isArray(candidate.story)) {
    throw new Error('這不是有效的 D&D Duet 戰役檔案。');
  }
  return addCampaign({ ...candidate, id: undefined }, fallback, false);
}

export function duplicateCampaign(campaign: Campaign, fallback: Campaign): Campaign {
  return addCampaign({ ...campaign, title: `${campaign.title}（副本）`, pending: {} }, fallback, false);
}
