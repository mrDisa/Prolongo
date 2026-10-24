import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, RotateCcw, Save } from 'lucide-react';
import { useBlocker } from 'react-router-dom';
import type { components } from '@/api/schema';
import { api } from '@/api/client';
import { unwrap } from '@/api/errors';
import { ClientsOnScale } from '@/components/ClientsOnScale';
import { EmptyState } from '@/components/EmptyState';
import { ErrorState } from '@/components/ErrorState';
import { FeatureWeightRow } from '@/components/FeatureWeightRow';
import { PageHeader } from '@/components/PageHeader';
import { ScoreWaterfall } from '@/components/ScoreWaterfall';
import { formatEUR } from '@/lib/format';

type Settings = components['schemas']['ModelSettings'];
type SettingsPatch = components['schemas']['ModelSettingsPatch'];
type FeatureSetting = components['schemas']['FeatureSetting'];
type PreviewCustomer = components['schemas']['PreviewCustomer'];
type CustomerListItem = components['schemas']['CustomerListItem'];

interface DraftSettings {
  horizonDays: number;
  thresholds: { medium: number; high: number };
  features: FeatureSetting[];
}

const SETTINGS_KEY = ['model-settings'] as const;
const HORIZONS = [7, 14, 21, 30, 45, 60, 90, 180, 365];

