import type { components } from '@/api/schema';
import { formatDate, formatEUR } from '@/lib/format';

type Transaction = components['schemas']['TransactionItem'];

export function PaymentStrip({ transactions }: { transactions: Transaction[] }) {
  const renewals = transactions.filter((transaction) => transaction.type === 'prolong' && transaction.status === 'performed').length;
  const paid = transactions.filter((transaction) => transaction.status === 'performed').length;
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
      {transactions.length ? (
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-[13px] text-ink-2" aria-label="Обозначения платежей">
          <span className="inline-flex items-center gap-1.5"><span className="h-3 w-3 rounded-[3px] bg-low" aria-hidden="true" />Прошли · {paid}</span>
          <span className="inline-flex items-center gap-1.5"><span className="h-3 w-3 rounded-[3px] bg-high" aria-hidden="true" />Не оплачены · {failed}</span>
        </div>
      ) : null}
      <p className="mt-3 text-[14px] leading-5 text-ink-2">
        Успешных продлений: <strong className="text-ink">{renewals}</strong>
        {renewals ? ' — поэтому «стабильно продлевает» снижает риск.' : ' — стабильные продления пока не снижают риск.'}
      </p>
    </div>
  );
}
