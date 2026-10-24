import type { components } from '@/api/schema';
import { EmptyState } from '@/components/EmptyState';
import { ErrorState } from '@/components/ErrorState';

type TimelinePoint = components['schemas']['TimelinePoint'];
type TimelineResponse = components['schemas']['TimelineResponse'];
type Bucket = { key: string; label: string; fact: number; forecast: number };

export function ForecastBars({ data, isPending, error, onRetry, horizonDays, referenceTime }: {
  data?: TimelineResponse;
  isPending: boolean;
  error: Error | null;
  onRetry: () => void;
  horizonDays: number;
  referenceTime: string;
}) {
  const buckets = data ? aggregateTimeline(data.points, horizonDays) : [];
  const max = Math.max(1, ...buckets.flatMap(({ fact, forecast }) => [fact, forecast]));
  const forecastTotal = buckets.reduce((total, bucket) => total + bucket.forecast, 0);
  const factTotal = buckets.reduce((total, bucket) => total + bucket.fact, 0);

  return (
    <section className="min-w-0 rounded-block bg-surface p-6" aria-labelledby="forecast-title">
      <div>
        <h2 id="forecast-title" className="font-display text-[20px] font-semibold">Когда могут не продлить</h2>
        <p className="mt-1 text-[14px] text-ink-2">Фактические удаления и ожидаемые непродления</p>
      </div>
      {error && !data ? (
        <div className="mt-4"><ErrorState message={error.message} onRetry={onRetry} /></div>
      ) : isPending && !data ? (
        <div className="mt-5 h-56 animate-pulse rounded-tile-lg bg-surface-2" aria-label="Загрузка графика" role="status" />
      ) : buckets.length === 0 ? (
        <div className="mt-4"><EmptyState title="Пока нет данных для графика" hint="История удалений и прогноз появятся после расчёта." /></div>
      ) : (
        <>
          <div className="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2 text-[12px] text-ink-2" aria-label="Обозначения графика">
            <span className="inline-flex items-center gap-2"><span className="h-3 w-3 rounded-sm bg-ink-2" aria-hidden="true" />Факт · {formatNumber(factTotal)}</span>
            <span className="inline-flex items-center gap-2"><span className="h-3 w-3 rounded-sm border border-accent bg-accent-soft" style={{ backgroundImage: 'repeating-linear-gradient(135deg, var(--stripe) 0 2px, transparent 2px 5px)' }} aria-hidden="true" />Прогноз · {formatNumber(forecastTotal)}</span>
          </div>
          <div className="mt-4 overflow-x-auto pb-2">
            <div
              className="flex h-[174px] items-end gap-1 border-b border-line px-1"
              role="img"
              aria-label={`График за ${horizonDays} дней от ${formatAxisDate(referenceTime)}: факт ${formatNumber(factTotal)}, прогноз ${formatNumber(forecastTotal)} непродлений`}
              style={{ minWidth: `${Math.max(100, buckets.length * 30)}px` }}
            >
              {buckets.map((bucket) => (
                <div key={bucket.key} className="flex h-full min-w-0 flex-1 flex-col items-center justify-end">
                  <div className="flex h-[140px] w-full items-end justify-center gap-1">
                    <span
                      title={`Факт, ${bucket.label}: ${formatNumber(bucket.fact)}`}
                      aria-hidden="true"
                      className="w-[38%] min-w-[5px] rounded-t-[3px] bg-ink-2"
                      style={{ height: `${barHeight(bucket.fact, max)}%` }}
                    />
                    <span
                      title={`Прогноз, ${bucket.label}: ${formatNumber(bucket.forecast)}`}
                      aria-hidden="true"
                      className="w-[38%] min-w-[5px] rounded-t-[3px] border-x border-t border-accent bg-accent-soft"
                      style={{ height: `${barHeight(bucket.forecast, max)}%`, backgroundImage: 'repeating-linear-gradient(135deg, var(--stripe) 0 2px, transparent 2px 5px)' }}
                    />
                  </div>
                  <span className="mt-2 max-w-full truncate text-[10px] text-ink-3">{bucket.label}</span>
                </div>
              ))}
            </div>
          </div>
          <p className="mt-3 text-[13px] text-ink-2">Прогноз на {horizonDays} дн. от {formatAxisDate(referenceTime)} · ожидаемых непродлений: {formatNumber(forecastTotal)}</p>
        </>
      )}
    </section>
  );
}

function aggregateTimeline(points: TimelinePoint[], horizonDays: number): Bucket[] {
  const unit: 'day' | 'week' | 'month' = horizonDays <= 31 ? 'day' : horizonDays <= 93 ? 'week' : 'month';
  const buckets = new Map<string, Bucket>();

  for (const point of points) {
    const date = new Date(`${point.date}T00:00:00.000Z`);
    const bucket = getBucket(date, unit);
    const current = buckets.get(bucket.key) ?? { ...bucket, fact: 0, forecast: 0 };
    if (point.kind === 'fact') current.fact += (point.deletions ?? 0) + (point.deletionsEstimated ?? 0);
    else current.forecast += point.expectedNonRenewals ?? 0;
    buckets.set(bucket.key, current);
  }

  return [...buckets.values()].sort((a, b) => a.key.localeCompare(b.key));
}

function getBucket(date: Date, unit: 'day' | 'week' | 'month'): Pick<Bucket, 'key' | 'label'> {
  if (unit === 'day') {
    const key = date.toISOString().slice(0, 10);
    return { key, label: formatAxisDate(key) };
  }
  if (unit === 'month') {
    const key = date.toISOString().slice(0, 7);
    return { key, label: new Intl.DateTimeFormat('ru-RU', { month: 'short', year: '2-digit', timeZone: 'UTC' }).format(date).replace(/\.$/, '') };
  }
  const mondayOffset = (date.getUTCDay() + 6) % 7;
  const monday = new Date(date.getTime() - mondayOffset * 24 * 60 * 60 * 1000);
  const key = monday.toISOString().slice(0, 10);
  return { key, label: formatAxisDate(key) };
}

function formatAxisDate(value: string): string {
  const date = new Date(value.length === 10 ? `${value}T00:00:00.000Z` : value);
  return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', timeZone: 'UTC' })
    .format(date)
    .replace(/\.$/, '');
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 }).format(value);
}

function barHeight(value: number, max: number): number {
  return value > 0 ? Math.max(3, (value / max) * 100) : 1;
}
