import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';

export const DEFAULT_HORIZON_DAYS = 30;
const MAX_HORIZON_DAYS = 365;

export function normalizeHorizon(value: string | null): number {
  if (value === null) return DEFAULT_HORIZON_DAYS;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) return DEFAULT_HORIZON_DAYS;
  return Math.min(parsed, MAX_HORIZON_DAYS);
}

export function useHorizon() {
  const [searchParams, setSearchParams] = useSearchParams();
  const horizonDays = normalizeHorizon(searchParams.get('h'));

  const setHorizonDays = useCallback((days: number) => {
    const next = new URLSearchParams(searchParams);
    const normalized = Math.min(MAX_HORIZON_DAYS, Math.max(1, Math.trunc(days)));
    if (normalized === DEFAULT_HORIZON_DAYS) next.delete('h');
    else next.set('h', String(normalized));
    setSearchParams(next);
  }, [searchParams, setSearchParams]);

  return { horizonDays, setHorizonDays };
}
