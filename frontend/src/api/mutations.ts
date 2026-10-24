import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { api } from './client';
import { toApiError, unwrap } from './errors';
import { ME_KEY, SIMULATOR_KEY, SOURCE_EVENTS_KEY } from './queries';
import type { components } from './schema';

type LoginRequest = components['schemas']['LoginRequest'];
type SimulatorAction = components['schemas']['SimulatorAction'];

export function useUploadSnapshot() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (file: File) => unwrap(api.PUT('/api/v1/source/snapshot', {
      body: { file: file.name },
      bodySerializer: () => {
        const form = new FormData();
        form.append('file', file);
        return form;
      },
    })),
    onSuccess: () => queryClient.invalidateQueries({
      predicate: (query) => !['me', 'model-settings'].includes(String(query.queryKey[0])),
    }),
  });
}

export function useControlSimulator() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SimulatorAction) => unwrap(api.POST('/api/v1/source/simulator', { body })),
    onSuccess: (state, variables) => {
      queryClient.setQueryData(SIMULATOR_KEY, state);
      if (variables.action === 'tick') queryClient.invalidateQueries({ queryKey: SOURCE_EVENTS_KEY });
    },
  });
}

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
