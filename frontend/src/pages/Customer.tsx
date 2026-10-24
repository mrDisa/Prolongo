import { useState } from 'react';
import type { ReactNode } from 'react';
import { ArrowLeft, Check, Copy } from 'lucide-react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import type { components } from '@/api/schema';
import { api } from '@/api/client';
import { unwrap, ApiError } from '@/api/errors';
import { EmptyState } from '@/components/EmptyState';
import { ErrorState } from '@/components/ErrorState';
import { MoneySplit } from '@/components/MoneySplit';
import { PaymentStrip } from '@/components/PaymentStrip';
import { RiskBadge } from '@/components/RiskBadge';
import { ScoreScale } from '@/components/ScoreScale';
import { ScoreWaterfall } from '@/components/ScoreWaterfall';
import { formatDate, formatDaysLeft, formatEUR, formatRelative } from '@/lib/format';
import { levelLabel, noScoreReasonLabel } from '@/lib/levels';
import { useHorizon } from '@/lib/period';

type CustomerDetail = components['schemas']['CustomerDetail'];
type Service = components['schemas']['ServiceItem'];
type ServiceStatus = components['schemas']['ServiceStatus'];

const LIVE_STATUSES: ServiceStatus[] = ['active', 'suspended', 'prolong_wait'];

const STATUS_LABELS: Record<ServiceStatus, string> = {
  active: 'Активна',
  blocked: 'Заблокирована',
  deleted: 'Удалена',
  suspended: 'Приостановлена',
  prolong_wait: 'Ждёт оплаты',
};

const STATUS_SUMMARIES: Record<ServiceStatus, string> = {
  active: 'все активны',
  blocked: 'все заблокированы',
  deleted: 'все удалены',
  suspended: 'все приостановлены',
  prolong_wait: 'все ждут оплаты',
};

const TERM_LABELS: Record<components['schemas']['PaymentTerm'], string> = {
  hour: 'час',
  week: 'неделя',
  month: 'месяц',
  quarter_year: 'квартал',
  half_year: 'полгода',
  year: 'год',
};

const SOURCE_LABELS: Record<components['schemas']['EventSource'], string> = {
  seed: 'начальные данные',
  snapshot: 'снимок данных',
  event: 'событие',
  admin: 'админка',
  simulator: 'симулятор',
  sql: 'изменение в БД',
  demo_backfill: 'демо-история',
};

function noScoreMessage(customer: CustomerDetail): string {
  return noScoreReasonLabel(customer.noScoreReason);
}

function summaryLine(customer: CustomerDetail): string {
  const services = customer.services;
  const liveCount = services.filter((service) => LIVE_STATUSES.includes(service.status)).length;
  const serviceText = services.length === 0
    ? 'Услуг пока нет.'
    : `${liveCount} из ${services.length} услуг живые${services.every((service) => service.status === services[0].status) ? `, ${STATUS_SUMMARIES[services[0].status]}` : ''}.`;

  if (customer.scoreStatus === 'no_score') return serviceText;
  const mainReason = customer.contributions.find((contribution) => contribution.points > 0)?.title.toLocaleLowerCase('ru-RU') ?? 'нет выраженных факторов риска';
  return `${serviceText} Главная причина — ${mainReason}.`;
}

function SectionTitle({ children }: { children: ReactNode }) {
  return <h2 className="font-display text-[20px] font-semibold">{children}</h2>;
}

function CustomerSkeleton() {
  return (
    <div aria-label="Загрузка карточки клиента" className="space-y-6" role="status">
      <div className="rounded-hero bg-surface p-6"><div className="h-7 w-1/3 animate-pulse rounded bg-surface-2" /><div className="mt-4 h-5 w-2/3 animate-pulse rounded bg-surface-2" /></div>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{Array.from({ length: 4 }, (_, index) => <div key={index} className="h-28 animate-pulse rounded-block bg-surface" />)}</div>
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.4fr)_minmax(340px,0.9fr)]"><div className="h-[480px] animate-pulse rounded-block bg-surface" /><div className="h-72 animate-pulse rounded-block bg-surface" /></div>
      <span className="sr-only">Загружаем данные клиента</span>
    </div>
  );
}

