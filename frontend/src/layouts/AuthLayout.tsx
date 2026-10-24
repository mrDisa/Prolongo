import { Outlet } from 'react-router-dom';

/** Layout for screens without a session. The login page draws its own two halves. */
export function AuthLayout() {
  return (
    <main className="min-h-screen">
      <Outlet />
    </main>
  );
}