export default function Model() {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<DraftSettings | null>(null);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [toast, setToast] = useState('');

  const settingsQuery = useQuery({
    queryKey: SETTINGS_KEY,
    queryFn: () => unwrap(api.GET('/api/v1/model/settings')),
    staleTime: 60_000,
  });
  const baselineQuery = useQuery({
    queryKey: ['customers', 'model-baseline'],
    queryFn: fetchAllCustomers,
    staleTime: 60_000,
  });

  useEffect(() => {
    if (settingsQuery.data) setDraft(makeDraft(settingsQuery.data));
  }, [settingsQuery.data]);

  const settings = settingsQuery.data;
  const patch = useMemo(() => settings && draft ? makePatch(settings, draft) : null, [settings, draft]);
  const dirtyCount = patch ? countChanges(patch) : 0;
  const isDirty = dirtyCount > 0;
  const valid = draft ? isValidDraft(draft) : false;

  const previewMutation = useMutation({
    mutationFn: (body: SettingsPatch) => unwrap(api.POST('/api/v1/model/preview', { body })),
  });
  const saveMutation = useMutation({
    mutationFn: (body: SettingsPatch) => unwrap(api.PATCH('/api/v1/model/settings', { body })),
    onSuccess: async (saved) => {
      queryClient.setQueryData(SETTINGS_KEY, saved);
      setDraft(makeDraft(saved));
      void queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'me' && query.queryKey[0] !== SETTINGS_KEY[0] });
      try {
        const summary = await unwrap(api.GET('/api/v1/analytics/summary', { params: { query: { horizonDays: saved.horizonDays } } }));
        setToast(`Модель пересчитана · расчёт №${summary.calcVersion}`);
      } catch {
        setToast(`Модель сохранена · версия №${saved.version}`);
      }
    },
  });

  useEffect(() => {
    if (!draft || !settings || !patch || !valid) return;
    const timer = window.setTimeout(() => previewMutation.mutate(patch), isDirty ? 250 : 0);
    return () => window.clearTimeout(timer);
  }, [draft, settings, patch, valid, isDirty, previewMutation.mutate]);

  useEffect(() => {
    if (!isDirty) return;
    const confirmExit = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', confirmExit);
    return () => window.removeEventListener('beforeunload', confirmExit);
  }, [isDirty]);

  const blocker = useBlocker(isDirty && !saveMutation.isPending);
  const preview = previewMutation.data;
  const draftCustomers = preview?.customers ?? [];
  const selectedCustomer = draftCustomers.find(({ id }) => id === selectedId) ?? draftCustomers[0];
  const baselineScores = new Map((baselineQuery.data ?? []).map((customer) => [customer.id, customer.riskScore ?? 0]));
  const featureMetrics = new Map((preview?.features ?? []).map((feature) => [feature.key, feature]));
  const sortedDraftCustomers = [...draftCustomers].sort((a, b) => b.expectedLossCents - a.expectedLossCents);

  function updateFeature(key: FeatureSetting['key'], change: Partial<Pick<FeatureSetting, 'weight' | 'enabled'>>) {
    setToast('');
    setDraft((current) => current ? ({
      ...current,
      features: current.features.map((feature) => feature.key === key ? { ...feature, ...change } : feature),
    }) : current);
  }

  if (settingsQuery.isError && !settingsQuery.data) {
    return <ErrorState message={settingsQuery.error instanceof Error ? settingsQuery.error.message : undefined} onRetry={() => void settingsQuery.refetch()} />;
  }
  if (!settings || !draft) return <ModelSkeleton />;

  const serviceFeatures = draft.features.filter((feature) => feature.scope === 'service');
  const accountFeatures = draft.features.filter((feature) => feature.scope === 'account');
  const selectedContributions = selectedCustomer ? toContributions(selectedCustomer, draft.features) : [];
  const selectedLevel = selectedCustomer?.riskLevel;

  return (
    <>
      <PageHeader
        title="Модель риска"
        subtitle="Двигайте веса — справа сразу видно, как изменятся клиенты и деньги"
        actions={(
          <>
            <button type="button" onClick={() => { setToast(''); setDraft(makeDefaults(settings)); }} disabled={!isDirty} className="inline-flex h-11 items-center gap-2 rounded-field border border-line-strong bg-surface px-4 text-[14px] font-semibold text-ink disabled:cursor-not-allowed disabled:opacity-50">
              <RotateCcw size={16} aria-hidden="true" /> Вернуть стандартные
            </button>
            <button type="button" onClick={() => patch && saveMutation.mutate(patch)} disabled={!isDirty || !valid || saveMutation.isPending} className="inline-flex h-11 items-center gap-2 rounded-field bg-accent px-4 text-[14px] font-semibold text-accent-ink disabled:cursor-not-allowed disabled:opacity-50">
              <Save size={16} aria-hidden="true" /> {saveMutation.isPending ? 'Сохраняем…' : `Сохранить · ${dirtyCount}`}
            </button>
          </>
        )}
      />

      {toast ? <p role="status" className="mb-5 rounded-field border border-low bg-low-soft px-4 py-3 text-[14px] font-semibold text-low-text">{toast}</p> : null}
      {saveMutation.isError ? <div className="mb-5"><ErrorState message={saveMutation.error instanceof Error ? saveMutation.error.message : 'Не удалось сохранить настройки'} onRetry={() => patch && saveMutation.mutate(patch)} /></div> : null}
      {draft && !valid ? <p role="alert" className="mb-5 rounded-field bg-high-soft px-4 py-3 text-[14px] font-medium text-high-text">Проверьте пороги: medium должен быть меньше high, горизонт — от 1 до 365 дней.</p> : null}

      <section className="mb-6 rounded-block bg-surface p-6" aria-labelledby="risk-steps-title">
        <h2 id="risk-steps-title" className="font-display text-[20px] font-semibold">Как считается риск</h2>
        <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-[1fr_auto_1fr_auto_1fr_auto_1fr] xl:items-center">
          <Step number="1" title={`Проверяем ${settings.features.length} признаков`} detail="Каждый получает значение от 0 до 1" />
          <ArrowRight className="hidden text-ink-3 xl:block" aria-hidden="true" />
          <Step number="2" title="Умножаем на вес" detail={`Например: 1 × ${settings.features[0]?.weight ?? 45} = +${settings.features[0]?.weight ?? 45}`} />
          <ArrowRight className="hidden text-ink-3 xl:block" aria-hidden="true" />
          <Step number="3" title="Складываем баллы" detail="Score от 0 до 100 по самой рисковой услуге" />
          <ArrowRight className="hidden text-ink-3 xl:block" aria-hidden="true" />
          <Step number="4" title="Уровень и деньги" detail="Score 72 → шанс ухода 72%" accent />
        </div>
      </section>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(380px,0.9fr)]">
        <div className="space-y-6">
          <section className="rounded-block bg-surface p-6" aria-labelledby="service-features-title">
            <div className="mb-4">
              <h2 id="service-features-title" className="font-display text-[20px] font-semibold">Признаки услуги</h2>
              <p className="mt-1 text-[13px] text-ink-2">Применяются к каждой живой услуге клиента.</p>
            </div>
            <div className="space-y-3">
              {serviceFeatures.map((feature) => (
                <FeatureWeightRow key={feature.key} feature={feature} saved={settings.features.find(({ key }) => key === feature.key) ?? feature} preview={featureMetrics.get(feature.key)} onWeightChange={(weight) => updateFeature(feature.key, { weight })} onEnabledChange={(enabled) => updateFeature(feature.key, { enabled })} />
              ))}
            </div>
          </section>

          <section className="rounded-block bg-surface p-6" aria-labelledby="account-features-title">
            <div className="mb-4">
              <h2 id="account-features-title" className="font-display text-[20px] font-semibold">Признаки аккаунта</h2>
              <p className="mt-1 text-[13px] text-ink-2">Учитываются для клиента целиком и добавляются к услугам.</p>
            </div>
            <div className="space-y-3">
              {accountFeatures.map((feature) => (
                <FeatureWeightRow key={feature.key} feature={feature} saved={settings.features.find(({ key }) => key === feature.key) ?? feature} preview={featureMetrics.get(feature.key)} onWeightChange={(weight) => updateFeature(feature.key, { weight })} onEnabledChange={(enabled) => updateFeature(feature.key, { enabled })} />
              ))}
            </div>
          </section>
        </div>

        <aside className="space-y-6 xl:sticky xl:top-8">
          <section className="rounded-block bg-surface p-6" aria-labelledby="clients-scale-title">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <h2 id="clients-scale-title" className="font-display text-[20px] font-semibold">Где сейчас клиенты</h2>
                <p className="mt-1 text-[13px] text-ink-2">Метки сразу двигаются при изменении весов</p>
              </div>
              {previewMutation.isPending ? <span className="text-[12px] text-ink-3" role="status">Пересчитываем…</span> : null}
            </div>
            {previewMutation.isError && !preview ? (
              <div className="mt-4"><ErrorState message={previewMutation.error instanceof Error ? previewMutation.error.message : undefined} onRetry={() => patch && previewMutation.mutate(patch)} /></div>
            ) : preview ? (
              <div className="mt-4">
                {previewMutation.isError ? <p role="alert" className="mb-3 text-[12px] text-high-text">Не удалось обновить preview; показаны последние рассчитанные значения.</p> : null}
                <ClientsOnScale customers={preview.customers} baselineScores={baselineScores} thresholds={draft.thresholds} selectedId={selectedCustomer?.id ?? null} onSelect={setSelectedId} />
                {baselineQuery.isError ? <p className="mt-2 text-[12px] text-high-text">Не удалось загрузить исходные оценки; показаны только значения черновика.</p> : null}
                <div className="mt-4 grid grid-cols-3 gap-2">
                  <DeltaTile label="Высокий" value={preview.summary.customers.high} baseline={preview.baseline.customers.high} color="high" />
                  <DeltaTile label="Средний" value={preview.summary.customers.medium} baseline={preview.baseline.customers.medium} color="medium" />
                  <DeltaTile label="Низкий" value={preview.summary.customers.low} baseline={preview.baseline.customers.low} color="low" />
                </div>
                <div className="mt-3 rounded-tile-lg bg-surface-2 p-4">
                  <div className="flex flex-wrap items-baseline justify-between gap-2">
                    <span className="text-[13px] font-medium text-ink-2">Ожидаемые потери</span>
                    <span className="font-display text-[20px] font-semibold">{formatEUR(preview.summary.money.expectedLossCents)}</span>
                  </div>
                  <p className="mt-1 text-right text-[12px] text-ink-3">{formatDelta(preview.summary.money.expectedLossCents - preview.baseline.money.expectedLossCents)} к сохранённой модели</p>
                </div>
              </div>
            ) : <ModelPreviewSkeleton />}
          </section>

          <section className="rounded-block bg-surface p-6" aria-labelledby="selected-customer-title">
            <h2 id="selected-customer-title" className="font-display text-[20px] font-semibold">Из чего сложился score</h2>
            {sortedDraftCustomers.length ? (
              <div className="mt-4 flex flex-wrap gap-2" aria-label="Выбрать клиента для лесенки">
                {sortedDraftCustomers.slice(0, 6).map((customer) => (
                  <button key={customer.id} type="button" aria-pressed={selectedCustomer?.id === customer.id} onClick={() => setSelectedId(customer.id)} className={`rounded-full border px-3 py-2 text-[13px] font-semibold ${selectedCustomer?.id === customer.id ? 'border-accent bg-accent-soft text-accent-text' : 'border-line text-ink-2 hover:bg-surface-2'}`}>
                    #{customer.id}
                  </button>
                ))}
              </div>
            ) : null}
            {selectedCustomer && selectedLevel ? (
              <div className="mt-4">
                <p className="mb-3 text-[13px] text-ink-2">#{selectedCustomer.id} · {selectedCustomer.email} · {selectedCustomer.riskiestServiceName}</p>
                <ScoreWaterfall contributions={selectedContributions} score={selectedCustomer.riskScore} level={selectedLevel} />
              </div>
            ) : (
              <div className="mt-4"><EmptyState title="Нет клиентов с оценкой" hint="Лесенка появится, когда preview вернёт клиента со score." /></div>
            )}
          </section>

          <section className="rounded-block bg-surface p-6" aria-labelledby="model-thresholds-title">
            <h2 id="model-thresholds-title" className="font-display text-[20px] font-semibold">Пороги и горизонт</h2>
            <div className="mt-4 grid grid-cols-2 gap-3">
              <NumberField label="Средний риск от" value={draft.thresholds.medium} min={0} max={99} onChange={(medium) => setDraft((current) => current ? { ...current, thresholds: { ...current.thresholds, medium } } : current)} />
              <NumberField label="Высокий риск от" value={draft.thresholds.high} min={1} max={100} onChange={(high) => setDraft((current) => current ? { ...current, thresholds: { ...current.thresholds, high } } : current)} />
            </div>
            <label className="mt-4 block text-[13px] font-medium text-ink-2" htmlFor="model-horizon">Горизонт прогноза</label>
            <select id="model-horizon" value={draft.horizonDays} onChange={(event) => setDraft((current) => current ? { ...current, horizonDays: Number(event.target.value) } : current)} className="mt-2 h-11 w-full rounded-field border border-line-strong bg-surface px-3 text-[14px] text-ink outline-none focus:border-accent">
              {[...new Set([...HORIZONS, draft.horizonDays])].sort((a, b) => a - b).map((days) => <option key={days} value={days}>{days} дней</option>)}
            </select>
          </section>
        </aside>
      </div>

      {blocker.state === 'blocked' ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" role="presentation">
          <section role="alertdialog" aria-modal="true" aria-labelledby="unsaved-title" className="w-full max-w-md rounded-block bg-surface p-6 shadow-2xl">
            <h2 id="unsaved-title" className="font-display text-[20px] font-semibold">Есть несохранённые изменения</h2>
            <p className="mt-2 text-[14px] leading-6 text-ink-2">Если уйти сейчас, черновик модели будет потерян.</p>
            <div className="mt-5 flex justify-end gap-3">
              <button type="button" onClick={() => blocker.reset()} className="h-10 rounded-field border border-line-strong px-4 text-[14px] font-semibold">Остаться</button>
              <button type="button" onClick={() => blocker.proceed()} className="h-10 rounded-field bg-high px-4 text-[14px] font-semibold text-white">Уйти без сохранения</button>
            </div>
          </section>
        </div>
      ) : null}
    </>
  );
}

