import { QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from 'react-router-dom';
import { setUnauthorizedHandler } from './api/client';
import { queryClient } from './api/queryClient';
import { loginUrl } from './lib/auth';
import { router } from './routes';

// Any 401 outside the auth endpoints means the session is gone:
// drop cached data and go to the login page, remembering where we were.
setUnauthorizedHandler(() => {
  const { pathname, search } = router.state.location;
  if (pathname === '/login') {
    return;
  }
  void router.navigate(loginUrl(pathname + search), { replace: true });
  queryClient.clear();
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
