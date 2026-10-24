import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useModelSettings } from '@/api/queries';

const MAX_HORIZON_DAYS = 365;

export function normalizeHorizon(value: string | null): number | undefined {
  if (value === null) return undefined;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) return undefined;
  return Math.min(parsed, MAX_HORIZON_DAYS);
}

export function useHorizon() {
  const [searchParams, setSearchParams] = useSearchParams();
  const requestHorizonDays = normalizeHorizon(searchParams.get('h'));
  const settingsQuery = useModelSettings();
  const horizonDays = requestHorizonDays ?? settingsQuery.data?.horizonDays;

  const setHorizonDays = useCallback((days: number) => {
    const next = new URLSearchParams(searchParams);
    const validDays = Number.isFinite(days) ? Math.trunc(days) : 1;
    const normalized = Math.min(MAX_HORIZON_DAYS, Math.max(1, validDays));
    next.set('h', String(normalized));
    setSearchParams(next);
  }, [searchParams, setSearchParams]);

  return {
    horizonDays,
    requestHorizonDays,
    settings: settingsQuery.data,
    setHorizonDays,
  };
}
