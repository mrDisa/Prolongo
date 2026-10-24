import { useState } from 'react';
import { CalendarDays, Check } from 'lucide-react';

const PERIODS = [7, 14, 30, 60, 90];
const QUICK_PERIODS = [21, 45, 180, 365];

export function PeriodSelector({ value, referenceTime, onChange }: {
  value: number;
  referenceTime?: string;
  onChange: (days: number) => void;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(String(value));
  const days = Number(draft);
  const valid = Number.isInteger(days) && days >= 1 && days <= 365;

  function choose(period: number) {
    onChange(period);
    setOpen(false);
  }

  return (
    <div className="relative flex rounded-seg border border-line-strong bg-surface p-1" aria-label="Период прогноза">
      {PERIODS.map((period) => (
        <button
          key={period}
          type="button"
          aria-pressed={value === period}
          onClick={() => choose(period)}
          className={`h-9 min-w-10 rounded-[8px] px-2 text-[13px] font-semibold transition-colors ${value === period ? 'bg-accent-soft text-accent-text' : 'text-ink-2 hover:bg-surface-2'}`}
        >{period}</button>
      ))}
      <button
        type="button"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => {
          setDraft(String(value));
          setOpen((current) => !current);
        }}
        className={`flex h-9 items-center gap-1 rounded-[8px] px-2 text-[13px] font-semibold transition-colors ${!PERIODS.includes(value) ? 'bg-accent-soft text-accent-text' : 'text-ink-2 hover:bg-surface-2'}`}
      ><CalendarDays size={15} aria-hidden="true" /><span className="hidden sm:inline">Свой</span></button>
      {open ? (
        <div role="dialog" aria-label="Свой период прогноза" className="absolute right-0 top-full z-20 mt-2 w-[min(320px,calc(100vw-32px))] rounded-block border border-line bg-surface p-4 shadow-xl">
          <label className="block text-[13px] font-semibold" htmlFor="custom-horizon">На сколько дней вперёд</label>
          <input
            id="custom-horizon"
            type="number"
            min={1}
            max={365}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            className="mt-2 h-11 w-full rounded-field border border-line-strong bg-surface px-3 text-ink outline-none focus:border-accent"
          />
          <p className="mt-1 min-h-5 text-[13px] text-ink-2">{valid ? `Прогноз ${formatPreview(days, referenceTime)}` : 'Введите число от 1 до 365'}</p>
          <div className="mt-3 flex flex-wrap gap-2" aria-label="Быстрый выбор периода">
            {QUICK_PERIODS.map((quick) => (
              <button key={quick} type="button" onClick={() => setDraft(String(quick))} className="rounded-full border border-line px-3 py-1.5 text-[12px] font-semibold text-ink-2 hover:bg-surface-2">{quick}</button>
            ))}
          </div>
          <button type="button" disabled={!valid} onClick={() => choose(days)} className="mt-4 inline-flex h-10 w-full items-center justify-center gap-2 rounded-field bg-accent px-4 text-[14px] font-semibold text-accent-ink disabled:cursor-not-allowed disabled:opacity-50">
            <Check size={16} aria-hidden="true" /> Готово
          </button>
        </div>
      ) : null}
    </div>
  );
}

function formatPreview(days: number, referenceTime?: string): string {
  const start = referenceTime ? new Date(referenceTime) : new Date();
  const date = new Date(start.getTime() + days * 24 * 60 * 60 * 1000);
  const parts = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short' }).formatToParts(date);
  const day = parts.find((part) => part.type === 'day')?.value;
  const month = parts.find((part) => part.type === 'month')?.value.replace(/\.$/, '');
  return `до ${day} ${month}`;
}
