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
import { useI18n, type Language } from '../../i18n';

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
  const { lang, setLang, t } = useI18n();
  return (
    <motion.main key="settings" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="single-page settings-page">
      <div className="page-intro">
        <p className="eyebrow">{t('settings.eyebrow')}</p>
        <h2>{t('settings.title')}</h2>
        <p>{t('settings.intro')}</p>
      </div>
      <section className="settings-row model-selector">
        <div><strong>{t('settings.language')}</strong><span>{t('settings.languageDesc')}</span></div>
        <select value={lang} onChange={(event) => setLang(event.target.value as Language)}>
          <option value="zh">繁體中文</option>
          <option value="en">English</option>
        </select>
      </section>
      <section className="settings-row">
        <div><strong>{t('settings.demo')}</strong><span>{t('settings.demoDesc')}</span></div>
        <button type="button" className={`switch ${demoMode ? 'switch-on' : ''}`} onClick={onToggleDemo}><i /></button>
      </section>
      <section className="settings-row model-selector">
        <div><strong>{t('settings.provider')}</strong><span>{t('settings.providerDesc')}</span></div>
        <select value={activeDmProvider} onChange={(event) => onProviderChange(event.target.value)}>
          {(status?.dmProviders?.length ? status.dmProviders : [{ id: 'codex', label: 'Codex CLI', connected: true }]).map((provider) => (
            <option key={provider.id} value={provider.id}>{provider.label}{'connected' in provider && !provider.connected ? t('settings.notReady') : ''}</option>
          ))}
        </select>
      </section>
      <section className="settings-row model-selector">
        <div><strong>{t('settings.model')}</strong><span>{t('settings.modelDesc')}</span></div>
        <select value={settings.selectedModel || ''} onChange={(event) => onUpdateSettings({ selectedModel: event.target.value })}>
          {(activeDmInfo?.models || status?.models || [{ id: '', label: t('settings.defaultModel') }]).map((model) => <option key={model.id || 'default'} value={model.id}>{model.label}</option>)}
        </select>
      </section>
      <section className="settings-row model-selector">
        <div><strong>{t('settings.effort')}</strong><span>{t('settings.effortDesc')}</span></div>
        <select value={settings.selectedEffort || ''} onChange={(event) => onUpdateSettings({ selectedEffort: event.target.value })}>
          {(activeDmInfo?.efforts || status?.efforts || [{ id: '', label: t('settings.defaultEffort') }]).map((effort) => <option key={effort.id || 'default'} value={effort.id}>{effort.label}</option>)}
        </select>
      </section>
      <section className="settings-row">
        <div><strong>{dmLabel} {t('settings.status')}</strong><span>{(activeDmInfo?.connected ?? status?.connected) ? `${t('settings.ready')}${activeDmInfo?.model || status?.model || '—'}` : activeDmInfo?.message || status?.message || t('settings.checking')}</span></div>
        <ShieldWarning size={22} />
      </section>
      <section className="settings-row model-selector">
        <div><strong>{t('settings.imageBackend')}</strong><span>{t('settings.imageBackendDesc')}</span></div>
        <select value={settings.imageBackend || status?.imageBackend || 'codex'} onChange={(event) => onUpdateSettings({ imageBackend: event.target.value })}>
          {(status?.imageBackends || [{ id: 'codex', label: status?.imageModel || 'Codex $imagegen' }]).map((backend) => <option key={backend.id} value={backend.id}>{backend.label}</option>)}
        </select>
      </section>
      <ToggleRow label={t('settings.autoScene')} description={t('settings.autoSceneDesc')} checked={Boolean(settings.autoSceneImages)} onToggle={() => onUpdateSettings({ autoSceneImages: !settings.autoSceneImages })} />
      <ToggleRow label={t('settings.tts')} description={t('settings.ttsDesc')} checked={Boolean(settings.ttsEnabled)} onToggle={() => onUpdateSettings({ ttsEnabled: !settings.ttsEnabled })} />
      <ToggleRow label={t('settings.statHints')} description={t('settings.statHintsDesc')} checked={settings.showStatHints !== false} onToggle={() => onUpdateSettings({ showStatHints: settings.showStatHints === false })} />
      <section className="settings-row">
        <div><strong>{t('settings.fontScale')}</strong><span>{Math.round((settings.fontScale || 1) * 100)}%</span></div>
        <div className="font-controls">
          <button type="button" onClick={() => onUpdateSettings({ fontScale: Math.max(.85, (settings.fontScale || 1) - .1) })}>A−</button>
          <button type="button" onClick={() => onUpdateSettings({ fontScale: 1 })}>{t('settings.fontReset')}</button>
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
        <div><strong>{t('settings.resetCampaign')}</strong><span>{t('settings.resetCampaignDesc')}</span></div>
        <MagneticButton variant="quiet" onClick={onResetCampaign}>{t('settings.resetCampaign')}</MagneticButton>
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
