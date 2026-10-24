import type { components } from '@/api/schema';
import { formatDate, formatEUR } from '@/lib/format';

type Transaction = components['schemas']['TransactionItem'];

export function PaymentStrip({ transactions }: { transactions: Transaction[] }) {
  const renewals = transactions.filter((transaction) => transaction.type === 'prolong' && transaction.status === 'performed').length;
  const failed = transactions.filter((transaction) => transaction.status === 'not_performed').length;
  const ariaLabel = `Платежи: ${transactions.length} всего, ${transactions.length - failed} успешных, ${failed} не прошли`;

  return (
    <div>
      {transactions.length ? (
        <div className="flex flex-wrap gap-1.5" role="img" aria-label={ariaLabel}>
          {transactions.map((transaction) => (
            <span
              key={transaction.id}
              title={`${transaction.status === 'performed' ? 'Оплачен' : 'Не прошёл'} · ${formatEUR(transaction.amountCents)} · ${formatDate(transaction.createdAt)}`}
              className={`h-4 w-4 rounded-[4px] ${transaction.status === 'performed' ? 'bg-low' : 'bg-high'}`}
            />
          ))}
        </div>
      ) : <p className="text-[14px] text-ink-2">Платежей пока нет</p>}
      <p className="mt-3 text-[14px] text-ink-2">Успешных продлений: <strong className="text-ink">{renewals}</strong></p>
    </div>
  );
}