function Step({ number, title, detail, accent = false }: { number: string; title: string; detail: string; accent?: boolean }) {
  return (
    <div className={`rounded-tile-lg p-4 ${accent ? 'bg-accent-soft' : 'bg-surface-2'}`}>
      <p className="text-[11px] font-semibold text-ink-3">ШАГ {number}</p>
      <p className="mt-2 text-[14px] font-semibold">{title}</p>
      <p className="mt-1 text-[12px] leading-5 text-ink-2">{detail}</p>
    </div>
  );
}

function DeltaTile({ label, value, baseline, color }: { label: string; value: number; baseline: number; color: 'high' | 'medium' | 'low' }) {
  const colorClass = color === 'high' ? 'text-high-text' : color === 'medium' ? 'text-med-text' : 'text-low-text';
  return (
    <div className="rounded-tile-lg bg-surface-2 p-3">
      <p className={`text-[12px] font-semibold ${colorClass}`}>{label}</p>
      <p className="mt-1 font-display text-[22px] font-semibold">{value}</p>
      <p className="mt-1 text-[11px] text-ink-3">{formatDelta(value - baseline)} к стандарту</p>
    </div>
  );
}

function NumberField({ label, value, min, max, onChange }: { label: string; value: number; min: number; max: number; onChange: (value: number) => void }) {
  return (
    <label className="block text-[13px] font-medium text-ink-2">
      {label}
      <input type="number" min={min} max={max} value={Number.isFinite(value) ? value : ''} onChange={(event) => onChange(event.target.value === '' ? Number.NaN : Number(event.target.value))} className="mt-2 h-11 w-full rounded-field border border-line-strong bg-surface px-3 text-[15px] font-semibold text-ink outline-none focus:border-accent" />
    </label>
  );
}

