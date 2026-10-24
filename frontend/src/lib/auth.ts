/**
 * Where to go after sign-in. Only paths inside the app are accepted, so a crafted
 * `?next=https://evil.example` or `?next=//evil.example` can never leave the site.
 */
export function safeNext(raw: string | null): string {
  if (!raw || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) {
    return '/';
  }
  if (raw === '/login' || raw.startsWith('/login?') || raw.startsWith('/login/')) {
    return '/';
  }
  return raw;
}

/** `/login`, remembering the page to return to (`from` is path + query). */
export function loginUrl(from: string): string {
  const next = safeNext(from);
  return next === '/' ? '/login' : `/login?next=${encodeURIComponent(next)}`;
}
