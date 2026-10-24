import type { components } from '@/api/schema';
import { formatEUR } from '@/lib/format';

type Money = components['schemas']['Money'];

export function MoneySplit({ money, probability }: { money: Money; probability: number }) {
  const total = money.revenueCents;
  const segmentWidth = (value: number) => total > 0 ? `${Math.min(100, Math.max(0, (value / total) * 100))}%` : '0%';

  return (
    <section className="rounded-block bg-surface p-6" aria-labelledby="money-split-title">
      <h2 id="money-split-title" className="font-display text-[20px] font-semibold">Сколько можем потерять</h2>
      <p className="mt-1 text-[14px] text-ink-2">Выручка живых услуг за {money.horizonDays} дн. · {formatEUR(total)}</p>
      <div className="mt-5 flex h-4 overflow-hidden rounded-full bg-track" role="img" aria-label={`Выручка: бонусами ${formatEUR(money.bonusCoveredCents)}, с баланса ${formatEUR(money.balanceCoveredCents)}, под риском ${formatEUR(money.atRiskCents)}`}>
        <span className="h-full bg-unrated" style={{ width: segmentWidth(money.bonusCoveredCents) }} />
        <span className="h-full bg-low" style={{ width: segmentWidth(money.balanceCoveredCents) }} />
        <span className="h-full bg-high" style={{ width: segmentWidth(money.atRiskCents) }} />
      </div>
      <dl className="mt-5 space-y-3 text-[14px]">
        <div className="flex items-center justify-between gap-3"><dt className="flex items-center gap-2 text-ink-2"><span className="h-2.5 w-2.5 rounded-full bg-unrated" />Покроют бонусы</dt><dd className="font-semibold">{formatEUR(money.bonusCoveredCents)}</dd></div>
        <div className="flex items-center justify-between gap-3"><dt className="flex items-center gap-2 text-ink-2"><span className="h-2.5 w-2.5 rounded-full bg-low" />С баланса автоматически</dt><dd className="font-semibold">{formatEUR(money.balanceCoveredCents)}</dd></div>
        <div className="flex items-center justify-between gap-3"><dt className="flex items-center gap-2 text-ink-2"><span className="h-2.5 w-2.5 rounded-full bg-high" />Под риском</dt><dd className="font-semibold">{formatEUR(money.atRiskCents)}</dd></div>
      </dl>
      <div className="mt-5 rounded-tile-lg bg-surface-2 p-4">
        <p className="text-[13px] text-ink-2">Под риском × шанс ухода = ожидаемые потери</p>
        <p className="mt-1 font-display text-[18px] font-semibold">{formatEUR(money.atRiskCents)} × {Math.round(probability * 100)}% = {formatEUR(money.expectedLossCents)}</p>
      </div>
      <p className="mt-4 text-[14px] leading-6 text-ink-2">{money.explanation}</p>
    </section>
  );
}
