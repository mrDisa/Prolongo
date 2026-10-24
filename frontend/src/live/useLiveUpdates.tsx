import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { LiveStatus } from '@/components/LiveDot';

interface LiveUpdatesState {
  status: LiveStatus;
  calculatedAt: string | null;
  calculationRevision: number;
}

const LiveUpdatesContext = createContext<LiveUpdatesState>({
  status: 'idle',
  calculatedAt: null,
  calculationRevision: 0,
});

const QUIET_TIMEOUT_MS = 20_000;
const FALLBACK_POLL_MS = 10_000;
const INVALIDATION_INTERVAL_MS = 1_000;
const CLOSED_RETRY_MS = 5_000;

export function LiveUpdatesProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<LiveStatus>('idle');
  const [calculatedAt, setCalculatedAt] = useState<string | null>(null);
  const [calculationRevision, setCalculationRevision] = useState(0);

  useEffect(() => {
    let eventSource: EventSource | null = null;
    let lastMessageAt = Date.now();
    let lastVersion: number | null = null;
    let lastInvalidationAt = 0;
    let invalidationTimer: number | null = null;
    let fallbackTimer: number | null = null;
    let reconnectTimer: number | null = null;
    let disposed = false;

    const clearFallback = () => {
      if (fallbackTimer !== null) window.clearInterval(fallbackTimer);
      fallbackTimer = null;
    };

    const invalidateQueries = () => {
      lastInvalidationAt = Date.now();
      void queryClient.invalidateQueries({ predicate: (query) => !['me', 'settings', 'model-settings'].includes(String(query.queryKey[0])) }).then(() => {
        if (!disposed) setCalculationRevision((revision) => revision + 1);
      });
    };

    const scheduleInvalidation = () => {
      const wait = INVALIDATION_INTERVAL_MS - (Date.now() - lastInvalidationAt);
      if (wait <= 0) {
        invalidateQueries();
        return;
      }
      if (invalidationTimer !== null) return;
      invalidationTimer = window.setTimeout(() => {
        invalidationTimer = null;
        if (!disposed) invalidateQueries();
      }, wait);
    };

    const receiveRecalculated = (event: MessageEvent<string>) => {
      lastMessageAt = Date.now();
      setStatus('live');
      clearFallback();
      try {
        const payload = JSON.parse(event.data) as { calcVersion?: number; calculatedAt?: string };
        if (typeof payload.calculatedAt === 'string') setCalculatedAt(payload.calculatedAt);
        if (typeof payload.calcVersion !== 'number') return;
        if (lastVersion === null) {
          lastVersion = payload.calcVersion;
          return;
        }
        if (payload.calcVersion === lastVersion) return;
        lastVersion = payload.calcVersion;
        scheduleInvalidation();
      } catch {
        // Ignore malformed event data and keep the EventSource connection alive.
      }
    };

    const fakeLive = import.meta.env.DEV && import.meta.env.VITE_FAKE_LIVE === '1';
    if (fakeLive) {
      let version = 0;
      receiveRecalculated({ data: JSON.stringify({ calcVersion: version, calculatedAt: new Date().toISOString() }) } as MessageEvent<string>);
      const timer = window.setInterval(() => {
        receiveRecalculated({ data: JSON.stringify({ calcVersion: ++version, calculatedAt: new Date().toISOString() }) } as MessageEvent<string>);
      }, 15_000);
      return () => {
        disposed = true;
        window.clearInterval(timer);
        if (invalidationTimer !== null) window.clearTimeout(invalidationTimer);
      };
    }

    const connect = () => {
      if (disposed) return;
      const source = new EventSource('/api/v1/stream', { withCredentials: true });
      eventSource = source;
      source.onopen = () => {
        if (eventSource !== source) return;
        lastMessageAt = Date.now();
        setStatus('live');
        clearFallback();
      };
      source.onerror = () => {
        if (eventSource !== source) return;
        setStatus('reconnecting');
        if (source.readyState !== EventSource.CLOSED || reconnectTimer !== null) return;
        source.close();
        reconnectTimer = window.setTimeout(() => {
          reconnectTimer = null;
          connect();
        }, CLOSED_RETRY_MS);
      };
      source.addEventListener('ping', () => {
        if (eventSource !== source) return;
        lastMessageAt = Date.now();
        setStatus('live');
        clearFallback();
      });
      source.addEventListener('recalculated', (event) => {
        if (eventSource === source) receiveRecalculated(event as MessageEvent<string>);
      });
    };
    connect();
    const watchdog = window.setInterval(() => {
      if (Date.now() - lastMessageAt < QUIET_TIMEOUT_MS) return;
      setStatus('reconnecting');
      if (fallbackTimer !== null) return;
      fallbackTimer = window.setInterval(() => {
        void queryClient.invalidateQueries({ queryKey: ['summary'] });
      }, FALLBACK_POLL_MS);
    }, 1_000);

    return () => {
      disposed = true;
      eventSource?.close();
      window.clearInterval(watchdog);
      clearFallback();
      if (invalidationTimer !== null) window.clearTimeout(invalidationTimer);
      if (reconnectTimer !== null) window.clearTimeout(reconnectTimer);
    };
  }, [queryClient]);

  const value = useMemo(() => ({ status, calculatedAt, calculationRevision }), [status, calculatedAt, calculationRevision]);

  return (
    <LiveUpdatesContext.Provider value={value}>
      {children}
    </LiveUpdatesContext.Provider>
  );
}

export function useLiveUpdates() {
  return useContext(LiveUpdatesContext);
}

export function useRecentlyChanged(value: string | number | null | undefined) {
  const previousValue = useRef(value);
  const [changed, setChanged] = useState(false);
  const timer = useRef<number | null>(null);

  useEffect(() => {
    if (Object.is(previousValue.current, value)) return;
    previousValue.current = value;
    setChanged(true);
    if (timer.current !== null) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      timer.current = null;
      setChanged(false);
    }, 1_200);
    return () => {
      if (timer.current !== null) window.clearTimeout(timer.current);
    };
  }, [value]);

  return changed;
}
