import { useRef, useState, type ChangeEvent } from 'react';
import { Copy, DownloadSimple, FolderOpen, Plus, Trash, UploadSimple } from '@phosphor-icons/react';
import type { Campaign, CampaignSummary } from '../types';
import { exportCampaignUrl } from '../campaign-storage';

interface CampaignManagerProps {
  campaign: Campaign;
  campaigns: CampaignSummary[];
  onSwitch: (id: string) => void;
  onDuplicate: () => void;
  onImport: (raw: string) => void;
  onNew: () => void;
  onDelete: (id: string) => void;
}

export function CampaignManager({ campaign, campaigns, onSwitch, onDuplicate, onImport, onNew, onDelete }: CampaignManagerProps) {
  const [selected, setSelected] = useState(campaign.id || '');
  const fileInput = useRef<HTMLInputElement>(null);

  function download() {
    if (!campaign.id) return;
    // The server answers with content-disposition: attachment.
    const anchor = document.createElement('a');
    anchor.href = exportCampaignUrl(campaign.id);
    anchor.download = `${campaign.title.replace(/[\\/:*?"<>|]/g, '-') || 'campaign'}.dnd-duet.json`;
    anchor.click();
  }

  async function upload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    onImport(await file.text());
    event.target.value = '';
  }

  return (
    <section className="campaign-manager">
      <header><div><strong>多戰役資料庫</strong><span>所有戰役都保存在本機伺服器；切換與匯入不會遺失進度。</span></div><FolderOpen size={23} /></header>
      <div className="campaign-picker">
        <select value={selected} onChange={(event) => setSelected(event.target.value)}>{campaigns.map((entry) => <option key={entry.id} value={entry.id}>{entry.title}／回合 {entry.round}／{new Date(entry.updatedAt).toLocaleString('zh-TW')}</option>)}</select>
        <button type="button" onClick={() => onSwitch(selected)} disabled={!selected || selected === campaign.id}><FolderOpen />載入選取戰役</button>
      </div>
      <div className="campaign-actions">
        <button type="button" onClick={onNew}><Plus />建立新戰役</button>
        <button type="button" onClick={onDuplicate} disabled={!campaign.id}><Copy />複製目前戰役</button>
        <button type="button" onClick={download} disabled={!campaign.id}><DownloadSimple />匯出 JSON</button>
        <button type="button" onClick={() => fileInput.current?.click()}><UploadSimple />匯入但不切換</button>
        <button type="button" onClick={() => selected && onDelete(selected)} disabled={!selected}><Trash />刪除選取戰役</button>
        <input ref={fileInput} type="file" accept="application/json,.json" hidden onChange={upload} />
      </div>
    </section>
  );
}
