import { useEffect, useMemo, useState } from 'react';
import { useQueries, useQuery } from '@tanstack/react-query';
import { ArrowDown, ArrowDownUp, ArrowUp, ChevronLeft, ChevronRight, Search } from 'lucide-react';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '@/api/client';
import { unwrap } from '@/api/errors';
import { ErrorState } from '@/components/ErrorState';
import { PageHeader } from '@/components/PageHeader';
import { formatEUR } from '@/lib/format';
import type { components } from '@/api/schema';

type Customer = components['schemas']['CustomerListItem'];
type SortKey = 'id' | 'email' | 'balance' | 'totalReplenished' | 'riskScore' | 'expectedLoss';
type RiskLevel = 'high' | 'medium' | 'low' | 'none';

const LEVELS: { value: RiskLevel; label: string }[] = [
  { value: 'high', label: 'Высокий' },
  { value: 'medium', label: 'Средний' },
  { value: 'low', label: 'Низкий' },
  { value: 'none', label: 'Нет оценки' },
];
const SORTABLE: { key: SortKey; label: string }[] = [
  { key: 'id', label: 'ID' },
  { key: 'email', label: 'Email' },
  { key: 'balance', label: 'Баланс' },
  { key: 'totalReplenished', label: 'Пополнено всего' },
  { key: 'riskScore', label: 'Risk-score' },
  { key: 'expectedLoss', label: 'Ожид. потери' },
];
const LEVEL_STYLES: Record<Exclude<RiskLevel, 'none'>, { badge: string; dot: string }> = {
  high: { badge: 'bg-high-soft text-high-text', dot: 'bg-high' },
  medium: { badge: 'bg-med-soft text-med-text', dot: 'bg-med' },
  low: { badge: 'bg-low-soft text-low-text', dot: 'bg-low' },
};

