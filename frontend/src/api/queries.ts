import { useQuery } from '@tanstack/react-query';
import { api } from './client';
import { unwrap } from './errors';

export const ME_KEY = ['me'] as const;
export const AUTH_CONFIG_KEY = ['auth-config'] as const;
export const MODEL_SETTINGS_KEY = ['model-settings'] as const;
export const MODEL_VERSIONS_KEY = ['model-settings-versions'] as const;
export const SOURCE_EVENTS_KEY = ['events'] as const;
export const SIMULATOR_KEY = ['simulator'] as const;

/** Model settings shared by the overview horizon and model editor. */
export function useModelSettings() {
  return useQuery({
    queryKey: MODEL_SETTINGS_KEY,
    queryFn: () => unwrap(api.GET('/api/v1/model/settings')),
    staleTime: 60_000,
  });
}

/** Saved versions of the model settings, newest (current) first. */
export function useModelVersions() {
  return useQuery({
    queryKey: MODEL_VERSIONS_KEY,
    queryFn: () => unwrap(api.GET('/api/v1/model/settings/versions')),
    staleTime: 60_000,
  });
}

/** Current user. A 401 here means "not signed in" and is handled by RequireAuth. */
export function useMe() {
  return useQuery({
    queryKey: ME_KEY,
    queryFn: () => unwrap(api.GET('/api/v1/auth/me')),
    staleTime: 60_000,
  });
}

/** Public sign-in options, e.g. whether to show «Войти через SSO». Works without a session. */
export function useAuthConfig() {
  return useQuery({
    queryKey: AUTH_CONFIG_KEY,
    queryFn: () => unwrap(api.GET('/api/v1/auth/config')),
    staleTime: 5 * 60_000,
  });
}

/** Latest source status changes shown on the data page. */
export function useSourceEvents() {
  return useQuery({
    queryKey: [...SOURCE_EVENTS_KEY, { page: 1, pageSize: 50 }],
    queryFn: () => unwrap(api.GET('/api/v1/source/events', { params: { query: { page: 1, pageSize: 50 } } })),
  });
}

/** Simulator state is refreshed frequently only while it is running. */
export function useSimulator() {
  return useQuery({
    queryKey: SIMULATOR_KEY,
    queryFn: () => unwrap(api.GET('/api/v1/source/simulator')),
    refetchInterval: (query) => query.state.data?.running ? 3_000 : false,
  });
}
