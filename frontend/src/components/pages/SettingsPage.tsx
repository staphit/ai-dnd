import { motion } from 'framer-motion';
import { ShieldWarning } from '@phosphor-icons/react';
import type {
  AiStatus,
  Campaign,
  CampaignSettings,
  CampaignSummary,
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
  onToggleDemo: () => void;
  onProviderChange: (provider: string) => void;
  onUpdateSettings: (patch: CampaignSettings, options?: { debounce?: boolean }) => void;
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
  onToggleDemo,
  onProviderChange,
  onUpdateSettings,
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
      <section className="settings-row">
        <div><strong>{t('settings.imageBackend')}</strong><span>{t('settings.imageBackendDesc')}</span></div>
        <span className="settings-value">{status?.imageModel || status?.imageBackends?.[0]?.label || 'Codex $imagegen（GPT）'}</span>
      </section>
      <ToggleRow label={t('settings.autoScene')} description={t('settings.autoSceneDesc')} checked={Boolean(settings.autoSceneImages)} onToggle={() => onUpdateSettings({ autoSceneImages: !settings.autoSceneImages })} />
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
