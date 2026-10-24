import type { Customer } from '@/lib/levels';
import { useRecentlyChanged } from '@/live/useLiveUpdates';

export function ScoreBar({ customer }: { customer: Pick<Customer, 'riskScore' | 'riskLevel'> }) {
  const changed = useRecentlyChanged(customer.riskScore);
  if (customer.riskScore === null) return <span className="font-display text-[16px] text-ink-3">—</span>;
  const color = customer.riskLevel === 'high' ? 'bg-high' : customer.riskLevel === 'medium' ? 'bg-med' : 'bg-low';

  return (
    <div className="flex min-w-[142px] items-center gap-3">
      <span className={`w-10 shrink-0 whitespace-nowrap rounded px-1 text-right font-display text-[16px] font-semibold transition-colors duration-300 ${changed ? 'bg-accent-soft' : 'bg-transparent'}`}>{customer.riskScore}</span>
      <span className="h-2 w-[90px] shrink-0 overflow-hidden rounded-full bg-track" aria-hidden="true">
        <span className={`block h-full rounded-full ${color}`} style={{ width: `${customer.riskScore}%` }} />
      </span>
    </div>
  );
}
