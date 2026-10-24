import { Database, LayoutDashboard, LogOut, SlidersHorizontal, Users } from 'lucide-react';
import { Link, NavLink } from 'react-router-dom';
import { useLogout } from '@/api/mutations';
import { useMe } from '@/api/queries';
import { cn } from '@/lib/utils';
import { Logo } from './Logo';
import { LiveDot } from './LiveDot';
import { ThemeToggle } from './ThemeToggle';
import { useLiveUpdates } from '@/live/useLiveUpdates';

const NAV_ITEMS = [
  { to: '/', label: 'Обзор', Icon: LayoutDashboard, end: true },
  { to: '/customers', label: 'Клиенты', Icon: Users, end: false },
  { to: '/model', label: 'Модель риска', Icon: SlidersHorizontal, end: false },
  { to: '/data', label: 'Данные', Icon: Database, end: false },
];

/** Sidebar body, shared by the fixed desktop sidebar and the mobile menu. */
export function SidebarContent() {
  const { data } = useMe();
  const { status, calculatedAt } = useLiveUpdates();
  const logout = useLogout();
  const user = data?.user;
  const initial = (user?.displayName ?? user?.username ?? '?').charAt(0).toUpperCase();

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
        <LiveDot status={status} label={status === 'live' && calculatedAt ? `Live · данные на ${formatCalculatedAt(calculatedAt)}` : undefined} />
        <ThemeToggle />
        <div className="flex items-center gap-3 rounded-tile bg-surface-2 p-3">
          <div
            aria-hidden="true"
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent-soft text-[14px] font-semibold text-accent-text"
          >
            {initial}
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-[14px] font-semibold">{user?.displayName ?? '—'}</div>
            <div className="truncate text-[12px] text-ink-3">{user?.username}</div>
          </div>
          <button
            type="button"
            aria-label="Выйти"
            onClick={() => logout.mutate()}
            disabled={logout.isPending}
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-seg text-ink-2 hover:bg-surface disabled:opacity-60"
          >
            <LogOut size={18} strokeWidth={2} aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>
  );
}

function formatCalculatedAt(value: string) {
  return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
    .format(new Date(value))
    .replace(/\.$/, '');
}
