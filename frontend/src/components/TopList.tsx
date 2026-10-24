import { Link } from 'react-router-dom';
import type { components } from '@/api/schema';
import { RiskBadge } from '@/components/RiskBadge';
import { formatEUR } from '@/lib/format';

type AtRiskCustomer = components['schemas']['CustomerListItem'];

export function TopList({ customers, horizonDays }: { customers: AtRiskCustomer[]; horizonDays: number }) {
  return (
    <ol className="divide-y divide-line">
      {customers.map((customer, index) => (
        <li key={customer.id}>
          <Link to={`/customers/${customer.id}?h=${horizonDays}`} className="grid gap-3 rounded-tile-lg px-2 py-4 transition-colors hover:bg-surface-2 sm:grid-cols-[40px_minmax(0,1fr)_auto] sm:items-center">
            <span className="flex h-9 w-9 items-center justify-center rounded-full bg-surface-2 font-display text-[15px] font-semibold text-ink-2">{index + 1}</span>
            <span className="min-w-0">
              <span className="flex flex-wrap items-center gap-2">
                <span className="break-all text-[14px] font-semibold text-ink">{customer.email}</span>
                <RiskBadge customer={customer} showScore />
              </span>
              <span className="mt-1 block line-clamp-2 text-[13px] text-ink-2">{customer.topReasons[0]?.text ?? 'Причина не указана'}</span>
            </span>
            <span className="whitespace-nowrap text-left sm:text-right">
              <span className="block font-display text-[18px] font-semibold">{formatEUR(customer.expectedLossCents)}</span>
              <span className="text-[12px] text-ink-3">за {horizonDays} дн.</span>
            </span>
          </Link>
        </li>
      ))}
    </ol>
  );
}
