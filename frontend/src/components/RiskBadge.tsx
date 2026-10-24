import type { Customer } from '@/lib/levels';
import { levelLabel, RISK_LEVEL_STYLES } from '@/lib/levels';

export function RiskBadge({ customer }: { customer: Customer }) {
  const isUnrated = customer.scoreStatus === 'no_score';
  const colors = isUnrated
    ? { badge: 'bg-unrated-soft text-unrated-text', dot: 'bg-unrated' }
    : RISK_LEVEL_STYLES[customer.riskLevel ?? 'low'];

  return (
    <span className={`inline-flex items-center gap-2 whitespace-nowrap rounded-full px-3 py-1 text-[13px] font-semibold leading-[18px] ${colors.badge}`}>
      <span aria-hidden="true" className={`h-2 w-2 rounded-full ${colors.dot}`} />
      {levelLabel(customer)}
    </span>
  );
}
