import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { LiveStatus } from '@/components/LiveDot';

interface LiveUpdatesState {
  status: LiveStatus;
  updatedAt: number;
  recentlyUpdated: boolean;
}

const LiveUpdatesContext = createContext<LiveUpdatesState>({
  status: 'idle',
  updatedAt: 0,
  recentlyUpdated: false,
});

const QUIET_TIMEOUT_MS = 20_000;
const FALLBACK_POLL_MS = 10_000;
const INVALIDATION_INTERVAL_MS = 1_000;
const HIGHLIGHT_DURATION_MS = 1_200;

export function LiveUpdatesProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<LiveStatus>('reconnecting');
  const [updatedAt, setUpdatedAt] = useState(0);
  const [recentlyUpdated, setRecentlyUpdated] = useState(false);
  const pulseTimer = useRef<number | null>(null);

  useEffect(() => {
    let eventSource: EventSource | null = null;
    let lastMessageAt = Date.now();
    let lastVersion: number | null = null;
    let lastInvalidationAt = 0;
    let invalidationTimer: number | null = null;
    let fallbackTimer: number | null = null;
    let disposed = false;

    const markUpdated = () => {
      const now = Date.now();
      setUpdatedAt(now);
      setRecentlyUpdated(true);
      if (pulseTimer.current !== null) window.clearTimeout(pulseTimer.current);
      pulseTimer.current = window.setTimeout(() => {
        pulseTimer.current = null;
        setRecentlyUpdated(false);
      }, HIGHLIGHT_DURATION_MS);
    };

    const clearFallback = () => {
      if (fallbackTimer !== null) window.clearInterval(fallbackTimer);
      fallbackTimer = null;
    };

    const invalidateQueries = () => {
      lastInvalidationAt = Date.now();
      void queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'me' }).then(markUpdated);
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
        const payload = JSON.parse(event.data) as { calcVersion?: number };
        if (typeof payload.calcVersion !== 'number' || payload.calcVersion === lastVersion) return;
        lastVersion = payload.calcVersion;
        scheduleInvalidation();
      } catch {
        // Ignore malformed event data and keep the EventSource connection alive.
      }
    };

    const fakeLive = import.meta.env.DEV && import.meta.env.VITE_FAKE_LIVE === '1';
    if (fakeLive) {
      let version = 0;
      setStatus('live');
      const timer = window.setInterval(() => {
        lastMessageAt = Date.now();
        receiveRecalculated({ data: JSON.stringify({ calcVersion: ++version }) } as MessageEvent<string>);
      }, 15_000);
      return () => {
        disposed = true;
        window.clearInterval(timer);
        if (invalidationTimer !== null) window.clearTimeout(invalidationTimer);
        if (pulseTimer.current !== null) window.clearTimeout(pulseTimer.current);
      };
    }

    eventSource = new EventSource('/api/v1/stream', { withCredentials: true });
    eventSource.onopen = () => {
      lastMessageAt = Date.now();
      setStatus('live');
      clearFallback();
    };
    eventSource.onerror = () => setStatus('reconnecting');
    eventSource.addEventListener('ping', () => {
      lastMessageAt = Date.now();
      setStatus('live');
      clearFallback();
    });
    eventSource.addEventListener('recalculated', (event) => receiveRecalculated(event as MessageEvent<string>));

    const watchdog = window.setInterval(() => {
      if (Date.now() - lastMessageAt < QUIET_TIMEOUT_MS) return;
      setStatus('reconnecting');
      if (fallbackTimer !== null) return;
      fallbackTimer = window.setInterval(() => {
        void queryClient.invalidateQueries({ queryKey: ['summary'] }).then(markUpdated);
      }, FALLBACK_POLL_MS);
    }, 1_000);

    return () => {
      disposed = true;
      eventSource?.close();
      window.clearInterval(watchdog);
      clearFallback();
      if (invalidationTimer !== null) window.clearTimeout(invalidationTimer);
      if (pulseTimer.current !== null) window.clearTimeout(pulseTimer.current);
    };
  }, [queryClient]);

  return (
    <LiveUpdatesContext.Provider value={{ status, updatedAt, recentlyUpdated }}>
      {children}
    </LiveUpdatesContext.Provider>
  );
}

export function useLiveUpdates() {
  return useContext(LiveUpdatesContext);
}
