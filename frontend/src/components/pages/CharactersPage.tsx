import { lazy, Suspense } from 'react';
import { motion } from 'framer-motion';
import * as api from '../../api';
import { errorMessage } from '../../app/app-utils';
import type { Campaign, PlayerCharacter } from '../../types';

const CharacterManager = lazy(() => import('../CharacterManager').then((module) => ({
  default: module.CharacterManager,
})));

interface CharactersPageProps {
  campaign: Campaign;
  showStatHints: boolean;
  onCampaign: (view: Campaign) => void;
  onError: (message: string) => void;
  onNotice: (message: string) => void;
  onGeneratePortrait: (player: PlayerCharacter, appearance: string) => Promise<void>;
}

export function CharactersPage({
  campaign,
  showStatHints,
  onCampaign,
  onError,
  onNotice,
  onGeneratePortrait,
}: CharactersPageProps) {
  const id = campaign.id;
  return (
    <motion.div key="characters" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
      <Suspense fallback={<main className="single-page lazy-page-loading" role="status"><span>正在載入角色成長資料…</span></main>}>
        <CharacterManager
          players={campaign.players}
          xpProgress={campaign.xpProgress}
          showStatHints={showStatHints}
          onLevelUp={(playerId, className) => {
            if (!id) return;
            api.levelUp(id, playerId, className).then(onCampaign).catch((caught) => onError(errorMessage(caught)));
          }}
          onSpendAbilityPoint={(playerId, ability) => {
            if (!id) return;
            api.spendAbilityPoint(id, playerId, ability).then(onCampaign).catch((caught) => onError(errorMessage(caught)));
          }}
          onSetPreparedSpells={(playerId, spellIds) => {
            if (!id) return;
            api.setPreparedSpells(id, playerId, spellIds).then(onCampaign).catch((caught) => onError(errorMessage(caught)));
          }}
          onSaveProfile={(playerId, profile) => {
            if (!id) return;
            api.patchPlayer(id, playerId, profile).then((view) => {
              onCampaign(view);
              onNotice('角色配置已儲存。');
            }).catch((caught) => onError(errorMessage(caught)));
          }}
          onGeneratePortrait={onGeneratePortrait}
        />
      </Suspense>
    </motion.div>
  );
}
