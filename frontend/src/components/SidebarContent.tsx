import { Database, LayoutDashboard, LogOut, SlidersHorizontal, Users } from 'lucide-react';
import { Link, NavLink } from 'react-router-dom';
import { cn } from '@/lib/utils';
import { Logo } from './Logo';
import { LiveDot } from './LiveDot';
import { ThemeToggle } from './ThemeToggle';

const NAV_ITEMS = [
  { to: '/', label: 'Обзор', Icon: LayoutDashboard, end: true },
  { to: '/customers', label: 'Клиенты', Icon: Users, end: false },
  { to: '/model', label: 'Модель риска', Icon: SlidersHorizontal, end: false },
  { to: '/data', label: 'Данные', Icon: Database, end: false },
];

/** Sidebar body, shared by the fixed desktop sidebar and the mobile menu. */
export function SidebarContent() {
  return (
    <div className="flex h-full flex-col gap-6 p-5">
      <Link to="/" aria-label="Prolongo, на главную" className="rounded-xl">
        <Logo />
      </Link>

      <nav aria-label="Основная навигация" className="flex flex-col gap-1">
        {NAV_ITEMS.map(({ to, label, Icon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              cn(
                'flex h-[46px] items-center gap-3 rounded-field px-3.5 text-[15px] font-semibold',
                isActive ? 'bg-accent-soft text-accent-text' : 'text-ink-2 hover:bg-surface-2',
              )
            }
          >
            <Icon size={19} strokeWidth={2} aria-hidden="true" />
            {label}
          </NavLink>
        ))}
      </nav>

      <div className="mt-auto flex flex-col gap-4">
        <LiveDot />
        <ThemeToggle />
        <div className="flex items-center gap-3 rounded-tile bg-surface-2 p-3">
          <div
            aria-hidden="true"
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent-soft text-[14px] font-semibold text-accent-text"
          >
            А
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-[14px] font-semibold">Администратор</div>
            <div className="truncate text-[12px] text-ink-3">admin</div>
          </div>
          {/* Real sign-out (POST /auth/logout) arrives with task F2. */}
          <Link
            to="/login"
            aria-label="Выйти"
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-seg text-ink-2 hover:bg-surface"
          >
            <LogOut size={18} strokeWidth={2} aria-hidden="true" />
          </Link>
        </div>
      </div>
    </div>
  );
}