function ModelSkeleton() {
  return <div className="space-y-6" aria-label="Загрузка модели" role="status"><div className="h-28 animate-pulse rounded-block bg-surface" /><div className="grid gap-6 xl:grid-cols-2"><div className="h-[600px] animate-pulse rounded-block bg-surface" /><div className="h-[600px] animate-pulse rounded-block bg-surface" /></div><span className="sr-only">Загружаем настройки модели</span></div>;
}

function ModelPreviewSkeleton() {
  return <div className="mt-4 h-64 animate-pulse rounded-tile-lg bg-surface-2" aria-label="Пересчёт модели" role="status" />;
}

function makeDraft(settings: Settings): DraftSettings {
  return {
    horizonDays: settings.horizonDays,
    thresholds: { ...settings.thresholds },
    features: settings.features.map((feature) => ({ ...feature })),
  };
}

function makeDefaults(settings: Settings): DraftSettings {
  return {
    horizonDays: 30,
    thresholds: { medium: 30, high: 60 },
    features: settings.features.map((feature) => ({ ...feature, weight: feature.defaultWeight, enabled: true })),
  };
}

function makePatch(settings: Settings, draft: DraftSettings): SettingsPatch {
  const patch: SettingsPatch = {};
  if (draft.horizonDays !== settings.horizonDays) patch.horizonDays = draft.horizonDays;
  const thresholds: NonNullable<SettingsPatch['thresholds']> = {};
  if (draft.thresholds.medium !== settings.thresholds.medium) thresholds.medium = draft.thresholds.medium;
  if (draft.thresholds.high !== settings.thresholds.high) thresholds.high = draft.thresholds.high;
  if (Object.keys(thresholds).length) patch.thresholds = thresholds;

  const featureChanges: NonNullable<SettingsPatch['features']> = {};
  for (const feature of draft.features) {
    const saved = settings.features.find(({ key }) => key === feature.key);
    if (!saved) continue;
    const change: { weight?: number; enabled?: boolean } = {};
    if (feature.weight !== saved.weight) change.weight = feature.weight;
    if (feature.enabled !== saved.enabled) change.enabled = feature.enabled;
    if (Object.keys(change).length) featureChanges[feature.key] = change;
  }
  if (Object.keys(featureChanges).length) patch.features = featureChanges;
  return patch;
}

