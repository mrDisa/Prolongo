import { useRef, useState } from 'react';
import { Activity, ArrowDownToLine, ExternalLink, Play, RotateCcw, Square, Upload } from 'lucide-react';
import { Link } from 'react-router-dom';
import { ApiError } from '@/api/errors';
import { useControlSimulator, useUploadSnapshot } from '@/api/mutations';
import { useSimulator, useSourceEvents } from '@/api/queries';
import type { components } from '@/api/schema';
import { EmptyState } from '@/components/EmptyState';
import { ErrorState } from '@/components/ErrorState';
import { LiveDot } from '@/components/LiveDot';
import { PageHeader } from '@/components/PageHeader';
import { formatDate } from '@/lib/format';
import { useLiveUpdates } from '@/live/useLiveUpdates';

type SnapshotResponse = components['schemas']['SnapshotResponse'];
type StatusEvent = components['schemas']['StatusEvent'];
type ServiceStatus = components['schemas']['ServiceStatus'];
type EventSource = components['schemas']['EventSource'];

const STATUS_LABELS: Record<ServiceStatus, string> = {
  active: 'Активна', blocked: 'Заблокирована', deleted: 'Удалена',
  suspended: 'Приостановлена', prolong_wait: 'Ждёт оплаты',
};
const SOURCE_LABELS: Record<EventSource, string> = {
  seed: 'начальные данные', snapshot: 'снимок данных', event: 'событие',
  admin: 'админка', simulator: 'симулятор', sql: 'изменение в БД', demo_backfill: 'демо-история',
};

function dateLabel(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : formatDate(value);
}

function UploadPanel() {
  const inputRef = useRef<HTMLInputElement>(null);
  const upload = useUploadSnapshot();
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const [validationError, setValidationError] = useState('');

  function selectFile(nextFile?: File) {
    upload.reset();
    setValidationError('');
    if (!nextFile) return;
    if (!nextFile.name.toLocaleLowerCase().endsWith('.json')) {
      setFile(null);
      setValidationError('Выберите файл с расширением .json.');
      return;
    }
    if (nextFile.size > 20 * 1024 * 1024) {
      setFile(null);
      setValidationError('Размер файла не должен превышать 20 МБ.');
      return;
    }
    setFile(nextFile);
  }

  function submit() {
    if (file) upload.mutate(file);
  }

  const apiError = upload.error instanceof ApiError ? upload.error : null;
  const details = apiError?.details;
  const snapshot = upload.data as SnapshotResponse | undefined;

  return (
    <section className="rounded-block bg-surface p-5 sm:p-6">
      <div className="flex items-start gap-3">
        <div className="rounded-field bg-accent-soft p-2 text-accent"><Upload aria-hidden="true" size={20} /></div>
        <div><h2 className="font-display text-[20px] font-semibold">Загрузить снимок</h2><p className="mt-1 text-[14px] text-ink-2">Полная замена данных из JSON-файла, до 20 МБ.</p></div>
      </div>
      <div
        className={`mt-5 rounded-tile-lg border border-dashed p-6 text-center transition-colors ${dragging ? 'border-accent bg-accent-soft' : 'border-line'}`}
        onDragOver={(event) => { event.preventDefault(); setDragging(true); }}
        onDragLeave={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragging(false); }}
        onDrop={(event) => { event.preventDefault(); setDragging(false); selectFile(event.dataTransfer.files[0]); }}
      >
        <ArrowDownToLine className="mx-auto text-ink-3" size={24} aria-hidden="true" />
        <p className="mt-2 text-[14px]">Перетащите JSON сюда или выберите файл</p>
        <input ref={inputRef} type="file" accept=".json,application/json" className="sr-only" aria-label="Выбрать JSON-файл" onChange={(event) => { selectFile(event.currentTarget.files?.[0]); event.currentTarget.value = ''; }} />
        <button type="button" onClick={() => inputRef.current?.click()} className="mt-3 rounded-field border border-line px-4 py-2 text-[14px] font-semibold hover:bg-surface-2">Выбрать файл</button>
        {file ? <p className="mt-3 break-all text-[13px] text-ink-2">{file.name} · {(file.size / 1024 / 1024).toFixed(2)} МБ</p> : null}
      </div>
      {validationError ? <p role="alert" className="mt-3 text-[14px] text-high-text">{validationError}</p> : null}
      {apiError ? <div role="alert" className="mt-3 rounded-field bg-high-soft p-3 text-[14px] text-high-text"><p className="font-semibold">{apiError.message}</p>{details ? <ul className="mt-2 space-y-1 font-mono text-[12px]">{Object.entries(details).flatMap(([path, messages]) => messages.map((message, index) => <li key={`${path}-${index}`}>{path}: {message}</li>))}</ul> : null}</div> : null}
      {upload.error && !apiError ? <p role="alert" className="mt-3 text-[14px] text-high-text">Не удалось загрузить файл. Проверьте соединение и попробуйте снова.</p> : null}
      {snapshot ? <div role="status" className="mt-4 rounded-tile-lg bg-low-soft p-4 text-[14px]"><strong>Снимок загружен · пакет #{snapshot.batchId}</strong><p className="mt-1 text-ink-2">Аккаунты: {snapshot.accounts} · услуги: {snapshot.services} · транзакции: {snapshot.transactions} · смен статуса: {snapshot.statusChanges}</p><p className="mt-1 text-[12px] text-ink-3">Удалено записей — аккаунтов: {snapshot.removed.accounts}, услуг: {snapshot.removed.services}, транзакций: {snapshot.removed.transactions}</p></div> : null}
      <button type="button" onClick={submit} disabled={!file || upload.isPending} className="mt-4 h-11 rounded-field bg-accent px-5 text-[14px] font-semibold text-accent-ink disabled:cursor-not-allowed disabled:opacity-50">
        {upload.isPending ? 'Загружаем…' : 'Загрузить снимок'}
      </button>
    </section>
  );
}

