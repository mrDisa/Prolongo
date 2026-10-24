import createClient from 'openapi-fetch';
import type { paths } from './schema';

type UnauthorizedHandler = () => void;

let onUnauthorized: UnauthorizedHandler | null = null;

/** Called on any 401 outside the auth endpoints (the session is gone). */
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null) {
  onUnauthorized = handler;
}

// Login, me and logout handle their own 401: it is a normal answer for them.
function isAuthEndpoint(url: string): boolean {
  try {
    return new URL(url, window.location.origin).pathname.startsWith('/api/v1/auth/');
  } catch {
    return false;
  }
}

async function apiFetch(request: Request): Promise<Response> {
  const response = await fetch(request);
  if (response.status === 401 && !isAuthEndpoint(request.url)) {
    onUnauthorized?.();
  }
  return response;
}

// Paths in the spec already start with /api/v1, so the base URL stays empty:
// in dev Vite proxies /api, in production nginx does (same origin, no CORS).
export const api = createClient<paths>({
  baseUrl: '',
  credentials: 'include',
  fetch: apiFetch,
});
