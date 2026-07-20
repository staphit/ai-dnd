import { useEffect, useRef, useState } from 'react';
import * as api from '../api';
import type { SceneSlotInfo } from '../api';
import {
  errorMessage,
  forgeRequest,
  settingsOf,
  timeLabel,
} from '../app/app-utils';
import type {
  AiStatus,
  Campaign,
  CampaignSettings,
  ForgeSettings,
  PlayerCharacter,
  SceneImage,
} from '../types';

interface UseSceneMediaOptions {
  campaign: Campaign;
  settings: CampaignSettings;
  status: AiStatus | null;
  forgeDefaults?: Omit<ForgeSettings, 'Enabled'>;
  latestNarration?: string;
  updateSettings: (patch: CampaignSettings, options?: { debounce?: boolean }) => void;
  onCampaign: (view: Campaign) => void;
  onError: (message: string) => void;
  onNotice: (message: string) => void;
}

export function useSceneMedia({
  campaign,
  settings,
  status,
  forgeDefaults,
  latestNarration,
  updateSettings,
  onCampaign,
  onError,
  onNotice,
}: UseSceneMediaOptions) {
  const [sceneImage, setSceneImage] = useState<SceneImage | null>(null);
  const [imageLoading, setImageLoading] = useState(false);
  const [imageError, setImageError] = useState('');
  const [pendingSceneSlotId, setPendingSceneSlotId] = useState('');
  const [sceneSlots, setSceneSlots] = useState<SceneSlotInfo[]>([]);
  const [generatingSlotId, setGeneratingSlotId] = useState('');
  const campaignRef = useRef(campaign);

  useEffect(() => {
    campaignRef.current = campaign;
  }, [campaign]);

  const localImages = (settings.imageBackend || status?.imageBackend || '').startsWith('local');
  const canGenerateImages = Boolean(status?.imageBackends?.length)
    || Boolean(status?.connected)
    || localImages;

  async function refreshSceneSlots(id: string) {
    try {
      const { slots } = await api.listSceneSlots(id);
      setSceneSlots(slots);
    } catch {
      // Scene history is optional; leave the current gallery intact.
    }
  }

  function adoptSceneMedia(view: Campaign) {
    const images = settingsOf(view).sceneImages || [];
    setSceneImage(images.length > 0 ? images[images.length - 1] : null);
    setPendingSceneSlotId('');
    setSceneSlots([]);
    if (view.id) void refreshSceneSlots(view.id);
  }

  function resetSceneMedia() {
    setSceneImage(null);
    setPendingSceneSlotId('');
    setSceneSlots([]);
    setGeneratingSlotId('');
    setImageError('');
  }

  function appendSceneImage(image: SceneImage) {
    setSceneImage(image);
    const current = campaignRef.current;
    const sceneImages = [...(settingsOf(current).sceneImages || []), image].slice(-24);
    updateSettings({ sceneImages });
  }

  async function generateImage(
    narrationOverride?: string,
    imagePromptOverride?: string,
    sceneOverride?: string,
    sceneSlotId?: string,
  ) {
    if (!canGenerateImages || imageLoading) return;
    const slotId = sceneSlotId || pendingSceneSlotId;
    const narration = narrationOverride || latestNarration;
    if (!narration && !slotId) {
      setImageError('目前沒有可供繪製的公開 DM 場景敘事。');
      return;
    }
    setImageLoading(true);
    setImageError('');
    setGeneratingSlotId(slotId || '');
    try {
      const response = await fetch('/api/scene-image', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          async: true,
          imageBackend: settings.imageBackend || '',
          campaignId: campaign.id || '',
          campaign: { title: campaign.title, scene: sceneOverride || campaign.scene },
          narration: narration || '',
          imagePrompt: imagePromptOverride ?? campaign.imagePrompt ?? '',
          sceneSlotId: slotId || undefined,
          forge: forgeDefaults ? forgeRequest(settings.forgeSettings) : undefined,
        }),
      });
      const data = await response.json().catch(() => (
        {} as { jobId?: string; url?: string; model?: string; error?: string }
      ));
      if (!response.ok) throw new Error(data.error || '場景插圖生成失敗');

      let url = data.url;
      let model = data.model;
      if (data.jobId) {
        const started = Date.now();
        for (;;) {
          await new Promise((resolve) => window.setTimeout(resolve, 4000));
          if (Date.now() - started > 600000) throw new Error('場景插圖生成逾時，請再試一次。');
          const jobResponse = await fetch(`/api/scene-image/job/${data.jobId}`);
          const job = await jobResponse.json().catch(() => (
            {} as { status?: string; url?: string; model?: string; error?: string }
          ));
          if (!jobResponse.ok) throw new Error(job.error || '查詢圖片生成進度失敗');
          if (job.status === 'done') {
            url = job.url;
            model = job.model;
            break;
          }
          if (job.status === 'error') throw new Error(job.error || '場景插圖生成失敗');
        }
      }
      if (!url) throw new Error('場景插圖生成失敗');
      appendSceneImage({
        url,
        scene: sceneOverride || campaignRef.current.scene,
        createdAt: timeLabel(),
        model: model || status?.imageModel || 'Image',
      });
      if (slotId && slotId === pendingSceneSlotId) setPendingSceneSlotId('');
      if (campaignRef.current.id) void refreshSceneSlots(campaignRef.current.id);
    } catch (caught) {
      setImageError(errorMessage(caught));
    } finally {
      setImageLoading(false);
      setGeneratingSlotId('');
    }
  }

  async function generatePortrait(player: PlayerCharacter, appearance: string) {
    if (!canGenerateImages) return onError('圖片服務尚未連線。');
    if (!campaign.id) return;
    const description = appearance.trim();
    if (!description) return onError('請先輸入角色外觀描述。');
    try {
      const response = await fetch('/api/character-image', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          imageBackend: settings.imageBackend || '',
          name: player.name,
          species: player.species,
          className: player.className,
          background: player.background,
          appearance: description,
        }),
      });
      const data = await response.json().catch(() => ({} as { url?: string; error?: string }));
      if (!response.ok) throw new Error(data.error || '角色圖片生成失敗');
      onCampaign(await api.patchPlayer(campaign.id, player.id, {
        appearance: description,
        portraitUrl: data.url,
      }));
      onNotice(`${player.name}的角色外觀與肖像已更新。`);
    } catch (caught) {
      onError(errorMessage(caught));
    }
  }

  return {
    canGenerateImages,
    generateImage,
    generatePortrait,
    imageError,
    imageLoading,
    generatingSlotId,
    pendingSceneSlotId,
    sceneImage,
    sceneSlots,
    setPendingSceneSlotId,
    setSceneImage,
    refreshSceneSlots,
    adoptSceneMedia,
    resetSceneMedia,
  };
}
