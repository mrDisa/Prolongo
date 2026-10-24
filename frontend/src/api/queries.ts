import { useQuery } from '@tanstack/react-query';
import { api } from './client';
import { unwrap } from './errors';

export const ME_KEY = ['me'] as const;

/** Current user. A 401 here means "not signed in" and is handled by RequireAuth. */
export function useMe() {
  return useQuery({
    queryKey: ME_KEY,
    queryFn: () => unwrap(api.GET('/api/v1/auth/me')),
    staleTime: 60_000,
  });
}
