// The server (Go + SQLite) is the store of record. localStorage keeps only:
// - the last-active campaign id, so a reload reopens the same campaign;
// - read-only access to the old client-side vault, so pre-cutover campaigns
//   can be uploaded once through the import banner.
import type { Campaign } from './types';

const vaultKey = 'dnd-duet-web-v2-vault';
const legacyKey = 'dnd-duet-web-v1';
const activeIdKey = 'dnd-duet-active-id';

// Re-export the server export URL so callers that used the old download helper
// keep a single import site.
export { exportUrl as exportCampaignUrl } from './api';

function parseCampaign(raw: unknown): Campaign | null {
  if (!raw || typeof raw !== 'object') return null;
  const candidate = raw as Campaign;
  return typeof candidate.title === 'string' ? candidate : null;
}

// readLegacyVault returns the campaigns saved by the old localStorage-backed
// client (v2 vault, falling back to the single v1 slot). It never writes:
// the data stays untouched until the player uploads it to the server.
// Pristine placeholders (setup never completed) are skipped.
export function readLegacyVault(): Campaign[] {
  const campaigns: Campaign[] = [];
  try {
    const parsed = JSON.parse(localStorage.getItem(vaultKey) || '') as { version?: number; campaigns?: Record<string, unknown> };
    if (parsed?.version === 2 && parsed.campaigns && typeof parsed.campaigns === 'object') {
      for (const [id, raw] of Object.entries(parsed.campaigns)) {
        const campaign = parseCampaign(raw);
        if (campaign && campaign.setupComplete === true) campaigns.push({ ...campaign, id: campaign.id || id });
      }
    }
  } catch {
    // No v2 vault; try the v1 single-campaign slot below.
  }
  if (campaigns.length === 0) {
    try {
      const single = parseCampaign(JSON.parse(localStorage.getItem(legacyKey) || ''));
      if (single && single.setupComplete === true) campaigns.push(single);
    } catch {
      // No legacy data at all.
    }
  }
  return campaigns;
}

export function getActiveCampaignId(): string {
  try {
    return localStorage.getItem(activeIdKey) || '';
  } catch {
    return '';
  }
}

export function setActiveCampaignId(id: string) {
  try {
    if (id) localStorage.setItem(activeIdKey, id);
    else localStorage.removeItem(activeIdKey);
  } catch {
    // Storage unavailable (private mode); the app still works per-session.
  }
}