function SimulatorPanel() {
  const query = useSimulator();
  const control = useControlSimulator();
  const state = query.data;
  const error = control.error instanceof Error ? control.error.message : '';
  function action(actionName: 'start' | 'tick' | 'stop') {
    control.mutate({ action: actionName });
  }

  return (
    <section className="rounded-block bg-surface p-5 sm:p-6">
      <div className="flex items-start justify-between gap-3"><div className="flex items-start gap-3"><div className="rounded-field bg-accent-soft p-2 text-accent"><Activity aria-hidden="true" size={20} /></div><div><h2 className="font-display text-[20px] font-semibold">Симулятор</h2><p className="mt-1 text-[14px] text-ink-2">Управление временем и событиями источника.</p></div></div><button type="button" onClick={() => query.refetch()} aria-label="Обновить состояние симулятора" className="rounded-field p-2 text-ink-2 hover:bg-surface-2"><RotateCcw size={17} /></button></div>
      {query.isPending ? <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4" aria-label="Загрузка симулятора" role="status">{Array.from({ length: 4 }, (_, index) => <div key={index} className="h-[68px] animate-pulse rounded-field bg-surface-2" />)}<span className="sr-only">Загружаем состояние симулятора</span></div> : query.isError && !state ? <ErrorState title="Не удалось загрузить симулятор" message={query.error.message} onRetry={() => query.refetch()} /> : state ? <>
        <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <InfoCell label="Состояние" value={state.running ? 'Работает' : 'Остановлен'} />
          <InfoCell label="Время источника" value={dateLabel(state.dataTime)} />
          <InfoCell label="Шаг времени" value={`${state.tickHours} ч.`} />
          <InfoCell label="Событий создано" value={String(state.eventsGenerated)} />
        </div>
        {error ? <p role="alert" className="mt-3 text-[14px] text-high-text">{error}</p> : null}
        <div className="mt-4 flex flex-wrap gap-2">
          <button type="button" disabled={control.isPending || state.running} onClick={() => action('start')} className="inline-flex h-10 items-center gap-2 rounded-field bg-accent px-4 text-[14px] font-semibold text-accent-ink disabled:opacity-50"><Play size={16} /> Запустить</button>
          <button type="button" disabled={control.isPending || state.running} onClick={() => action('tick')} className="h-10 rounded-field border border-line px-4 text-[14px] font-semibold hover:bg-surface-2 disabled:opacity-50">Один шаг</button>
          <button type="button" disabled={control.isPending || !state.running} onClick={() => action('stop')} className="inline-flex h-10 items-center gap-2 rounded-field border border-line px-4 text-[14px] font-semibold hover:bg-surface-2 disabled:opacity-50"><Square size={15} /> Остановить</button>
        </div>
      </> : null}
    </section>
  );
}

