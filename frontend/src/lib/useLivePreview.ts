import { useCallback, useEffect, useRef, useState } from 'react';
import type { components } from '@/api/schema';
import { api } from '@/api/client';
import { unwrap } from '@/api/errors';

type SettingsPatch = components['schemas']['ModelSettingsPatch'];
type PreviewResponse = components['schemas']['PreviewResponse'];

/**
 * Live preview of a settings draft.
 *
 * Only one request runs at a time. While it runs, only the newest draft waits in line,
 * so dragging a slider sends a few requests instead of one per step, and answers never
 * arrive out of order. The last answer stays on screen while the next one is computed.
 */
export function useLivePreview(patch: SettingsPatch | null, revision: number) {
  const [data, setData] = useState<PreviewResponse>();
  const [error, setError] = useState<Error | null>(null);
  const [pending, setPending] = useState(false);
  const running = useRef(false);
  const queued = useRef<SettingsPatch | null>(null);
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const request = useCallback((body: SettingsPatch) => {
    queued.current = body;
    if (running.current) return;
    running.current = true;
    setPending(true);

    void (async () => {
      while (queued.current && mounted.current) {
        const next = queued.current;
        queued.current = null;
        try {
          const result = await unwrap(api.POST('/api/v1/model/preview', { body: next }));
          if (!mounted.current) return;
          setData(result);
          setError(null);
        } catch (caught) {
          // A newer draft is already waiting: its answer will replace this error.
          if (mounted.current && !queued.current) setError(caught instanceof Error ? caught : new Error('Не удалось пересчитать'));
        }
      }
      running.current = false;
      if (mounted.current) setPending(false);
    })();
  }, []);

  useEffect(() => {
    if (patch) request(patch);
  }, [patch, revision, request]);

  const retry = useCallback(() => {
    if (patch) request(patch);
  }, [patch, request]);

  return { data, error, pending, retry };
}

/**
 * Smooths a busy flag for display: shows it only if the work lasts longer than `showAfterMs`
 * and keeps it at least `minVisibleMs`, so quick back-to-back requests do not blink.
 */
export function useSmoothFlag(active: boolean, showAfterMs = 150, minVisibleMs = 600) {
  const [visible, setVisible] = useState(false);
  const shownAt = useRef(0);

  useEffect(() => {
    if (active) {
      if (visible) return;
      const timer = window.setTimeout(() => {
        shownAt.current = Date.now();
        setVisible(true);
      }, showAfterMs);
      return () => window.clearTimeout(timer);
    }
    if (!visible) return;
    const left = Math.max(0, minVisibleMs - (Date.now() - shownAt.current));
    const timer = window.setTimeout(() => setVisible(false), left);
    return () => window.clearTimeout(timer);
  }, [active, visible, showAfterMs, minVisibleMs]);

  return visible;
}
