export interface ChatEntry {
  id: string;
  speaker: 'dm' | 'player1' | 'player2' | 'system';
  text: string;
  createdAt: string;
}

export interface CampaignState {
  id: string;
  title: string;
  createdAt: string;
  updatedAt: string;
  scene: string;
  round: number;
  pendingActions: { player1?: string; player2?: string };
  history: ChatEntry[];
}