function positiveInt(value: string | null, fallback: number): number {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function isRiskLevel(value: string | null): value is RiskLevel {
  return value === 'high' || value === 'medium' || value === 'low' || value === 'none';
}

function levelLabel(customer: Customer): string {
  if (customer.scoreStatus === 'no_score') return 'Нет оценки';
  if (customer.riskLevel === 'high') return 'Высокий';
  if (customer.riskLevel === 'medium') return 'Средний';
  return 'Низкий';
}

function mainReason(customer: Customer): string {
  if (customer.scoreStatus === 'no_score') {
    return customer.noScoreReason === 'blocked'
      ? 'Все услуги заблокированы'
      : 'Клиент уже ушёл — все услуги удалены';
  }
  return customer.topReasons[0]?.text ?? 'Причин для беспокойства нет';
}

function SortHeader({
  column,
  label,
  active,
  order,
  onSort,
}: {
  column: (typeof SORTABLE)[number];
  label: string;
  active: boolean;
  order: 'asc' | 'desc';
  onSort: (key: SortKey) => void;
}) {
  const ariaSort = active ? (order === 'asc' ? 'ascending' : 'descending') : 'none';
  const Icon = active ? (order === 'asc' ? ArrowUp : ArrowDown) : ArrowDownUp;
  return (
    <th scope="col" aria-sort={ariaSort} className="px-4 py-3 text-left font-medium">
      <button
        type="button"
        onClick={() => onSort(column.key)}
        className={`inline-flex min-h-9 items-center gap-2 rounded-lg px-2 text-left font-semibold transition-colors hover:bg-surface-2 ${active ? 'bg-accent-soft text-accent-text' : 'text-ink-2'}`}
      >
        {label}
        <Icon size={15} strokeWidth={2} aria-hidden="true" />
      </button>
    </th>
  );
}

function RiskBadge({ customer }: { customer: Customer }) {
  const isUnrated = customer.scoreStatus === 'no_score';
  const colors = isUnrated ? { badge: 'bg-unrated-soft text-unrated-text', dot: 'bg-unrated' } : LEVEL_STYLES[customer.riskLevel ?? 'low'];
  return (
    <span className={`inline-flex items-center gap-2 whitespace-nowrap rounded-full px-3 py-1 text-[13px] font-semibold leading-[18px] ${colors.badge}`}>
      <span aria-hidden="true" className={`h-2 w-2 rounded-full ${colors.dot}`} />
      {levelLabel(customer)}
    </span>
  );
}

function ScoreBar({ customer }: { customer: Customer }) {
  if (customer.riskScore === null) return <span className="font-display text-[16px] text-ink-3">—</span>;
  const color = customer.riskLevel === 'high' ? 'bg-high' : customer.riskLevel === 'medium' ? 'bg-med' : 'bg-low';
  return (
    <div className="flex min-w-[112px] items-center gap-3">
      <span className="w-7 text-right font-display text-[16px] font-semibold">{customer.riskScore}</span>
      <span className="h-2 w-[72px] overflow-hidden rounded-full bg-track" aria-hidden="true">
        <span className={`block h-full rounded-full ${color}`} style={{ width: `${customer.riskScore}%` }} />
      </span>
    </div>
  );
}

export default function Customers() {
  const [searchParams, setSearchParams] = useSearchParams();
  const sortParam = searchParams.get('sort');
  const sort: SortKey = SORTABLE.some(({ key }) => key === sortParam) ? (sortParam as SortKey) : 'expectedLoss';
  const order: 'asc' | 'desc' = searchParams.get('order') === 'asc' ? 'asc' : 'desc';
  const page = positiveInt(searchParams.get('page'), 1);
  const pageSizeValue = positiveInt(searchParams.get('pageSize'), 25);
  const pageSize = [25, 50, 100].includes(pageSizeValue) ? pageSizeValue : 25;
  const horizonDays = Math.min(365, Math.max(1, positiveInt(searchParams.get('h'), 30)));
  const selectedLevels = useMemo(
    () => (searchParams.get('level') ?? '').split(',').filter(isRiskLevel),
    [searchParams],
  );
  const urlSearch = searchParams.get('search') ?? '';
  const [searchInput, setSearchInput] = useState(urlSearch);

  useEffect(() => setSearchInput(urlSearch), [urlSearch]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const value = searchInput.trim();
      if (value === urlSearch) return;
      const next = new URLSearchParams(searchParams);
      if (value) next.set('search', value);
      else next.delete('search');
      next.delete('page');
      setSearchParams(next, { replace: true });
    }, 300);
    return () => window.clearTimeout(timer);
  }, [searchInput, searchParams, setSearchParams, urlSearch]);

  const query = {
    sort,
    order,
    ...(selectedLevels.length ? { level: selectedLevels.join(',') } : {}),
    ...(urlSearch ? { search: urlSearch } : {}),
    page,
    pageSize,
    horizonDays,
  };
  const customersQuery = useQuery({
    queryKey: ['customers', query],
    queryFn: () => unwrap(api.GET('/api/v1/customers', { params: { query } })),
    placeholderData: (previous) => previous,
  });
  const countQueries = useQueries({
    queries: [null, ...LEVELS.map(({ value }) => value)].map((level) => ({
      queryKey: ['customers-count', level, urlSearch, horizonDays],
      queryFn: () => unwrap(api.GET('/api/v1/customers', {
        params: { query: { page: 1, pageSize: 1, horizonDays, ...(level ? { level } : {}), ...(urlSearch ? { search: urlSearch } : {}) } },
      })),
    })),
  });

  function updateParams(values: Record<string, string | null>, resetPage = false) {
    const next = new URLSearchParams(searchParams);
    Object.entries(values).forEach(([key, value]) => value === null ? next.delete(key) : next.set(key, value));
    if (resetPage) next.delete('page');
    setSearchParams(next);
  }

  function selectLevel(value: RiskLevel | 'all') {
    const nextLevels = value === 'all'
      ? []
      : selectedLevels.includes(value)
        ? selectedLevels.filter((level) => level !== value)
        : [...selectedLevels, value];
    updateParams({ level: nextLevels.length ? nextLevels.join(',') : null }, true);
  }

  function sortBy(key: SortKey) {
    const nextOrder = key === sort ? (order === 'asc' ? 'desc' : 'asc') : (key === 'id' || key === 'email' ? 'asc' : 'desc');
    updateParams({ sort: key, order: nextOrder }, true);
  }

  const response = customersQuery.data;
  const totalPages = response ? Math.max(1, Math.ceil(response.total / response.pageSize)) : 1;
  const firstResult = response?.total ? (response.page - 1) * response.pageSize + 1 : 0;
  const lastResult = response ? Math.min(response.page * response.pageSize, response.total) : 0;

  if (customersQuery.isError) {
    return <ErrorState message={customersQuery.error instanceof Error ? customersQuery.error.message : undefined} onRetry={() => customersQuery.refetch()} />;
  }

  return (
    <>
      <PageHeader
        title="Клиенты"
        subtitle="Рейтинг риска оттока. Нажмите на заголовок колонки, чтобы отсортировать"
        actions={
          <label className="flex h-[46px] w-full items-center gap-3 rounded-field border border-line-strong bg-surface px-4 text-ink-2 sm:w-[320px]">
            <Search size={18} aria-hidden="true" />
            <span className="sr-only">Поиск по email или ID</span>
            <input
              type="search"
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder="Поиск по email или ID"
              className="min-w-0 flex-1 bg-transparent text-[15px] text-ink outline-none placeholder:text-ink-3"
            />
          </label>
        }
      />

      <section aria-label="Фильтр по уровню риска" className="mb-5 flex flex-wrap gap-2">
        <button
          type="button"
          aria-pressed={selectedLevels.length === 0}
          onClick={() => selectLevel('all')}
          className={`h-10 rounded-full border px-4 text-[14px] font-semibold transition-colors ${selectedLevels.length === 0 ? 'border-accent bg-accent-soft text-accent-text' : 'border-line bg-surface text-ink-2 hover:bg-surface-2'}`}
        >
          Все <span className="ml-1 opacity-75">· {countQueries[0]?.data?.total ?? '—'}</span>
        </button>
        {LEVELS.map(({ value, label }, index) => {
          const active = selectedLevels.includes(value);
          return (
            <button
              key={value}
              type="button"
              aria-pressed={active}
              onClick={() => selectLevel(value)}
              className={`h-10 rounded-full border px-4 text-[14px] font-semibold transition-colors ${active ? 'border-accent bg-accent-soft text-accent-text' : 'border-line bg-surface text-ink-2 hover:bg-surface-2'}`}
            >
              {label} <span className="ml-1 opacity-75">· {countQueries[index + 1]?.data?.total ?? '—'}</span>
            </button>
          );
        })}
      </section>

      <section className="overflow-hidden rounded-block bg-surface" aria-label="Рейтинг клиентов">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[1120px] border-collapse text-[14px]">
            <thead className="border-b border-line bg-surface-2/60 text-ink-2">
              <tr>
                {SORTABLE.map((column) => (
                  <SortHeader
                    key={column.key}
                    column={column}
                    label={column.key === 'expectedLoss' ? `${column.label} · ${horizonDays} дн.` : column.label}
                    active={sort === column.key}
                    order={order}
                    onSort={sortBy}
                  />
                ))}
                <th scope="col" className="px-4 py-3 text-left font-medium">Уровень</th>
                <th scope="col" className="px-4 py-3 text-left font-medium">Главная причина</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {customersQuery.isPending ? (
                Array.from({ length: 5 }, (_, index) => (
                  <tr key={`skeleton-${index}`} aria-hidden="true">
                    {Array.from({ length: 8 }, (_, cell) => <td key={cell} className="px-4 py-5"><span className="block h-4 animate-pulse rounded bg-surface-2" /></td>)}
                  </tr>
                ))
              ) : response?.items.length ? response.items.map((customer) => (
                <tr key={customer.id} className="transition-colors hover:bg-surface-2/50">
                  <td className="whitespace-nowrap px-6 py-[18px] font-mono text-[13px] text-ink-2">{customer.id}</td>
                  <td className="whitespace-nowrap px-4 py-[18px]">
                    <Link to={`/customers/${customer.id}`} className="font-semibold text-accent-text hover:underline">{customer.email}</Link>
                  </td>
                  <td className="whitespace-nowrap px-4 py-[18px] font-semibold">{formatEUR(customer.balanceCents)}</td>
                  <td className="whitespace-nowrap px-4 py-[18px]">{formatEUR(customer.totalReplenishedCents)}</td>
                  <td className="px-4 py-[18px]"><ScoreBar customer={customer} /></td>
                  <td className="px-4 py-[18px]"><RiskBadge customer={customer} /></td>
                  <td className="whitespace-nowrap px-4 py-[18px] font-semibold">
                    {customer.scoreStatus === 'no_score' ? '—' : formatEUR(customer.expectedLossCents)}
                    {customer.scoreStatus !== 'no_score' ? <span className="ml-1 text-[12px] font-medium text-ink-3">/ {horizonDays} дн.</span> : null}
                  </td>
                  <td className="max-w-[300px] px-4 py-[18px] text-ink-2"><span className="line-clamp-2">{mainReason(customer)}</span></td>
                </tr>
              )) : (
                <tr><td colSpan={8} className="px-6 py-14 text-center">
                  <p className="font-display text-[18px] font-semibold">Клиенты не найдены</p>
                  <p className="mt-1 text-ink-2">Измените поиск или фильтр по уровню риска.</p>
                </td></tr>
              )}
            </tbody>
          </table>
        </div>

        <footer className="flex flex-wrap items-center justify-between gap-4 border-t border-line px-5 py-4 text-[13px] text-ink-2">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            <span>Показано {firstResult}–{lastResult} из {response?.total ?? '—'}</span>
            <span aria-hidden="true">·</span>
            <span>
              Сортировка: {sort === 'expectedLoss' ? 'ожидаемые потери' : SORTABLE.find(({ key }) => key === sort)?.label.toLocaleLowerCase('ru-RU')} {order === 'asc' ? '↑' : '↓'}
            </span>
            <span aria-hidden="true">·</span>
            <label className="flex items-center gap-2">
              <span>Строк на странице</span>
              <select
                aria-label="Строк на странице"
                value={pageSize}
                onChange={(event) => updateParams({ pageSize: event.target.value }, true)}
                className="h-9 rounded-seg border border-line-strong bg-surface px-2 text-ink"
              >
                {[25, 50, 100].map((size) => <option key={size} value={size}>{size}</option>)}
              </select>
            </label>
          </div>
          <div className="flex items-center gap-3">
            <span>Страница {response?.page ?? page} из {totalPages}</span>
            <button
              type="button"
              aria-label="Предыдущая страница"
              disabled={page <= 1 || customersQuery.isFetching}
              onClick={() => updateParams({ page: String(page - 1) })}
              className="flex h-9 w-9 items-center justify-center rounded-seg border border-line-strong bg-surface hover:bg-surface-2 disabled:cursor-not-allowed disabled:opacity-40"
            ><ChevronLeft size={18} aria-hidden="true" /></button>
            <button
              type="button"
              aria-label="Следующая страница"
              disabled={page >= totalPages || customersQuery.isFetching}
              onClick={() => updateParams({ page: String(page + 1) })}
              className="flex h-9 w-9 items-center justify-center rounded-seg border border-line-strong bg-surface hover:bg-surface-2 disabled:cursor-not-allowed disabled:opacity-40"
            ><ChevronRight size={18} aria-hidden="true" /></button>
          </div>
        </footer>
      </section>
    </>
  );
}