function ServiceTable({ services }: { services: Service[] }) {
  if (!services.length) return <p className="mt-4 text-[14px] text-ink-2">Услуг нет.</p>;
  return (
    <div className="mt-4 overflow-x-auto">
      <table className="w-full min-w-[620px] text-left text-[14px]">
        <thead className="border-b border-line text-[12px] font-medium text-ink-3">
          <tr><th className="px-3 py-3">Услуга</th><th className="px-3 py-3">Статус</th><th className="px-3 py-3">Цена / период</th><th className="px-3 py-3">Оплачена до</th><th className="px-3 py-3">Автопродление</th></tr>
        </thead>
        <tbody className="divide-y divide-line">
          {services.map((service) => (
            <tr key={service.id}>
              <td className="whitespace-nowrap px-3 py-3 font-mono text-[13px]">{service.name}</td>
              <td className="whitespace-nowrap px-3 py-3"><span className="rounded-full bg-surface-2 px-3 py-1 text-[13px] font-semibold">{STATUS_LABELS[service.status]}</span></td>
              <td className="whitespace-nowrap px-3 py-3">{formatEUR(service.priceCents)} / {TERM_LABELS[service.paymentTerm]}</td>
              <td className={`whitespace-nowrap px-3 py-3 ${service.daysLeft < 0 ? 'text-high-text' : ''}`}>
                <span>{formatDaysLeft(service.daysLeft)}</span>
                <span className="mt-0.5 block text-[12px] text-ink-3">{formatDate(service.expiresAt)}</span>
              </td>
              <td className="whitespace-nowrap px-3 py-3">{service.autoProlong ? 'Включено' : 'Выключено'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function CustomerContent({ customer, thresholds }: {
  customer: CustomerDetail;
  thresholds: { medium: number; high: number };
}) {
  const [copied, setCopied] = useState<'success' | 'error' | null>(null);
  const noScore = customer.scoreStatus === 'no_score';
  const hasScore = customer.riskLevel !== null && customer.riskScore !== null && customer.loyalty !== null && customer.probability !== null;

  async function copyForCrm() {
    const reason = noScore ? noScoreMessage(customer) : customer.contributions.find((contribution) => contribution.points > 0)?.text ?? 'Нет выраженных факторов риска';
    const recommendations = customer.recommendations.map(({ action }, index) => `${index + 1}. ${action}`).join('\n');
    const text = [
      `${customer.email} · клиент #${customer.id}`,
      noScore ? `Оценки риска нет: ${reason}.` : `Риск: ${customer.riskScore}/100 (${levelLabel(customer)}), шанс ухода ${Math.round((customer.probability ?? 0) * 100)}%.`,
      `Главная причина: ${reason}`,
      `Ожидаемые потери: ${formatEUR(customer.money.expectedLossCents)} за ${customer.money.horizonDays} дн.`,
      recommendations ? `Рекомендации:\n${recommendations}` : '',
    ].filter(Boolean).join('\n');

    try {
      await navigator.clipboard.writeText(text);
      setCopied('success');
    } catch {
      setCopied('error');
    }
  }

  return (
    <>
      <section className={`grid gap-6 rounded-hero bg-surface p-6 ${hasScore ? 'lg:grid-cols-[minmax(0,1fr)_minmax(300px,0.85fr)]' : ''}`} aria-label="Сводка по клиенту">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <RiskBadge customer={customer} />
            <span className="font-mono text-[13px] text-ink-2">ID {customer.id}</span>
          </div>
          <h1 className="mt-3 break-all font-display text-[28px] font-semibold tracking-[-0.02em]">{customer.email}</h1>
          <p className="mt-3 max-w-2xl text-[15px] leading-6 text-ink-2">{summaryLine(customer)}</p>
        </div>
        {hasScore ? (
          <div className="rounded-tile-lg bg-surface-2 p-5">
            <div className="flex items-baseline justify-between gap-3">
              <h2 className="text-[14px] font-semibold text-ink-2">Risk-score</h2>
              <p className="font-display text-[24px] font-semibold">{customer.riskScore}<span className="text-[15px] text-ink-3"> / 100</span></p>
            </div>
            <div className="mt-5">
              <ScoreScale
                score={customer.riskScore!}
                loyalty={customer.loyalty!}
                probability={customer.probability!}
                mediumThreshold={thresholds.medium}
                highThreshold={thresholds.high}
              />
            </div>
          </div>
        ) : null}
      </section>

      <section className="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4" aria-label="Деньги клиента">
        <article className="rounded-block bg-surface p-5">
          <p className="text-[13px] text-ink-2">Баланс</p><p className="mt-2 font-display text-[22px] font-semibold">{formatEUR(customer.balanceCents)}</p>
          <p className="mt-1 text-[13px] text-ink-3">Из них к выводу: {formatEUR(customer.withdrawBalanceCents)}</p>
        </article>
        <article className="rounded-block bg-surface p-5">
          <p className="text-[13px] text-ink-2">Бонусы</p><p className="mt-2 font-display text-[22px] font-semibold">{formatEUR(customer.bonusBalanceCents)}</p>
          <p className="mt-1 text-[13px] text-ink-3">{customer.bonusState === 'unlocked' ? 'Можно тратить' : 'Заблокированы'}</p>
        </article>
        <article className="rounded-block bg-surface p-5">
          <p className="text-[13px] text-ink-2">Пополнено за всё время</p><p className="mt-2 font-display text-[22px] font-semibold">{formatEUR(customer.totalReplenishedCents)}</p>
        </article>
        <article className="rounded-block bg-surface p-5">
          <p className="text-[13px] text-ink-2">Приносит в месяц</p><p className="mt-2 font-display text-[22px] font-semibold">{formatEUR(customer.money.mrrCents)}</p>
        </article>
      </section>

      <div className="mt-6 grid items-start gap-6 xl:grid-cols-[minmax(0,1.4fr)_minmax(340px,0.9fr)]">
        {noScore ? (
          <div className="xl:col-span-2"><EmptyState title="Оценки нет" hint={noScoreMessage(customer)} /></div>
        ) : null}
        <div className="space-y-6">
          {!noScore && customer.riskScore !== null && customer.riskLevel !== null ? (
            <ScoreWaterfall contributions={customer.contributions} score={customer.riskScore} level={customer.riskLevel} />
          ) : null}

          <section className="rounded-block bg-surface p-6" aria-labelledby="services-title">
            <SectionTitle><span id="services-title">Услуги</span></SectionTitle>
            <ServiceTable services={customer.services} />
          </section>

          <section className="rounded-block bg-surface p-6" aria-labelledby="payments-title">
            <SectionTitle><span id="payments-title">Платежи</span></SectionTitle>
            <div className="mt-4"><PaymentStrip transactions={customer.transactions} /></div>
          </section>

          <section className="rounded-block bg-surface p-6" aria-labelledby="history-title">
            <SectionTitle><span id="history-title">История статусов</span></SectionTitle>
            {customer.statusHistory.length ? (
              <ol className="mt-4 space-y-4">
                {customer.statusHistory.map((event) => (
                  <li key={event.id} className="flex gap-3 border-l-2 border-line pl-4">
                    <div className="min-w-0 flex-1">
                      <p className="text-[14px] font-semibold">Услуга #{event.serviceId}: {event.fromStatus ? STATUS_LABELS[event.fromStatus] : 'создана'} → {STATUS_LABELS[event.toStatus]}</p>
                      <p className="mt-1 text-[13px] text-ink-2">{SOURCE_LABELS[event.source]} · {formatDate(event.occurredAt)}</p>
                    </div>
                    <span className="shrink-0 text-[12px] text-ink-3">{formatRelative(event.occurredAt)}</span>
                  </li>
                ))}
              </ol>
            ) : <p className="mt-4 text-[14px] text-ink-2">История изменений пока пуста.</p>}
          </section>
        </div>

        <aside className="space-y-6">
          {!noScore && customer.probability !== null ? (
            <MoneySplit money={customer.money} probability={customer.probability} />
          ) : null}

          <section className="rounded-block bg-surface p-6" aria-labelledby="recommendations-title">
            <SectionTitle><span id="recommendations-title">Что сделать</span></SectionTitle>
            {customer.recommendations.length ? (
              <ol className="mt-4 space-y-3">
                {customer.recommendations.map(({ feature, action }, index) => (
                  <li key={`${feature}-${index}`} className="flex gap-3 text-[14px] leading-6">
                    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-accent-soft text-[13px] font-semibold text-accent-text">{index + 1}</span>
                    <span>{action}</span>
                  </li>
                ))}
              </ol>
            ) : <p className="mt-4 text-[14px] text-ink-2">Нет рекомендаций для этого клиента.</p>}
            <button type="button" onClick={() => void copyForCrm()} className="mt-5 inline-flex h-[46px] items-center gap-2 rounded-field bg-accent px-5 font-semibold text-accent-ink transition-colors hover:opacity-90">
              {copied === 'success' ? <Check size={18} aria-hidden="true" /> : <Copy size={18} aria-hidden="true" />}
              {copied === 'success' ? 'Скопировано' : 'Скопировать для CRM'}
            </button>
            <p aria-live="polite" className="mt-2 min-h-5 text-[13px] text-ink-2">{copied === 'error' ? 'Не удалось скопировать. Проверьте разрешение буфера обмена.' : ''}</p>
          </section>
        </aside>
      </div>
    </>
  );
}

export default function Customer() {
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const { horizonDays } = useHorizon();
  const customerId = Number(id);
  const validId = Number.isInteger(customerId) && customerId > 0;
  const customerQuery = useQuery({
    queryKey: ['customer', customerId, horizonDays],
    enabled: validId,
    queryFn: () => unwrap(api.GET('/api/v1/customers/{id}', {
      params: { path: { id: customerId }, query: { horizonDays } },
    })),
  });
  const settingsQuery = useQuery({
    queryKey: ['model-settings'],
    queryFn: () => unwrap(api.GET('/api/v1/model/settings')),
    staleTime: 60_000,
  });
  const thresholds = settingsQuery.data?.thresholds ?? { medium: 30, high: 60 };

  return (
    <>
      <Link
        to={{ pathname: '/customers', search: searchParams.toString() ? `?${searchParams.toString()}` : '' }}
        className="mb-4 inline-flex min-h-10 items-center gap-2 rounded-lg px-2 text-[14px] font-semibold text-accent-text hover:bg-surface-2"
      >
        <ArrowLeft size={18} aria-hidden="true" /> Все клиенты
      </Link>
      {!validId ? (
        <ErrorState title="Клиент не найден" message="В адресе указан некорректный ID клиента." />
      ) : customerQuery.data ? (
        <CustomerContent customer={customerQuery.data} thresholds={thresholds} />
      ) : customerQuery.isPending ? (
        <CustomerSkeleton />
      ) : customerQuery.isError ? (
        <ErrorState
          title={customerQuery.error instanceof ApiError && customerQuery.error.status === 404 ? 'Клиент не найден' : undefined}
          message={customerQuery.error instanceof Error ? customerQuery.error.message : undefined}
          onRetry={() => void customerQuery.refetch()}
        />
      ) : null}
    </>
  );
}
