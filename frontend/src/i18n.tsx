// UI language layer: a context plus a flat key dictionary. The toggle covers
// app chrome and the DM narration directive; game data (class/skill/spell
// names, scripted-module prose, demo DM) stays Traditional Chinese because it
// is authored server-side.
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';

export type Language = 'zh' | 'en';

const STORAGE_KEY = 'dnd-duet-language';

const messages = {
  'nav.table': { zh: '遊戲桌', en: 'Table' },
  'nav.characters': { zh: '成長', en: 'Growth' },
  'nav.journal': { zh: '戰役紀錄', en: 'Journal' },
  'nav.settings': { zh: '設定', en: 'Settings' },
  'nav.aria': { zh: '主要功能', en: 'Main navigation' },
  'script.progress': { zh: '劇本進度', en: 'Script progress' },
  'script.nodes': { zh: '節點', en: 'nodes' },
  'script.fate': { zh: '命運傾向', en: 'Fate' },
  'script.align.light': { zh: '光明', en: 'Light' },
  'script.align.dark': { zh: '幽暗', en: 'Dark' },
  'script.align.balanced': { zh: '平衡', en: 'Balanced' },
  'script.ending.good': { zh: '光明結局', en: 'Good ending' },
  'script.ending.neutral': { zh: '蒼灰結局', en: 'Neutral ending' },
  'script.ending.bad': { zh: '沉沒結局', en: 'Bad ending' },
  'topbar.demoDm': { zh: '示範 DM', en: 'Demo DM' },
  'topbar.noAgent': { zh: 'OpenAI Agent 未設定', en: 'No model configured' },
  'settings.eyebrow': { zh: '戰役設定', en: 'Campaign settings' },
  'settings.title': { zh: '地城主與戰役', en: 'Dungeon Master & campaign' },
  'settings.intro': { zh: '設定會即時保存在伺服器上的這個戰役；匯入預設不切換。', en: 'Settings save to this campaign on the server immediately; importing does not switch campaigns.' },
  'settings.language': { zh: '介面與 DM 語言', en: 'UI & DM language' },
  'settings.languageDesc': { zh: '介面文字與之後的 DM 敘事語言。職業／技能等規則名詞與劇本模式、示範 DM 內文維持中文。', en: 'Applies to app chrome and future DM narration. Rule terms (classes, skills), scripted modules and the demo DM stay Chinese.' },
  'settings.demo': { zh: '示範 DM', en: 'Demo DM' },
  'settings.demoDesc': { zh: '完全不呼叫模型。', en: 'Never calls a model.' },
  'settings.provider': { zh: 'DM 資料源', en: 'DM provider' },
  'settings.providerDesc': { zh: 'Codex（ChatGPT 登入）或 Grok（`grok login`／XAI_API_KEY）。切換後請重新連線該故事。', en: 'Codex (ChatGPT login) or Grok (`grok login` / XAI_API_KEY). Reconnect the story after switching.' },
  'settings.notReady': { zh: '（未就緒）', en: ' (not ready)' },
  'settings.model': { zh: '模型', en: 'Model' },
  'settings.modelDesc': { zh: '只影響之後的新 DM 請求；目前進度與既有訊息不會改變。', en: 'Affects future DM requests only; existing progress and messages stay unchanged.' },
  'settings.defaultModel': { zh: '預設模型', en: 'Default model' },
  'settings.effort': { zh: '推理強度（effort）', en: 'Reasoning effort' },
  'settings.effortDesc': { zh: '越高越深思但回應越慢；Grok 可能僅有預設。', en: 'Higher thinks deeper but responds slower; Grok may only offer the default.' },
  'settings.defaultEffort': { zh: '預設推理強度', en: 'Default effort' },
  'settings.status': { zh: '狀態', en: 'status' },
  'settings.ready': { zh: '已就緒／', en: 'Ready / ' },
  'settings.checking': { zh: '正在檢查', en: 'Checking' },
  'settings.imageBackend': { zh: '圖片生成引擎', en: 'Image engine' },
  'settings.imageBackendDesc': { zh: '場景圖與角色肖像使用的後端；本地選項需先啟動 SD Forge（--api）。', en: 'Backend for scene art and portraits; the local option needs SD Forge running with --api.' },
  'settings.autoScene': { zh: '每回合自動生成場景圖', en: 'Auto-generate scene art each turn' },
  'settings.autoSceneDesc': { zh: '開啟後，每次 DM 完成公開敘事便自動生成並加入圖庫。', en: 'When on, every DM narration renders a scene image into the gallery automatically.' },
  'settings.statHints': { zh: '角色屬性懸浮說明', en: 'Stat hover hints' },
  'settings.statHintsDesc': { zh: '滑鼠停留或用鍵盤聚焦屬性時，顯示規則用途與計算方式。', en: 'Hovering or keyboard-focusing a stat shows what it does and how it is computed.' },
  'settings.fontScale': { zh: '介面字型大小', en: 'UI font size' },
  'settings.fontReset': { zh: '重設', en: 'Reset' },
  'settings.resetCampaign': { zh: '重設目前戰役', en: 'Reset current campaign' },
  'settings.resetCampaignDesc': { zh: '刪除伺服器上這個戰役的所有進度並回到開團設定。', en: 'Deletes all server-side progress for this campaign and returns to setup.' },
} satisfies Record<string, Record<Language, string>>;

export type MessageKey = keyof typeof messages;

interface I18nValue {
  lang: Language;
  setLang: (lang: Language) => void;
  t: (key: MessageKey) => string;
}

const I18nContext = createContext<I18nValue>({
  lang: 'zh',
  setLang: () => {},
  t: (key) => messages[key].zh,
});

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Language>(() => {
    try {
      return window.localStorage.getItem(STORAGE_KEY) === 'en' ? 'en' : 'zh';
    } catch {
      return 'zh';
    }
  });

  useEffect(() => {
    document.documentElement.lang = lang === 'en' ? 'en' : 'zh-Hant';
  }, [lang]);

  function setLang(next: Language) {
    setLangState(next);
    try {
      window.localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Private-mode storage failures just lose persistence, not the toggle.
    }
  }

  const t = (key: MessageKey) => messages[key][lang];
  return <I18nContext.Provider value={{ lang, setLang, t }}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  return useContext(I18nContext);
}
