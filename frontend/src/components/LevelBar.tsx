import { Link } from 'react-router-dom';
import type { components } from '@/api/schema';

type CustomerCounts = components['schemas']['Summary']['customers'];

const levels = (thresholds: { medium: number; high: number }) => [
  { key: 'high', label: 'Высокий риск', hint: `Score от ${thresholds.high} — действовать сегодня`, color: 'bg-high', text: 'text-high-text' },
  { key: 'medium', label: 'Средний риск', hint: `Score от ${thresholds.medium} — связаться`, color: 'bg-med', text: 'text-med-text' },
  { key: 'low', label: 'Низкий риск', hint: `Score ниже ${thresholds.medium} — наблюдать`, color: 'bg-low', text: 'text-low-text' },
  { key: 'noScore', label: 'Нет оценки', hint: 'Уровень риска не рассчитывается', color: 'bg-unrated', text: 'text-unrated-text' },
] as const;

export function LevelBar({ customers, horizonDays, thresholds }: {
  customers: CustomerCounts;
  horizonDays: number;
  thresholds: { medium: number; high: number };
}) {
  const total = customers.total;
  const levelItems = levels(thresholds);
  return (
    <section className="rounded-block bg-surface p-6" aria-label="Клиенты по уровню риска">
      <div className="flex items-end justify-between gap-3">
        <h2 className="font-display text-[20px] font-semibold">Уровни риска</h2>
        <Link to={`/customers?h=${horizonDays}`} className="text-[13px] font-semibold text-accent-text hover:underline">Все клиенты</Link>
      </div>
      <div className="mt-4 flex h-3 overflow-hidden rounded-full bg-surface-2" role="img" aria-label={levelItems.map(({ key, label }) => `${label}: ${customers[key]}`).join(', ')}>
        {levelItems.map(({ key, color }) => <span key={key} className={`${color} h-full transition-[width]`} style={{ width: `${total ? (customers[key] / total) * 100 : 0}%` }} />)}
      </div>
      <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {levelItems.map(({ key, label, hint, color, text }) => (
          <Link key={key} to={`/customers?level=${key === 'noScore' ? 'none' : key}&h=${horizonDays}`} className="rounded-tile-lg border border-line bg-surface p-4 transition-colors hover:bg-surface-2">
            <p className={`flex items-center gap-2 text-[13px] font-semibold ${text}`}><span className={`h-2.5 w-2.5 rounded-full ${color}`} aria-hidden="true" />{label}</p>
            <p className="mt-2 font-display text-[28px] font-semibold">{customers[key]}</p>
            <p className="mt-1 text-[12px] text-ink-3">{hint}</p>
          </Link>
        ))}
      </div>
    </section>
  );
}