function countChanges(patch: SettingsPatch): number {
  let count = Number(patch.horizonDays !== undefined);
  count += Number(patch.thresholds?.medium !== undefined) + Number(patch.thresholds?.high !== undefined);
  for (const changes of Object.values(patch.features ?? {})) {
    count += Number(changes.weight !== undefined) + Number(changes.enabled !== undefined);
  }
  return count;
}

function isValidDraft(draft: DraftSettings): boolean {
  return Number.isInteger(draft.horizonDays) && draft.horizonDays >= 1 && draft.horizonDays <= 365
    && Number.isFinite(draft.thresholds.medium) && Number.isFinite(draft.thresholds.high)
    && draft.thresholds.medium >= 0 && draft.thresholds.medium < draft.thresholds.high && draft.thresholds.high <= 100;
}

function formatDelta(value: number): string {
  return value > 0 ? `+${value}` : String(value);
}

async function fetchAllCustomers(): Promise<CustomerListItem[]> {
  const all: CustomerListItem[] = [];
  let page = 1;
  let total = Number.POSITIVE_INFINITY;
  while (all.length < total) {
    const response = await unwrap(api.GET('/api/v1/customers', { params: { query: { page, pageSize: 100, sort: 'riskScore', order: 'desc' } } }));
    all.push(...response.items);
    total = response.total;
    if (response.items.length === 0) break;
    page += 1;
  }
  return all;
}

function toContributions(customer: PreviewCustomer, features: FeatureSetting[]): components['schemas']['Contribution'][] {
  return customer.contributions.map((contribution) => {
    const feature = features.find(({ key }) => key === contribution.feature);
    const weight = feature?.weight ?? contribution.points;
    return {
      feature: contribution.feature,
      title: feature?.title ?? contribution.feature,
      scope: feature?.scope ?? 'service',
      serviceId: null,
      value: weight ? contribution.points / weight : 0,
      weight,
      points: contribution.points,
      text: `${customer.riskiestServiceName}: ${feature?.title ?? contribution.feature}`,
    };
  });
}
