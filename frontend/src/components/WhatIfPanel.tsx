import { useState } from 'react';
import { ArrowRight, Calculator } from 'lucide-react';
import { useMutation } from '@tanstack/react-query';
import type { components } from '@/api/schema';
import { api } from '@/api/client';
import { unwrap } from '@/api/errors';
import { RiskBadge } from '@/components/RiskBadge';
import { formatEUR } from '@/lib/format';

type WhatIfAction = components['schemas']['WhatIfAction'];
type WhatIfState = components['schemas']['WhatIfState'];
type Service = components['schemas']['ServiceItem'];
type Contribution = components['schemas']['Contribution'];

const LIVE = ['active', 'suspended', 'prolong_wait'];

/** Features that add risk before the actions and are gone after them. */
function removedReasons(before: WhatIfState, after: WhatIfState, titles: Map<string, string>): string[] {
  const left = new Set(after.contributions.filter((c) => c.points > 0).map((c) => c.feature));
  return before.contributions
    .filter((c) => c.points > 0 && !left.has(c.feature))
    .map((c) => titles.get(c.feature) ?? c.feature);
}

function StateCell({ label, state }: { label: string; state: WhatIfState }) {
  return (
    <div className="min-w-0 flex-1 rounded-tile-lg bg-surface-2 p-4">
      <p className="text-[12px] text-ink-3">{label}</p>
      <div className="mt-2"><RiskBadge customer={state} showScore /></div>
      <p className="mt-3 font-display text-[18px] font-semibold">{formatEUR(state.money.expectedLossCents)}</p>
      <p className="text-[12px] text-ink-3">ожидаемые потери</p>
    </div>
  );
}

