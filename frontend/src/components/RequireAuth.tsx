import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { ApiError } from '@/api/errors';
import { useMe } from '@/api/queries';
import { ErrorState } from '@/components/ErrorState';
import { Skeleton } from '@/components/ui/skeleton';
import { loginUrl } from '@/lib/auth';

function FullScreenSkeleton() {
  return (
    <div className="min-h-screen min-[900px]:flex" aria-busy="true">
      <span className="sr-only" role="status">
        Загрузка…
      </span>
      <div className="hidden w-sidebar shrink-0 flex-col gap-3 border-r border-line bg-surface p-5 min-[900px]:flex">
        <Skeleton className="h-[38px] w-36" />
        <Skeleton className="mt-4 h-[46px]" />
        <Skeleton className="h-[46px]" />
        <Skeleton className="h-[46px]" />
        <Skeleton className="h-[46px]" />
      </div>
      <div className="flex-1 space-y-4 px-4 py-6 min-[900px]:px-10 min-[900px]:py-8">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-52 rounded-hero" />
        <Skeleton className="h-40 rounded-block" />
      </div>
    </div>
  );
}

/** Layout route: lets signed-in users through, sends everyone else to /login. */
export function RequireAuth() {
  const { isPending, isError, error, refetch } = useMe();
  const location = useLocation();

  if (isPending) {
    return <FullScreenSkeleton />;
  }

  if (isError) {
    if (error instanceof ApiError && error.status === 401) {
      return <Navigate to={loginUrl(location.pathname + location.search)} replace />;
    }
    return (
      <div className="mx-auto max-w-xl px-4 py-12">
        <ErrorState
          title="Не удалось проверить вход"
          message={
            error instanceof ApiError
              ? error.message
              : 'Не удалось связаться с сервером. Проверьте соединение.'
          }
          onRetry={() => void refetch()}
        />
      </div>
    );
  }

  return <Outlet />;
}
