import { RISK_LEVELS, type RiskLevel } from '@/lib/levels';

export function FilterChips({
  selectedLevels,
  counts,
  onSelect,
}: {
  selectedLevels: RiskLevel[];
  counts: (number | undefined)[];
  onSelect: (value: RiskLevel | 'all') => void;
}) {
  return (
    <section aria-label="Фильтр по уровню риска" className="mb-5 flex flex-wrap gap-2">
      <button
        type="button"
        aria-pressed={selectedLevels.length === 0}
        onClick={() => onSelect('all')}
        className={`h-10 rounded-full border px-4 text-[14px] font-semibold transition-colors ${selectedLevels.length === 0 ? 'border-accent bg-accent-soft text-accent-text' : 'border-line bg-surface text-ink-2 hover:bg-surface-2'}`}
      >
        Все <span className="ml-1 opacity-75">· {counts[0] ?? '—'}</span>
      </button>
      {RISK_LEVELS.map(({ value, label }, index) => {
        const active = selectedLevels.includes(value);
        return (
          <button
            key={value}
            type="button"
            aria-pressed={active}
            onClick={() => onSelect(value)}
            className={`h-10 rounded-full border px-4 text-[14px] font-semibold transition-colors ${active ? 'border-accent bg-accent-soft text-accent-text' : 'border-line bg-surface text-ink-2 hover:bg-surface-2'}`}
          >
            {label} <span className="ml-1 opacity-75">· {counts[index + 1] ?? '—'}</span>
          </button>
        );
      })}
    </section>
  );
}
