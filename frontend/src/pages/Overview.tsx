import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';
import { unwrap } from '@/api/errors';
import { EmptyState } from '@/components/EmptyState';
import { ErrorState } from '@/components/ErrorState';
import { InsightHero } from '@/components/InsightHero';
import { LevelBar } from '@/components/LevelBar';
import { PageHeader } from '@/components/PageHeader';
import { PeriodSelector } from '@/components/PeriodSelector';
import { TopList } from '@/components/TopList';
import { formatHorizonEnd } from '@/lib/insight';
import { useHorizon } from '@/lib/period';

export default function Overview() {
  const { horizonDays, requestHorizonDays, settings, setHorizonDays } = useHorizon();
  const thresholds = settings?.thresholds ?? { medium: 30, high: 60 };
  const summaryQuery = useQuery({
    queryKey: ['summary', requestHorizonDays ?? 'model-default'],
    queryFn: () => unwrap(api.GET('/api/v1/analytics/summary', {
      params: { query: requestHorizonDays === undefined ? {} : { horizonDays: requestHorizonDays } },
    })),
  });
  const summary = summaryQuery.data;

  return (
    <>
      <PageHeader
        title="Обзор"
        subtitle={summary ? `Кто может уйти и сколько это стоит — прогноз на ${summary.horizonDays} дн., ${formatHorizonEnd(summary.referenceTime, summary.horizonDays)}` : 'Кто может уйти и сколько это стоит'}
        actions={<PeriodSelector value={summary?.horizonDays ?? horizonDays ?? 30} referenceTime={summary?.referenceTime} onChange={setHorizonDays} />}
      />

      {summaryQuery.isError && !summary ? (
        <ErrorState message={summaryQuery.error instanceof Error ? summaryQuery.error.message : undefined} onRetry={() => void summaryQuery.refetch()} />
      ) : summary ? (
        <div className="space-y-6">
          <InsightHero summary={summary} />
          <LevelBar customers={summary.customers} horizonDays={summary.horizonDays} thresholds={thresholds} />
          <section className="rounded-block bg-surface p-6" aria-labelledby="top-list-title">
            <div className="mb-4">
              <h2 id="top-list-title" className="font-display text-[20px] font-semibold">Кому звонить первым</h2>
              <p className="mt-1 text-[14px] text-ink-2">Клиенты с наибольшими ожидаемыми потерями</p>
            </div>
            {summary.topAtRisk.length ? <TopList customers={summary.topAtRisk} horizonDays={summary.horizonDays} /> : (
              <EmptyState title="Нет клиентов под риском" hint="Когда появятся клиенты с оценкой, они будут показаны здесь." />
            )}
          </section>
        </div>
      ) : summaryQuery.isPending ? (
        <OverviewSkeleton />
      ) : null}
    </>
  );
}

function OverviewSkeleton() {
  return (
    <div className="space-y-6" aria-label="Загрузка обзора" role="status">
      <div className="h-56 animate-pulse rounded-hero bg-surface" />
      <div className="h-56 animate-pulse rounded-block bg-surface" />
      <div className="h-96 animate-pulse rounded-block bg-surface" />
      <span className="sr-only">Загружаем сводку</span>
    </div>
  );
}
