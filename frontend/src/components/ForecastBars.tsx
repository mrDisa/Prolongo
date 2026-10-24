import type { components } from '@/api/schema';
import { EmptyState } from '@/components/EmptyState';
import { ErrorState } from '@/components/ErrorState';

type TimelinePoint = components['schemas']['TimelinePoint'];
type TimelineResponse = components['schemas']['TimelineResponse'];
type ForecastBucket = { key: string; label: string; value: number };

export function ForecastBars({ data, isPending, error, onRetry, horizonDays, referenceTime }: {
  data?: TimelineResponse;
  isPending: boolean;
  error: Error | null;
  onRetry: () => void;
  horizonDays: number;
  referenceTime: string;
}) {
  const points = data?.points ?? [];
  const factPoints = points.filter((point) => point.kind === 'fact');
  const forecastPoints = points.filter((point) => point.kind === 'forecast');
  const buckets = aggregateForecast(forecastPoints, horizonDays);
  const max = Math.max(1, ...buckets.map((bucket) => bucket.value));
  const factTotal = factPoints.reduce((total, point) => total + (point.deletions ?? 0) + (point.deletionsEstimated ?? 0), 0);
  const forecastTotal = forecastPoints.reduce((total, point) => total + (point.expectedNonRenewals ?? 0), 0);
  const peak = buckets.reduce<ForecastBucket | undefined>((highest, bucket) => !highest || bucket.value > highest.value ? bucket : highest, undefined);
  const chartReferenceTime = data?.referenceTime ?? referenceTime;
  const ticks = getTicks(buckets);

  return (
    <section className="min-w-0 rounded-block bg-surface p-6" aria-labelledby="forecast-title">
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
        <div>
          <h2 id="forecast-title" className="font-display text-[20px] font-semibold">Когда могут не продлить</h2>
          <p className="mt-1 text-[14px] text-ink-2">Ожидаемое число непродлённых услуг по дням на {horizonDays} дн. вперёд</p>
        </div>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-[12px] text-ink-2" aria-label="Обозначения графика">
          <span className="inline-flex items-center gap-2"><span className="h-3 w-3 rounded-sm bg-ink-2" aria-hidden="true" />Факт</span>
          <span className="inline-flex items-center gap-2"><span className="h-3 w-3 rounded-sm border border-accent bg-accent-soft" style={{ backgroundImage: 'repeating-linear-gradient(135deg, var(--stripe) 0 2px, transparent 2px 5px)' }} aria-hidden="true" />Прогноз</span>
        </div>
      </div>

      {error && !data ? (
        <div className="mt-4"><ErrorState message={error.message} onRetry={onRetry} /></div>
      ) : isPending && !data ? (
        <div className="mt-5 h-56 animate-pulse rounded-tile-lg bg-surface-2" aria-label="Загрузка графика" role="status" />
      ) : buckets.length === 0 ? (
        <div className="mt-4"><EmptyState title="Пока нет данных для прогноза" hint="Прогноз появится после расчёта риска клиентов." /></div>
      ) : (
        <>
          <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-[150px_minmax(0,1fr)]">
            <aside className="flex min-h-[220px] flex-col justify-center rounded-tile-lg border border-dashed border-line-strong p-4">
              <p className="text-[12px] font-semibold text-ink-2">Факт · {factPoints.length} дн. назад</p>
              <p className="mt-1 font-display text-[24px] font-semibold">{formatNumber(factTotal)}</p>
              <p className="mt-1 text-[12px] leading-4 text-ink-3">удалений с датой.<br />История копится с момента запуска</p>
            </aside>

            <div className="min-w-0">
              <div className="w-full">
                <div
                  className="flex h-[194px] items-end gap-[2px] border-b border-line px-1"
                  role="img"
                  aria-label={"Прогноз на " + horizonDays + " дней от " + formatAxisDate(chartReferenceTime) + ": ожидается " + formatNumber(forecastTotal) + " непродлений"}
                >
                  {buckets.map((bucket) => (
                    <div key={bucket.key} className="flex h-full min-w-0 flex-1 items-end justify-center">
                      <div className="flex h-[164px] w-full items-end justify-center">
                        {bucket.value > 0 ? (
                          <div
                            title={"Прогноз, " + bucket.label + ": " + formatNumber(bucket.value)}
                            aria-hidden="true"
                            className="relative block w-[58%] min-w-[5px] rounded-t-[3px] border-x border-t border-accent bg-accent-soft"
                            style={{ height: barHeight(bucket.value, max), backgroundImage: 'repeating-linear-gradient(135deg, var(--stripe) 0 2px, transparent 2px 5px)' }}
                          >
                            <span className="absolute bottom-[calc(100%_+_3px)] left-1/2 -translate-x-1/2 whitespace-nowrap text-[10px] font-medium leading-none text-ink-2">
                              {formatNumber(bucket.value)}
                            </span>
                          </div>
                        ) : (
                          <div
                            title={"Прогноз, " + bucket.label + ": 0"}
                            aria-hidden="true"
                            className="h-[2px] w-[58%] min-w-[5px] rounded-full bg-line-strong"
                          />
                        )}
                      </div>
                    </div>
                  ))}
                </div>
                <div className="relative mt-2 h-5 text-[11px] text-ink-3" aria-hidden="true">
                  {ticks.map((tick) => (
                    <span key={tick.key} className="absolute -translate-x-1/2 whitespace-nowrap" style={{ left: tick.position + '%' }}>
                      {tick.label}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          </div>

          <p className="mt-4 rounded-tile-lg bg-surface-2 p-4 text-[13px] leading-5 text-ink-2">
            {forecastInsight(forecastTotal, peak, forecastPoints)}
          </p>
        </>
      )}
    </section>
  );
}

function aggregateForecast(points: TimelinePoint[], horizonDays: number): ForecastBucket[] {
  const unit: 'day' | 'week' | 'month' = horizonDays <= 31 ? 'day' : horizonDays <= 93 ? 'week' : 'month';
  const buckets = new Map<string, ForecastBucket>();

  for (const point of points) {
    const date = new Date(point.date + 'T00:00:00.000Z');
    const bucket = getBucket(date, unit);
    const current = buckets.get(bucket.key) ?? { ...bucket, value: 0 };
    current.value += point.expectedNonRenewals ?? 0;
    buckets.set(bucket.key, current);
  }

  return [...buckets.values()].sort((a, b) => a.key.localeCompare(b.key));
}

function getBucket(date: Date, unit: 'day' | 'week' | 'month'): Pick<ForecastBucket, 'key' | 'label'> {
  if (unit === 'day') {
    const key = date.toISOString().slice(0, 10);
    return { key, label: formatAxisDate(key) };
  }
  if (unit === 'month') {
    const key = date.toISOString().slice(0, 7);
    const label = new Intl.DateTimeFormat('ru-RU', { month: 'short', year: '2-digit', timeZone: 'UTC' }).format(date).replace(/\.$/, '');
    return { key, label };
  }
  const mondayOffset = (date.getUTCDay() + 6) % 7;
  const monday = new Date(date.getTime() - mondayOffset * 24 * 60 * 60 * 1000);
  const key = monday.toISOString().slice(0, 10);
  return { key, label: formatAxisDate(key) };
}

function getTicks(buckets: ForecastBucket[]): Array<{ key: string; label: string; position: number }> {
  if (!buckets.length) return [];
  const positionFor = (index: number) => ((index + 0.5) / buckets.length) * 100;
  const ticks = [{ key: 'start', label: 'завтра', position: positionFor(0) }];
  if (buckets.length > 2) {
    const middleIndex = Math.floor((buckets.length - 1) / 2);
    ticks.push({ key: 'middle', label: formatAxisDate(buckets[middleIndex].key), position: positionFor(middleIndex) });
  }
  if (buckets.length > 1) {
    const lastIndex = buckets.length - 1;
    ticks.push({ key: 'end', label: formatAxisDate(buckets[lastIndex].key), position: positionFor(lastIndex) });
  }
  return ticks;
}

function formatAxisDate(value: string): string {
  const dateInput = /^\d{4}-\d{2}$/.test(value)
    ? value + '-01T00:00:00.000Z'
    : /^\d{4}-\d{2}-\d{2}/.test(value)
      ? value.slice(0, 10) + 'T00:00:00.000Z'
      : value;
  const date = new Date(dateInput);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', timeZone: 'UTC' })
    .format(date)
    .replace(/\.$/, '');
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 }).format(value);
}

function barHeight(value: number, max: number): string {
  return value > 0 ? Math.max(3, (value / max) * 100) + '%' : '1px';
}

function forecastInsight(total: number, peak: ForecastBucket | undefined, points: TimelinePoint[]): string {
  if (total <= 0 || !peak || peak.value <= 0) return 'По текущему прогнозу непродлений не ожидается.';
  const firstForecastDate = points.find((point) => point.kind === 'forecast')?.date;
  const peakDate = peak.key === firstForecastDate ? 'уже завтра' : 'около ' + peak.label;
  return 'Ожидаем, что не продлят ≈ ' + formatNumber(total) + ' услуги, из них ' + formatNumber(peak.value) + ' — ' + peakDate + ': у этих услуг наступает срок продления.';
}