/** "What if…" on the customer card: retention actions and their effect, nothing is saved. */
export function WhatIfPanel({ customerId, calcVersion, horizonDays, services, contributions }: {
  customerId: number;
  calcVersion: number;
  horizonDays: number | undefined;
  services: Service[];
  contributions: Contribution[];
}) {
  const waiting = services.filter((s) => s.status === 'prolong_wait').length;
  const manual = services.filter((s) => LIVE.includes(s.status) && !s.autoProlong).length;
  const [topUp, setTopUp] = useState(waiting > 0);
  const [amountEur, setAmountEur] = useState('50');
  const [autoProlong, setAutoProlong] = useState(waiting === 0 && manual > 0);
  const [payWaiting, setPayWaiting] = useState(waiting > 0);
  const titles = new Map(contributions.map((c) => [c.feature, c.title]));

  const amountCents = Math.round(Number(amountEur.replace(',', '.')) * 100);
  const amountValid = Number.isFinite(amountCents) && amountCents >= 1 && amountCents <= 10_000_000;
  const actions: WhatIfAction[] = [
    ...(topUp ? [{ type: 'top_up' as const, amountCents }] : []),
    ...(autoProlong ? [{ type: 'enable_auto_prolong' as const }] : []),
    ...(payWaiting ? [{ type: 'pay_waiting' as const }] : []),
  ];
  const canRun = actions.length > 0 && (!topUp || amountValid);

  // The result stays on screen through recalculations (the timer runs every minute);
  // a hint says when the data it was counted on is no longer the latest.
  const [countedOn, setCountedOn] = useState<number | null>(null);
  const whatIf = useMutation({
    onSuccess: () => setCountedOn(calcVersion),
    mutationFn: () => unwrap(api.POST('/api/v1/customers/{id}/what-if', {
      params: { path: { id: customerId }, query: horizonDays === undefined ? {} : { horizonDays } },
      body: { actions },
    })),
  });
  const reset = () => whatIf.reset();
  const result = whatIf.data;
  const gone = result ? removedReasons(result.before, result.after, titles) : [];

  return (
    <section className="rounded-block bg-surface p-6" aria-labelledby="what-if-title">
      <h2 id="what-if-title" className="font-display text-[20px] font-semibold">Что будет, если…</h2>
      <p className="mt-1 text-[14px] text-ink-2">Прикиньте эффект звонка до того, как звонить. Ничего не сохраняется.</p>

      <fieldset className="mt-4 space-y-3 text-[14px]">
        <legend className="sr-only">Действия клиента</legend>
        <label className="flex flex-wrap items-center gap-2">
          <input type="checkbox" checked={topUp} onChange={(e) => { setTopUp(e.target.checked); reset(); }} className="h-4 w-4 accent-accent" />
          <span>Пополнит баланс на €</span>
          <input
            type="text" inputMode="decimal" value={amountEur} aria-label="Сумма пополнения, евро"
            onChange={(e) => { setAmountEur(e.target.value); reset(); }}
            className={`h-9 w-24 rounded-field border bg-surface-2 px-3 ${topUp && !amountValid ? 'border-high' : 'border-line'}`}
          />
        </label>
        <label className={`flex items-center gap-2 ${manual === 0 ? 'text-ink-3' : ''}`}>
          <input type="checkbox" checked={autoProlong} disabled={manual === 0} onChange={(e) => { setAutoProlong(e.target.checked); reset(); }} className="h-4 w-4 accent-accent" />
          <span>Включит автопродление{manual === 0 ? ' — уже включено' : ` (услуг без него: ${manual})`}</span>
        </label>
        <label className={`flex items-center gap-2 ${waiting === 0 ? 'text-ink-3' : ''}`}>
          <input type="checkbox" checked={payWaiting} disabled={waiting === 0} onChange={(e) => { setPayWaiting(e.target.checked); reset(); }} className="h-4 w-4 accent-accent" />
          <span>Оплатит ждущие услуги{waiting === 0 ? ' — таких нет' : ` (${waiting})`}</span>
        </label>
      </fieldset>

      <button
        type="button" disabled={!canRun || whatIf.isPending} onClick={() => whatIf.mutate()}
        className="mt-4 inline-flex h-[46px] items-center gap-2 rounded-field bg-accent px-5 font-semibold text-accent-ink transition-colors hover:opacity-90 disabled:opacity-50"
      >
        <Calculator size={18} aria-hidden="true" /> {whatIf.isPending ? 'Считаем…' : 'Посчитать'}
      </button>

      <div aria-live="polite">
        {whatIf.isError ? (
          <p className="mt-3 text-[14px] text-high-text">{whatIf.error instanceof Error ? whatIf.error.message : 'Не удалось посчитать'}</p>
        ) : null}
        {result ? (
          <div className="mt-5">
            <div className="flex items-stretch gap-2">
              <StateCell label="Сейчас" state={result.before} />
              <ArrowRight size={20} aria-hidden="true" className="shrink-0 self-center text-ink-3" />
              <StateCell label="После действий" state={result.after} />
            </div>
            <p className={`mt-4 font-display text-[18px] font-semibold ${result.savedCents > 0 ? 'text-low-text' : result.savedCents < 0 ? 'text-high-text' : ''}`}>
              {result.savedCents > 0
                ? `Спасём ${formatEUR(result.savedCents)} за ${result.after.money.horizonDays} дн.`
                : result.savedCents < 0 ? `Потери вырастут на ${formatEUR(-result.savedCents)}` : 'Ожидаемые потери не изменятся'}
            </p>
            {countedOn !== null && countedOn !== calcVersion ? (
              <p className="mt-1 text-[13px] text-ink-3">Данные обновились после расчёта — нажмите «Посчитать» ещё раз.</p>
            ) : null}
            {gone.length ? <p className="mt-1 text-[14px] text-ink-2">Уходит причина: {gone.join(', ').toLocaleLowerCase('ru-RU')}</p> : null}
            <ul className="mt-3 space-y-1 text-[13px] text-ink-2">
              {result.notes.map((note, index) => <li key={`${index}-${note}`}>· {note}</li>)}
            </ul>
          </div>
        ) : null}
      </div>
    </section>
  );
}