function InfoCell({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0 rounded-field bg-surface-2 p-3"><p className="text-[12px] text-ink-3">{label}</p><p className="mt-1 break-words text-[14px] font-semibold">{value}</p></div>;
}

function EventRow({ event }: { event: StatusEvent }) {
  return <article className="grid gap-2 border-b border-line py-4 last:border-0 sm:grid-cols-[150px_minmax(0,1fr)_auto] sm:items-center">
    <time dateTime={event.occurredAt} className="text-[13px] text-ink-3">{dateLabel(event.occurredAt)}</time>
    <div className="min-w-0"><div className="flex flex-wrap items-center gap-2 text-[14px]"><span className="font-mono">Услуга #{event.serviceId}</span><span className="text-ink-3">·</span>{event.fromStatus ? <><StatusChip status={event.fromStatus} /><span aria-hidden="true" className="text-ink-3">→</span></> : <span className="text-ink-3">Создана</span>}<StatusChip status={event.toStatus} /></div><p className="mt-1 text-[13px] text-ink-2">{event.fromStatus ? 'Статус услуги изменён' : 'Услуга добавлена'}</p><p className="mt-1 text-[12px] text-ink-3">Источник: <span className="font-mono">{SOURCE_LABELS[event.source]}</span></p></div>
    <Link to={`/customers/${event.accountId}`} className="text-[14px] font-semibold text-accent hover:underline">Клиент #{event.accountId} →</Link>
  </article>;
}

function StatusChip({ status }: { status: ServiceStatus }) {
  return <span className="rounded-full bg-surface-2 px-2.5 py-1 text-[12px] font-medium">{STATUS_LABELS[status]}</span>;
}

export default function Data() {
  const events = useSourceEvents();
  const live = useLiveUpdates();
  return (
    <>
      <PageHeader title="Данные" subtitle="Загрузка снимка, симулятор и лента изменений" actions={
        <><a href="/admin/" target="_blank" rel="noreferrer" className="inline-flex h-10 items-center gap-2 rounded-field border border-line px-3 text-[13px] font-semibold hover:bg-surface-2">Админка <ExternalLink size={14} /></a><a href="/api/v1/docs" target="_blank" rel="noreferrer" className="inline-flex h-10 items-center gap-2 rounded-field border border-line px-3 text-[13px] font-semibold hover:bg-surface-2">Swagger <ExternalLink size={14} /></a><a href="/prometheus/" target="_blank" rel="noreferrer" className="inline-flex h-10 items-center gap-2 rounded-field border border-line px-3 text-[13px] font-semibold hover:bg-surface-2">Prometheus <ExternalLink size={14} /></a></>
      } />
      <div className="grid gap-5 xl:grid-cols-2"><UploadPanel /><SimulatorPanel /></div>
      <section className="mt-6 rounded-block bg-surface p-5 sm:p-6">
        <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-display text-[20px] font-semibold">Лента изменений</h2><p className="mt-1 text-[14px] text-ink-2">Последние изменения статусов услуг.</p></div><LiveDot status={live.status} label={live.status === 'live' ? 'Обновляется в реальном времени' : undefined} /></div>
        {events.isPending ? <div className="mt-5 space-y-3" aria-label="Загрузка событий" role="status">{Array.from({ length: 4 }, (_, index) => <div key={index} className="h-16 animate-pulse rounded-field bg-surface-2" />)}</div> : events.isError && !events.data ? <div className="mt-4"><ErrorState title="Не удалось загрузить ленту" message={events.error.message} onRetry={() => events.refetch()} /></div> : events.data?.items.length ? <div className="mt-2">{events.data.items.map((event) => <EventRow key={event.id} event={event} />)}</div> : <div className="mt-4"><EmptyState title="Изменений пока нет" hint="События появятся после загрузки снимка или работы симулятора." /></div>}
        {events.data && events.data.total > events.data.items.length ? <p className="mt-3 text-[12px] text-ink-3">Показаны последние {events.data.items.length} событий из {events.data.total}.</p> : null}
      </section>
    </>
  );
}
