import { BookOpen, Compass, GearSix, Scroll } from '@phosphor-icons/react';
import type { Page } from '../types';

const items = [
  { id: 'table' as const, label: '遊戲桌', icon: Compass },
  { id: 'journal' as const, label: '戰役紀錄', icon: BookOpen },
  { id: 'settings' as const, label: '設定', icon: GearSix },
];

interface SidebarProps {
  page: Page;
  setPage: (page: Page) => void;
}

export function Sidebar({ page, setPage }: SidebarProps) {
  return (
    <aside className="sidebar">
      <div className="brand-mark" aria-label="灰燼王冠">
        <Scroll size={22} weight="regular" />
      </div>
      <nav aria-label="主要功能" className="sidebar-nav">
        {items.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => setPage(id)}
            className={`nav-item ${page === id ? 'nav-item-active' : ''}`}
            aria-current={page === id ? 'page' : undefined}
          >
            <Icon size={20} weight={page === id ? 'fill' : 'regular'} />
            <span>{label}</span>
          </button>
        ))}
      </nav>
      <div className="sidebar-chapter">I</div>
    </aside>
  );
}
