import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { api } from './client';
import { toApiError, unwrap } from './errors';
import { ME_KEY } from './queries';
import type { components } from './schema';

type LoginRequest = components['schemas']['LoginRequest'];

export function useLogin() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: LoginRequest) => unwrap(api.POST('/api/v1/auth/login', { body })),
    // Reload the user so the guard sees the new session straight away.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ME_KEY }),
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  return useMutation({
    mutationFn: async () => {
      const { error, response } = await api.POST('/api/v1/auth/logout');
      // 401 means the session is already gone, which is what we wanted.
      if (error !== undefined && response.status !== 401) {
        throw toApiError(error, response);
      }
    },
    onSettled: () => {
      navigate('/login', { replace: true });
      queryClient.clear();
    },
  });
}
