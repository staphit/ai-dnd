import { useEffect, useRef, type Dispatch, type SetStateAction } from 'react';
import * as api from '../api';
import { errorMessage } from '../app/app-utils';
import type { Campaign, CampaignSettings } from '../types';

interface UseCampaignSettingsOptions {
  campaignId?: string;
  setCampaign: Dispatch<SetStateAction<Campaign>>;
  onSyncError: (message: string) => void;
}

interface SettingsBatch {
  id: string;
  timer: number | null;
  patch: CampaignSettings;
}

export function useCampaignSettings({
  campaignId,
  setCampaign,
  onSyncError,
}: UseCampaignSettingsOptions) {
  const campaignIdRef = useRef(campaignId);
  const onSyncErrorRef = useRef(onSyncError);
  const batchRef = useRef<SettingsBatch>({ id: '', timer: null, patch: {} });

  useEffect(() => {
    campaignIdRef.current = campaignId;
    onSyncErrorRef.current = onSyncError;
  }, [campaignId, onSyncError]);

  function flushSettings() {
    const batch = batchRef.current;
    if (batch.timer !== null) {
      window.clearTimeout(batch.timer);
      batch.timer = null;
    }
    const { id, patch } = batch;
    batch.patch = {};
    if (!id || Object.keys(patch).length === 0) return;
    void api.patchSettings(id, patch as Record<string, unknown>).catch((caught) => {
      onSyncErrorRef.current(`設定尚未同步到伺服器：${errorMessage(caught)}`);
    });
  }

  useEffect(() => () => flushSettings(), []);

  function updateSettings(patch: CampaignSettings, options: { debounce?: boolean } = {}) {
    setCampaign((current) => ({
      ...current,
      settings: { ...(current.settings || {}), ...patch },
    }));

    const id = campaignIdRef.current;
    if (!id) return;
    const batch = batchRef.current;
    if (batch.id !== id) flushSettings();
    batch.id = id;
    batch.patch = { ...batch.patch, ...patch };
    if (batch.timer !== null) window.clearTimeout(batch.timer);
    if (options.debounce) {
      batch.timer = window.setTimeout(() => {
        batch.timer = null;
        flushSettings();
      }, 600);
      return;
    }
    flushSettings();
  }

  return updateSettings;
}
