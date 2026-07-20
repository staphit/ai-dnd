import { WifiHigh, WifiSlash } from '@phosphor-icons/react';
import type { AiStatus, Campaign } from '../types';
import { useI18n } from '../i18n';

interface TopbarProps {
  campaign: Campaign;
  status: AiStatus | null;
  demoMode: boolean;
}

export function Topbar({ campaign, status, demoMode }: TopbarProps) {
  const { t } = useI18n();
  const connected = demoMode || status?.connected;
  const label = demoMode ? t('topbar.demoDm') : status?.model || t('topbar.noAgent');

  return (
    <header className="topbar">
      <div>
        <p className="eyebrow">{campaign.chapter}</p>
        <h1>{campaign.title}</h1>
      </div>
      <div className="topbar-tools">
        <div className={`connection ${connected ? 'connection-online' : ''}`}>
          <span className="connection-pulse" aria-hidden="true" />
          {connected ? <WifiHigh size={16} /> : <WifiSlash size={16} />}
          <span>{label}</span>
        </div>
      </div>
    </header>
  );
}
