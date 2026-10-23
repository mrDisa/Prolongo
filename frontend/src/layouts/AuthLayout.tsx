import { Outlet } from 'react-router-dom';
import { ThemeToggle } from '@/components/ThemeToggle';

/** Layout for screens without a session (login). */
export function AuthLayout() {
  return (
    <div className="relative flex min-h-screen items-center justify-center px-6 py-12">
      <ThemeToggle className="absolute right-6 top-6" />
      <main className="w-full max-w-[400px]">
        <Outlet />
      </main>
    </div>
  );
}
