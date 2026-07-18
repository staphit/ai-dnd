import { motion } from 'framer-motion';
import { ShieldWarning } from '@phosphor-icons/react';
import type {
  AiStatus,
  Campaign,
  CampaignSettings,
  CampaignSummary,
  ForgeSettings,
} from '../../types';
import { CampaignManager } from '../CampaignManager';
import { MagneticButton } from '../MagneticButton';

type DmProviderInfo = NonNullable<AiStatus['dmProviders']>[number];

interface SettingsPageProps {
  campaign: Campaign;
  campaigns: CampaignSummary[];
  settings: CampaignSettings;
  status: AiStatus | null;
  demoMode: boolean;
  activeDmProvider: string;
  activeDmInfo?: DmProviderInfo;
  dmLabel: string;
  forgeDefaults?: Omit<ForgeSettings, 'Enabled'>;
  forgeSettings?: ForgeSettings;
  onToggleDemo: () => void;
  onProviderChange: (provider: string) => void;
  onUpdateSettings: (patch: CampaignSettings, options?: { debounce?: boolean }) => void;
  onUpdateForgeSettings: (patch: Partial<ForgeSettings>) => void;
  onSwitchCampaign: (id: string) => void;
  onNewCampaign: () => void;
  onDuplicateCampaign: () => void;
  onImportCampaign: (raw: string) => void;
  onDeleteCampaign: (id: string) => void;
  onResetCampaign: () => void;
}

export function SettingsPage({
  campaign,
  campaigns,
  settings,
  status,
  demoMode,
  activeDmProvider,
  activeDmInfo,
  dmLabel,
  forgeDefaults,
  forgeSettings,
  onToggleDemo,
  onProviderChange,
  onUpdateSettings,
  onUpdateForgeSettings,
  onSwitchCampaign,
  onNewCampaign,
  onDuplicateCampaign,
  onImportCampaign,
  onDeleteCampaign,
  onResetCampaign,
}: SettingsPageProps) {
  return (
    <motion.main key="settings" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page settings-page">
      <div className="page-intro">
        <p className="eyebrow">戰役設定</p>
        <h2>地城主與戰役</h2>
        <p>設定會即時保存在伺服器上的這個戰役；匯入預設不切換。</p>
      </div>
      <section className="settings-row">
        <div><strong>示範 DM</strong><span>完全不呼叫模型。</span></div>
        <button type="button" className={`switch ${demoMode ? 'switch-on' : ''}`} onClick={onToggleDemo}><i /></button>
      </section>
      <section className="settings-row model-selector">
        <div><strong>DM 資料源</strong><span>Codex（ChatGPT 登入）或 Grok（`grok login`／XAI_API_KEY）。切換後請重新連線該故事。</span></div>
        <select value={activeDmProvider} onChange={(event) => onProviderChange(event.target.value)}>
          {(status?.dmProviders?.length ? status.dmProviders : [{ id: 'codex', label: 'Codex CLI', connected: true }]).map((provider) => (
            <option key={provider.id} value={provider.id}>{provider.label}{'connected' in provider && !provider.connected ? '（未就緒）' : ''}</option>
          ))}
        </select>
      </section>
      <section className="settings-row model-selector">
        <div><strong>模型</strong><span>只影響之後的新 DM 請求；目前進度與既有訊息不會改變。</span></div>
        <select value={settings.selectedModel || ''} onChange={(event) => onUpdateSettings({ selectedModel: event.target.value })}>
          {(activeDmInfo?.models || status?.models || [{ id: '', label: '預設模型' }]).map((model) => <option key={model.id || 'default'} value={model.id}>{model.label}</option>)}
        </select>
      </section>
      <section className="settings-row model-selector">
        <div><strong>推理強度（effort）</strong><span>越高越深思但回應越慢；Grok 可能僅有預設。</span></div>
        <select value={settings.selectedEffort || ''} onChange={(event) => onUpdateSettings({ selectedEffort: event.target.value })}>
          {(activeDmInfo?.efforts || status?.efforts || [{ id: '', label: '預設推理強度' }]).map((effort) => <option key={effort.id || 'default'} value={effort.id}>{effort.label}</option>)}
        </select>
      </section>
      <section className="settings-row">
        <div><strong>{dmLabel} 狀態</strong><span>{(activeDmInfo?.connected ?? status?.connected) ? `已就緒／${activeDmInfo?.model || status?.model || '—'}` : activeDmInfo?.message || status?.message || '正在檢查'}</span></div>
        <ShieldWarning size={22} />
      </section>
      <section className="settings-row model-selector">
        <div><strong>圖片生成引擎</strong><span>場景圖與角色肖像使用的後端；本地選項需先啟動 SD Forge（--api）。</span></div>
        <select value={settings.imageBackend || status?.imageBackend || 'codex'} onChange={(event) => onUpdateSettings({ imageBackend: event.target.value })}>
          {(status?.imageBackends || [{ id: 'codex', label: status?.imageModel || 'Codex $imagegen' }]).map((backend) => <option key={backend.id} value={backend.id}>{backend.label}</option>)}
        </select>
      </section>
      <ToggleRow label="每回合自動生成場景圖" description="開啟後，每次 DM 完成公開敘事便自動生成並加入圖庫。" checked={Boolean(settings.autoSceneImages)} onToggle={() => onUpdateSettings({ autoSceneImages: !settings.autoSceneImages })} />
      <ToggleRow label="語音朗讀 DM 敘事" description="使用本地 GPT-SoVITS 朗讀每回合公開敘事；需先啟動 scripts/sovits.sh 並設定聲線。" checked={Boolean(settings.ttsEnabled)} onToggle={() => onUpdateSettings({ ttsEnabled: !settings.ttsEnabled })} />
      <ToggleRow label="角色屬性懸浮說明" description="滑鼠停留或用鍵盤聚焦屬性時，顯示規則用途與計算方式。" checked={settings.showStatHints !== false} onToggle={() => onUpdateSettings({ showStatHints: settings.showStatHints === false })} />
      <section className="settings-row">
        <div><strong>介面字型大小</strong><span>{Math.round((settings.fontScale || 1) * 100)}%</span></div>
        <div className="font-controls">
          <button type="button" onClick={() => onUpdateSettings({ fontScale: Math.max(.85, (settings.fontScale || 1) - .1) })}>A−</button>
          <button type="button" onClick={() => onUpdateSettings({ fontScale: 1 })}>重設</button>
          <button type="button" onClick={() => onUpdateSettings({ fontScale: Math.min(1.25, (settings.fontScale || 1) + .1) })}>A＋</button>
        </div>
      </section>
      <CampaignManager
        campaign={campaign}
        campaigns={campaigns}
        onSwitch={onSwitchCampaign}
        onNew={onNewCampaign}
        onDuplicate={onDuplicateCampaign}
        onImport={onImportCampaign}
        onDelete={onDeleteCampaign}
      />
      <section className="settings-danger">
        <div><strong>重設目前戰役</strong><span>刪除伺服器上這個戰役的所有進度並回到開團設定。</span></div>
        <MagneticButton variant="quiet" onClick={onResetCampaign}>重設目前戰役</MagneticButton>
      </section>
      {forgeDefaults && forgeSettings && (
        <>
          <ToggleRow label="自訂 Forge 場景圖參數" description="僅本地 Forge 使用；關閉時完全沿用伺服器 preset。開啟後 negative prompt 會強制使用 CFG > 1。" checked={forgeSettings.Enabled} onToggle={() => onUpdateForgeSettings({ Enabled: !forgeSettings.Enabled })} />
          {forgeSettings.Enabled && <ForgeSettingsForm settings={forgeSettings} onChange={onUpdateForgeSettings} />}
        </>
      )}
    </motion.main>
  );
}

