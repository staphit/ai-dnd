import { WifiHigh, WifiSlash } from '@phosphor-icons/react';
import type { AiStatus, Campaign } from '../types';

interface TopbarProps {
  campaign: Campaign;
  status: AiStatus | null;
  demoMode: boolean;
}

export function Topbar({ campaign, status, demoMode }: TopbarProps) {
  const connected = demoMode || status?.connected;
  const label = demoMode ? '示範 DM' : status?.model || 'OpenAI Agent 未設定';

  return (
    <header className="topbar">
      <div>
        <p className="eyebrow">{campaign.chapter}</p>
        <h1>{campaign.title}</h1>
      </div>
      <div className={`connection ${connected ? 'connection-online' : ''}`}>
        <span className="connection-pulse" aria-hidden="true" />
        {connected ? <WifiHigh size={16} /> : <WifiSlash size={16} />}
        <span>{label}</span>
      </div>
    </header>
  );
}
