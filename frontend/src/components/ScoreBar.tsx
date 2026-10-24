import type { Customer } from '@/lib/levels';

export function ScoreBar({ customer }: { customer: Customer }) {
  if (customer.riskScore === null) return <span className="font-display text-[16px] text-ink-3">—</span>;
  const color = customer.riskLevel === 'high' ? 'bg-high' : customer.riskLevel === 'medium' ? 'bg-med' : 'bg-low';

  return (
    <div className="flex min-w-[130px] items-center gap-3">
      <span className="w-7 text-right font-display text-[16px] font-semibold">{customer.riskScore}</span>
      <span className="h-2 w-[90px] overflow-hidden rounded-full bg-track" aria-hidden="true">
        <span className={`block h-full rounded-full ${color}`} style={{ width: `${customer.riskScore}%` }} />
      </span>
    </div>
  );
}