function ToggleRow({ label, description, checked, onToggle }: { label: string; description: string; checked: boolean; onToggle: () => void }) {
  return (
    <section className="settings-row">
      <div><strong>{label}</strong><span>{description}</span></div>
      <button type="button" role="switch" aria-checked={checked} aria-label={label} className={`switch ${checked ? 'switch-on' : ''}`} onClick={onToggle}><i /></button>
    </section>
  );
}

function ForgeSettingsForm({ settings, onChange }: { settings: ForgeSettings; onChange: (patch: Partial<ForgeSettings>) => void }) {
  return (
    <section className="forge-settings" aria-label="Forge 場景圖參數">
      <label className="forge-prompt"><span>Positive prompt</span><textarea rows={4} value={settings.PositivePrompt} placeholder="留空時使用 DM 產生的場景提示詞與寫實場景前後綴" onChange={(event) => onChange({ PositivePrompt: event.target.value })} /></label>
      <label className="forge-prompt"><span>Negative prompt</span><textarea rows={4} value={settings.NegativePrompt} onChange={(event) => onChange({ NegativePrompt: event.target.value })} /></label>
      <div className="forge-grid">
        <NumberField label="Steps" min={1} max={150} step={1} value={settings.Steps} onChange={(Steps) => onChange({ Steps })} />
        <NumberField label="CFG scale" min={1.1} max={30} step={0.1} value={settings.CFGScale} onChange={(CFGScale) => onChange({ CFGScale })} />
        <NumberField label="Seed" min={-1} max={2147483647} step={1} value={settings.Seed} onChange={(Seed) => onChange({ Seed })} />
        <label><span>Sampler</span><input type="text" value={settings.Sampler} onChange={(event) => onChange({ Sampler: event.target.value })} /></label>
        <label><span>Scheduler</span><input type="text" value={settings.Scheduler} onChange={(event) => onChange({ Scheduler: event.target.value })} /></label>
        <NumberField label="寬度" min={256} max={2048} step={8} value={settings.Width} onChange={(Width) => onChange({ Width })} />
        <NumberField label="高度" min={256} max={2048} step={8} value={settings.Height} onChange={(Height) => onChange({ Height })} />
      </div>
      <p>Seed 設為 -1 會每次隨機；固定 seed 才能與 Forge WebUI 重現相同構圖。Positive 留空時，系統仍使用本回合 DM prompt。</p>
    </section>
  );
}

function NumberField({ label, min, max, step, value, onChange }: { label: string; min: number; max: number; step: number; value: number; onChange: (value: number) => void }) {
  return <label><span>{label}</span><input type="number" min={min} max={max} step={step} value={value} onChange={(event) => onChange(Number(event.target.value))} /></label>;
}
