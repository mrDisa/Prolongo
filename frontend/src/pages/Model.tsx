import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, RotateCcw, Save } from 'lucide-react';
import { useBlocker } from 'react-router-dom';
import { MODEL_SETTINGS_KEY, useModelSettings } from '@/api/queries';
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
import { useLiveUpdates } from '@/live/useLiveUpdates';

type Settings = components['schemas']['ModelSettings'];
type SettingsPatch = components['schemas']['ModelSettingsPatch'];
type FeatureSetting = components['schemas']['FeatureSetting'];
type PreviewCustomer = components['schemas']['PreviewCustomer'];

interface DraftSettings {
  horizonDays: number;
  thresholds: { medium: number; high: number };
  features: FeatureSetting[];
}

const HORIZONS = [7, 14, 21, 30, 45, 60, 90, 180, 365];

export default function Model() {
  const queryClient = useQueryClient();
  const { calculationRevision } = useLiveUpdates();
  const [draft, setDraft] = useState<DraftSettings | null>(null);
  const [baseSettings, setBaseSettings] = useState<Settings | null>(null);
  const [remoteSettingsChanged, setRemoteSettingsChanged] = useState(false);
  const [resetConfirmationOpen, setResetConfirmationOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [toast, setToast] = useState('');
  const baseSettingsRef = useRef<Settings | null>(null);
  const draftRef = useRef<DraftSettings | null>(null);
  draftRef.current = draft;

  const settingsQuery = useModelSettings();

  useEffect(() => {
    const next = settingsQuery.data;
    if (!next || baseSettingsRef.current === next) return;

    const previous = baseSettingsRef.current;
    if (!previous) {
      baseSettingsRef.current = next;
      setBaseSettings(next);
      setDraft(makeDraft(next));
      setRemoteSettingsChanged(false);
      return;
    }

    if (previous.version === next.version) {
      baseSettingsRef.current = next;
      return;
    }

    const currentDraft = draftRef.current;
    const localChanges = currentDraft ? makePatch(previous, currentDraft) : {};
    const hasLocalChanges = countChanges(localChanges) > 0;
    baseSettingsRef.current = next;
    setBaseSettings(next);
    if (hasLocalChanges) {
      setDraft(rebaseDraft(next, localChanges));
      setRemoteSettingsChanged(true);
    } else {
      setDraft(makeDraft(next));
      setRemoteSettingsChanged(false);
    }
  }, [settingsQuery.data]);

  const settings = baseSettings;
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
      baseSettingsRef.current = saved;
      setBaseSettings(saved);
      setDraft(makeDraft(saved));
      setRemoteSettingsChanged(false);
      queryClient.setQueryData(MODEL_SETTINGS_KEY, saved);
      void queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'me' && query.queryKey[0] !== MODEL_SETTINGS_KEY[0] });
      try {
        const summary = await unwrap(api.GET('/api/v1/analytics/summary', { params: { query: { horizonDays: saved.horizonDays } } }));
        setToast(`Модель пересчитана · расчёт №${summary.calcVersion}`);
      } catch {
        setToast(`Модель сохранена · версия №${saved.version}`);
      }
    },
  });
  const resetMutation = useMutation({
    mutationFn: () => unwrap(api.POST('/api/v1/model/settings/reset')),
    onSuccess: async (reset) => {
      baseSettingsRef.current = reset;
      setBaseSettings(reset);
      setDraft(makeDraft(reset));
      setRemoteSettingsChanged(false);
      setToast('');
      queryClient.setQueryData(MODEL_SETTINGS_KEY, reset);
      void queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'me' && query.queryKey[0] !== MODEL_SETTINGS_KEY[0] });
      try {
        const summary = await unwrap(api.GET('/api/v1/analytics/summary', { params: { query: { horizonDays: reset.horizonDays } } }));
        setToast(`Стандартные настройки восстановлены · расчёт №${summary.calcVersion}`);
      } catch {
        setToast(`Стандартные настройки восстановлены · версия №${reset.version}`);
      }
    },
  });

  useEffect(() => {
    if (!draft || !settings || !patch || !valid) return;
    const timer = window.setTimeout(() => previewMutation.mutate(patch), isDirty ? 250 : 0);
    return () => window.clearTimeout(timer);
  }, [draft, settings, patch, valid, isDirty, calculationRevision, previewMutation.mutate]);

  useEffect(() => {
    if (!isDirty) return;
    const confirmExit = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', confirmExit);
    return () => window.removeEventListener('beforeunload', confirmExit);
  }, [isDirty]);

  const blocker = useBlocker(isDirty && !saveMutation.isPending && !resetMutation.isPending);
  const preview = previewMutation.data;
  const draftCustomers = preview?.customers ?? [];
  const featureMetrics = new Map((preview?.features ?? []).map((feature) => [feature.key, feature]));
  const sortedDraftCustomers = [...draftCustomers].sort((a, b) => b.expectedLossCents - a.expectedLossCents);
  const topDraftCustomers = sortedDraftCustomers.slice(0, 6);
  const selectedCustomer = topDraftCustomers.find(({ id }) => id === selectedId) ?? topDraftCustomers[0];

  function updateFeature(key: FeatureSetting['key'], change: Partial<Pick<FeatureSetting, 'weight' | 'enabled'>>) {
    setToast('');
    setDraft((current) => current ? ({
      ...current,
      features: current.features.map((feature) => feature.key === key ? { ...feature, ...change } : feature),
    }) : current);
  }

  function resetToDefaults() {
    setResetConfirmationOpen(true);
  }

  function confirmReset() {
    setResetConfirmationOpen(false);
    setToast('');
    resetMutation.mutate();
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
            <button type="button" onClick={resetToDefaults} disabled={resetMutation.isPending || saveMutation.isPending} className="inline-flex h-11 items-center gap-2 rounded-field border border-line-strong bg-surface px-4 text-[14px] font-semibold text-ink disabled:cursor-not-allowed disabled:opacity-50">
              <RotateCcw size={16} aria-hidden="true" /> {resetMutation.isPending ? 'Сбрасываем…' : 'Вернуть стандартные'}
            </button>
            <button type="button" onClick={() => patch && saveMutation.mutate(patch)} disabled={!isDirty || !valid || saveMutation.isPending} className="inline-flex h-11 items-center gap-2 rounded-field bg-accent px-4 text-[14px] font-semibold text-accent-ink disabled:cursor-not-allowed disabled:opacity-50">
              <Save size={16} aria-hidden="true" /> {saveMutation.isPending ? 'Сохраняем…' : `Сохранить · ${dirtyCount}`}
            </button>
          </>
        )}
      />

      {toast ? <p role="status" className="mb-5 rounded-field border border-low bg-low-soft px-4 py-3 text-[14px] font-semibold text-low-text">{toast}</p> : null}
      {saveMutation.isError ? <div className="mb-5"><ErrorState message={saveMutation.error instanceof Error ? saveMutation.error.message : 'Не удалось сохранить настройки'} onRetry={() => patch && saveMutation.mutate(patch)} /></div> : null}
      {resetMutation.isError ? <div className="mb-5"><ErrorState message={resetMutation.error instanceof Error ? resetMutation.error.message : 'Не удалось вернуть стандартные настройки'} onRetry={() => resetMutation.mutate()} /></div> : null}
      {remoteSettingsChanged && isDirty ? <p role="status" className="mb-5 rounded-field border border-med bg-med-soft px-4 py-3 text-[14px] font-medium text-med-text">Настройки на сервере изменились. Черновик обновлён поверх новых значений, ваши правки сохранены в нём.</p> : null}
      {draft && !valid ? <p role="alert" className="mb-5 rounded-field bg-high-soft px-4 py-3 text-[14px] font-medium text-high-text">Проверьте значения: пороги должны быть целыми от 1 до 100, medium меньше high, горизонт — целым числом от 1 до 365 дней.</p> : null}

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
                <ClientsOnScale customers={preview.customers} thresholds={draft.thresholds} selectedId={selectedCustomer?.id ?? null} onSelect={setSelectedId} />
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
                {topDraftCustomers.map((customer) => (
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
              <NumberField label="Средний риск от" value={draft.thresholds.medium} min={1} max={99} onChange={(medium) => setDraft((current) => current ? { ...current, thresholds: { ...current.thresholds, medium } } : current)} />
              <NumberField label="Высокий риск от" value={draft.thresholds.high} min={1} max={100} onChange={(high) => setDraft((current) => current ? { ...current, thresholds: { ...current.thresholds, high } } : current)} />
            </div>
            <label className="mt-4 block text-[13px] font-medium text-ink-2" htmlFor="model-horizon">Горизонт прогноза</label>
            <select id="model-horizon" value={draft.horizonDays} onChange={(event) => setDraft((current) => current ? { ...current, horizonDays: Number(event.target.value) } : current)} className="mt-2 h-11 w-full rounded-field border border-line-strong bg-surface px-3 text-[14px] text-ink outline-none focus:border-accent">
              {[...new Set([...HORIZONS, draft.horizonDays])].sort((a, b) => a - b).map((days) => <option key={days} value={days}>{days} дней</option>)}
            </select>
          </section>
        </aside>
      </div>

      {blocker.state === 'blocked' || resetConfirmationOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" role="presentation">
          <section role="alertdialog" aria-modal="true" aria-labelledby="unsaved-title" className="w-full max-w-md rounded-block bg-surface p-6 shadow-2xl">
            <h2 id="unsaved-title" className="font-display text-[20px] font-semibold">
              {resetConfirmationOpen ? 'Вернуть стандартные настройки?' : 'Есть несохранённые изменения'}
            </h2>
            <p className="mt-2 text-[14px] leading-6 text-ink-2">
              {resetConfirmationOpen
                ? 'Сохранённые настройки модели будут сброшены на сервере, пересчёт запустится сразу и обновление получат все открытые вкладки.'
                : 'Если уйти сейчас, черновик модели будет потерян.'}
            </p>
            <div className="mt-5 flex justify-end gap-3">
              <button type="button" onClick={() => resetConfirmationOpen ? setResetConfirmationOpen(false) : blocker.reset?.()} className="h-10 rounded-field border border-line-strong px-4 text-[14px] font-semibold">
                {resetConfirmationOpen ? 'Отмена' : 'Остаться'}
              </button>
              <button type="button" onClick={() => resetConfirmationOpen ? confirmReset() : blocker.proceed?.()} className="h-10 rounded-field bg-high px-4 text-[14px] font-semibold text-white">
                {resetConfirmationOpen ? 'Сбросить настройки' : 'Уйти без сохранения'}
              </button>
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
      <p className="mt-1 text-[11px] text-ink-3">{formatDelta(value - baseline)} к сохранённой модели</p>
    </div>
  );
}

function NumberField({ label, value, min, max, onChange }: { label: string; value: number; min: number; max: number; onChange: (value: number) => void }) {
  return (
    <label className="block text-[13px] font-medium text-ink-2">
      {label}
      <input type="number" min={min} max={max} step={1} value={Number.isFinite(value) ? value : ''} onChange={(event) => onChange(event.target.value === '' ? Number.NaN : Number(event.target.value))} className="mt-2 h-11 w-full rounded-field border border-line-strong bg-surface px-3 text-[15px] font-semibold text-ink outline-none focus:border-accent" />
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

function rebaseDraft(settings: Settings, changes: SettingsPatch): DraftSettings {
  const draft = makeDraft(settings);
  if (changes.horizonDays !== undefined) draft.horizonDays = changes.horizonDays;
  if (changes.thresholds?.medium !== undefined) draft.thresholds.medium = changes.thresholds.medium;
  if (changes.thresholds?.high !== undefined) draft.thresholds.high = changes.thresholds.high;
  for (const feature of draft.features) {
    const change = changes.features?.[feature.key];
    if (!change) continue;
    if (change.weight !== undefined) feature.weight = change.weight;
    if (change.enabled !== undefined) feature.enabled = change.enabled;
  }
  return draft;
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
    && Number.isInteger(draft.thresholds.medium) && Number.isInteger(draft.thresholds.high)
    && draft.thresholds.medium >= 1 && draft.thresholds.medium < draft.thresholds.high && draft.thresholds.high <= 100;
}

function formatDelta(value: number): string {
  return value > 0 ? `+${value}` : String(value);
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
