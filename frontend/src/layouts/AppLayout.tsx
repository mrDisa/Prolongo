import { Menu, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { Logo } from '@/components/Logo';
import { SidebarContent } from '@/components/SidebarContent';

/**
 * App shell. From 900 px the sidebar is pinned to the viewport and only the
 * content scrolls; below that it becomes a top bar with a burger menu.
 */
export function AppLayout() {
  const [menuOpen, setMenuOpen] = useState(false);
  const location = useLocation();

  useEffect(() => {
    setMenuOpen(false);
  }, [location.pathname]);

  return (
    <div className="min-h-screen min-[900px]:flex">
      <aside className="sticky top-0 hidden h-screen w-sidebar shrink-0 overflow-y-auto border-r border-line bg-surface min-[900px]:block">
        <SidebarContent />
      </aside>

      <header className="sticky top-0 z-20 border-b border-line bg-surface min-[900px]:hidden">
        <div className="flex h-14 items-center justify-between px-4">
          <Logo />
          <button
            type="button"
            aria-label={menuOpen ? 'Закрыть меню' : 'Открыть меню'}
            aria-expanded={menuOpen}
            aria-controls="mobile-menu"
            onClick={() => setMenuOpen((open) => !open)}
            className="flex h-10 w-10 items-center justify-center rounded-field text-ink hover:bg-surface-2"
          >
            {menuOpen ? <X size={22} aria-hidden="true" /> : <Menu size={22} aria-hidden="true" />}
          </button>
        </div>
        {menuOpen ? (
          <div id="mobile-menu" className="max-h-[calc(100vh-56px)] overflow-y-auto border-t border-line">
            <SidebarContent />
          </div>
        ) : null}
      </header>

      <main className="min-w-0 flex-1 px-4 py-6 min-[900px]:px-8 min-[900px]:py-8 xl:px-10">
        <div className="mx-auto max-w-content">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
