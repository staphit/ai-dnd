import { BookOpen, Compass, GearSix, Scroll, TrendUp } from '@phosphor-icons/react';
import type { Page, ScriptProgress } from '../types';

const items = [
  { id: 'table' as const, label: '遊戲桌', icon: Compass },
  { id: 'characters' as const, label: '成長', icon: TrendUp },
  { id: 'journal' as const, label: '戰役紀錄', icon: BookOpen },
  { id: 'settings' as const, label: '設定', icon: GearSix },
];

interface SidebarProps { page: Page; setPage: (page: Page) => void; script?: ScriptProgress }

function alignmentLabel(alignment: number): string {
  if (alignment >= 2) return '光明';
  if (alignment <= -2) return '幽暗';
  return '平衡';
}

export function Sidebar({ page, setPage, script }: SidebarProps) {
  return (
    <aside className="sidebar">
      <div className="brand-mark" aria-label="灰燼王冠"><Scroll size={22} weight="regular" /></div>
      <nav aria-label="主要功能" className="sidebar-nav">{items.map(({ id, label, icon: Icon }) => <button key={id} type="button" onClick={() => setPage(id)} className={`nav-item ${page === id ? 'nav-item-active' : ''}`} aria-current={page === id ? 'page' : undefined}><Icon size={20} weight={page === id ? 'fill' : 'regular'} /><span>{label}</span></button>)}</nav>
      {script && (
        <div className="sidebar-script" aria-label="劇本進度" title={script.nodeTitle}>
          <span className={`script-stage ${script.ended ? (script.ending === 'good' ? 'script-stage-good' : script.ending === 'neutral' ? 'script-stage-neutral' : 'script-stage-bad') : ''}`}>
            {script.ended ? (script.ending === 'good' ? '光明結局' : script.ending === 'neutral' ? '蒼灰結局' : '沉沒結局') : script.stage}
          </span>
          <strong className="script-node">{script.nodeTitle}</strong>
          <span className="script-progress">{Math.min(script.visitedCount + 1, script.totalNodes)}/{script.totalNodes} 節點</span>
          <span className="script-alignment">命運傾向・{alignmentLabel(script.alignment)}</span>
        </div>
      )}
      <div className="sidebar-chapter">I</div>
    </aside>
  );
}
