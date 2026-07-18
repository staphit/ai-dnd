import { useEffect, useState, type Dispatch, type SetStateAction } from 'react';
import * as api from '../api';
import { ApiError } from '../api';
import { errorMessage } from '../app/app-utils';
import { getActiveCampaignId, readLegacyVault, setActiveCampaignId } from '../campaign-storage';
import { initialCampaign } from '../data';
import type { Campaign, CampaignSummary, Page } from '../types';

interface UseCampaignLibraryOptions {
  campaign: Campaign;
  campaigns: CampaignSummary[];
  setCampaignState: Dispatch<SetStateAction<Campaign>>;
  setCampaigns: Dispatch<SetStateAction<CampaignSummary[]>>;
  adoptCampaign: (campaign: Campaign) => void;
  resetSceneMedia: () => void;
  setPage: Dispatch<SetStateAction<Page>>;
  onError: (message: string) => void;
  onNotice: (message: string) => void;
}

export function useCampaignLibrary({
  campaign,
  campaigns,
  setCampaignState,
  setCampaigns,
  adoptCampaign,
  resetSceneMedia,
  setPage,
  onError,
  onNotice,
}: UseCampaignLibraryOptions) {
  const [booting, setBooting] = useState(true);
  const [legacyCampaigns, setLegacyCampaigns] = useState<Campaign[]>([]);
  const [legacyImporting, setLegacyImporting] = useState(false);

  async function refreshCampaigns() {
    try {
      const { campaigns: list } = await api.listCampaigns();
      setCampaigns(list);
    } catch {
      // The active campaign remains usable if refreshing the library fails.
    }
  }

  async function bootstrap() {
    const lastId = getActiveCampaignId();
    if (lastId) {
      try {
        adoptCampaign(await api.getCampaign(lastId));
        setPage('table');
        void refreshCampaigns();
        return;
      } catch (caught) {
        if (!(caught instanceof ApiError && caught.status === 404)) throw caught;
      }
    }

    const { campaigns: list } = await api.listCampaigns();
    setCampaigns(list);
    if (list.length > 0) {
      adoptCampaign(await api.getCampaign(list[0].id));
      setPage('table');
      return;
    }
    setLegacyCampaigns(readLegacyVault());
    setCampaignState(structuredClone(initialCampaign));
    resetSceneMedia();
  }

  useEffect(() => {
    let cancelled = false;
    void bootstrap()
      .catch((caught) => {
        if (!cancelled) onError(`無法連線本機伺服器：${errorMessage(caught)}`);
      })
      .finally(() => {
        if (!cancelled) setBooting(false);
      });
    return () => { cancelled = true; };
    // Bootstrap is intentionally run only once; campaign changes are explicit.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function resetCampaign() {
    if (!campaign.id) return;
    if (!window.confirm('重設會刪除伺服器上這個戰役的所有進度，並回到開團設定。確定繼續？')) return;
    const id = campaign.id;
    try {
      await api.deleteCampaign(id);
    } catch (caught) {
      onError(errorMessage(caught));
      return;
    }
    setActiveCampaignId('');
    setCampaigns((current) => current.filter((entry) => entry.id !== id));
    setCampaignState(structuredClone(initialCampaign));
    resetSceneMedia();
    onError('');
    setPage('table');
  }

  async function switchCampaign(id: string) {
    try {
      const next = await api.getCampaign(id);
      adoptCampaign(next);
      setPage('table');
      onNotice(`已載入「${next.title}」。`);
    } catch (caught) {
      onError(errorMessage(caught));
    }
  }

  function newCampaign() {
    setCampaignState(structuredClone(initialCampaign));
    resetSceneMedia();
    setPage('table');
  }

  async function duplicateCurrentCampaign() {
    if (!campaign.id) return;
    try {
      const response = await fetch(api.exportUrl(campaign.id));
      const envelope = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(envelope.error || '匯出戰役失敗');
      const source = (envelope.campaign || envelope) as Record<string, unknown>;
      const copyTitle = `${String(source.title || campaign.title)}（副本）`;
      const copy = { ...source, id: undefined, title: copyTitle, pending: {} };
      await api.importCampaign(JSON.stringify(copy), false);
      await refreshCampaigns();
      onNotice(`已建立「${copyTitle}」，目前戰役未切換。`);
    } catch (caught) {
      onError(errorMessage(caught));
    }
  }

  async function importFile(raw: string) {
    try {
      const imported = await api.importCampaign(raw, false);
      await refreshCampaigns();
      onNotice(`已匯入「${imported.title}」，目前戰役未切換。`);
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 409) {
        if (!window.confirm('伺服器上已有同 ID 的戰役。要以匯入檔覆蓋伺服器上的版本嗎？')) return;
        try {
          const imported = await api.importCampaign(raw, true);
          await refreshCampaigns();
          if (imported.id && imported.id === campaign.id) adoptCampaign(imported);
          onNotice(`已匯入並覆蓋「${imported.title}」。`);
        } catch (again) {
          onError(errorMessage(again));
        }
        return;
      }
      onError(errorMessage(caught));
    }
  }

  async function removeCampaign(id: string) {
    const target = campaigns.find((entry) => entry.id === id);
    if (!window.confirm(`確定要刪除「${target?.title || id}」嗎？伺服器上的進度將永久移除。`)) return;
    try {
      await api.deleteCampaign(id);
      setCampaigns((current) => current.filter((entry) => entry.id !== id));
      if (campaign.id === id) {
        setActiveCampaignId('');
        try {
          await bootstrap();
        } catch {
          setCampaignState(structuredClone(initialCampaign));
          resetSceneMedia();
        }
        setPage('table');
      }
      onNotice('戰役已刪除。');
    } catch (caught) {
      onError(errorMessage(caught));
    }
  }

  async function importLegacyVault() {
    if (legacyImporting) return;
    setLegacyImporting(true);
    onError('');
    let lastView: Campaign | null = null;
    const failures: string[] = [];
    for (const legacy of legacyCampaigns) {
      try {
        lastView = await api.importCampaign(JSON.stringify(legacy), false);
      } catch (caught) {
        if (caught instanceof ApiError && caught.status === 409) {
          if (window.confirm(`伺服器上已有「${legacy.title}」（相同 ID）。要以本機版本覆蓋嗎？`)) {
            try {
              lastView = await api.importCampaign(JSON.stringify(legacy), true);
            } catch (again) {
              failures.push(`${legacy.title}：${errorMessage(again)}`);
            }
          }
          continue;
        }
        failures.push(`${legacy.title}：${errorMessage(caught)}`);
      }
    }
    setLegacyImporting(false);
    setLegacyCampaigns([]);
    await refreshCampaigns();
    if (lastView) {
      adoptCampaign(lastView);
      setPage('table');
    }
    if (failures.length > 0) onError(`部分戰役匯入失敗：${failures.join('；')}`);
    else if (lastView) onNotice('本機戰役已上傳到伺服器，之後的進度都會保存在伺服器上。');
  }

  function completeSetup(view: Campaign) {
    adoptCampaign(view);
    void refreshCampaigns();
    setPage('table');
    onError('');
  }

  return {
    booting,
    legacyCampaigns,
    legacyImporting,
    setLegacyCampaigns,
    bootstrap,
    completeSetup,
    duplicateCurrentCampaign,
    importFile,
    importLegacyVault,
    newCampaign,
    removeCampaign,
    resetCampaign,
    switchCampaign,
  };
}
