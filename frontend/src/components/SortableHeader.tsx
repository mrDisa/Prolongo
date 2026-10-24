import { ArrowDown, ArrowDownUp, ArrowUp } from 'lucide-react';

export type SortKey = 'id' | 'email' | 'balance' | 'totalReplenished' | 'riskScore' | 'expectedLoss';

export function SortableHeader({
  column,
  label,
  active,
  order,
  onSort,
}: {
  column: { key: SortKey; label: string };
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
