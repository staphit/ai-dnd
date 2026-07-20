import { BookOpen, Compass, GearSix, Scroll, TrendUp } from '@phosphor-icons/react';
import type { Page, ScriptProgress } from '../types';
import { useI18n, type MessageKey } from '../i18n';

const items: Array<{ id: Page; labelKey: MessageKey; icon: typeof Compass }> = [
  { id: 'table', labelKey: 'nav.table', icon: Compass },
  { id: 'characters', labelKey: 'nav.characters', icon: TrendUp },
  { id: 'journal', labelKey: 'nav.journal', icon: BookOpen },
  { id: 'settings', labelKey: 'nav.settings', icon: GearSix },
];

interface SidebarProps { page: Page; setPage: (page: Page) => void; script?: ScriptProgress }

function alignmentKey(alignment: number): MessageKey {
  if (alignment >= 2) return 'script.align.light';
  if (alignment <= -2) return 'script.align.dark';
  return 'script.align.balanced';
}

export function Sidebar({ page, setPage, script }: SidebarProps) {
  const { t } = useI18n();
  return (
    <aside className="sidebar">
      <div className="brand-mark" aria-label="灰燼王冠"><Scroll size={22} weight="regular" /></div>
      <nav aria-label={t('nav.aria')} className="sidebar-nav">{items.map(({ id, labelKey, icon: Icon }) => <button key={id} type="button" onClick={() => setPage(id)} className={`nav-item ${page === id ? 'nav-item-active' : ''}`} aria-current={page === id ? 'page' : undefined}><Icon size={20} weight={page === id ? 'fill' : 'regular'} /><span>{t(labelKey)}</span></button>)}</nav>
      {script && (
        <div className="sidebar-script" aria-label={t('script.progress')} title={script.nodeTitle}>
          <span className={`script-stage ${script.ended ? (script.ending === 'good' ? 'script-stage-good' : script.ending === 'neutral' ? 'script-stage-neutral' : 'script-stage-bad') : ''}`}>
            {script.ended ? (script.ending === 'good' ? t('script.ending.good') : script.ending === 'neutral' ? t('script.ending.neutral') : t('script.ending.bad')) : script.stage}
          </span>
          <strong className="script-node">{script.nodeTitle}</strong>
          <span className="script-progress">{Math.min(script.visitedCount + 1, script.totalNodes)}/{script.totalNodes} {t('script.nodes')}</span>
          <span className="script-alignment">{t('script.fate')}・{t(alignmentKey(script.alignment))}</span>
        </div>
      )}
      <div className="sidebar-chapter">I</div>
    </aside>
  );
}
