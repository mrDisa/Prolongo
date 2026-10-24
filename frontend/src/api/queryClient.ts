import { QueryClient } from '@tanstack/react-query';
import { ApiError } from './errors';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5_000,
      refetchOnWindowFocus: false,
      // Retry once, and only for network failures and 5xx: a 4xx will not fix itself.
      retry: (failureCount, error) => {
        const retryable = !(error instanceof ApiError) || error.status >= 500;
        return retryable && failureCount < 1;
      },
    },
  },
});
