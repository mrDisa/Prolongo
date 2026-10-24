import { ArrowRight, Users } from 'lucide-react';
import { Link } from 'react-router-dom';
import type { components } from '@/api/schema';
import { formatEUR } from '@/lib/format';
import { customerCountPhrase, topLossShare } from '@/lib/insight';
import { useRecentlyChanged } from '@/live/useLiveUpdates';

type Summary = components['schemas']['Summary'];

export function InsightHero({ summary }: { summary: Summary }) {
  const first = summary.topAtRisk[0];
  const { expectedLossCents, worstCaseLossCents, mrrAtRiskCents, totalMrrCents } = summary.money;
  const expectedLossChanged = useRecentlyChanged(expectedLossCents);
  const scoredCustomers = summary.customers.total - summary.customers.noScore;
  const attentionCustomers = summary.customers.high + summary.customers.medium;
  const concentration = topLossShare(summary);
  const insight = expectedLossCents > 0 && first
    ? `${concentration.percent}% этой суммы приходится на ${customerCountPhrase(concentration.count)}. Начните с ${first.email}: ${first.topReasons[0]?.text ?? 'высокие ожидаемые потери'}.`
    : 'За выбранный период ожидаемых потерь нет.';

  return (
    <section className="grid gap-7 rounded-hero bg-surface p-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(280px,0.8fr)] lg:p-8" aria-label="Ожидаемые потери и главная рекомендация">
      <div className="min-w-0">
        <p className="inline-flex rounded-full bg-accent-soft px-3 py-1 text-[13px] font-semibold text-accent-text">Ожидаемые потери · {summary.horizonDays} дн.</p>
        <p className={`mt-4 inline-block rounded px-1 font-display text-[clamp(38px,6vw,58px)] font-semibold leading-tight tracking-[-0.04em] transition-colors duration-300 ${expectedLossChanged ? 'bg-accent-soft' : 'bg-transparent'}`}>{formatEUR(expectedLossCents)}</p>
        <p className="mt-3 max-w-3xl text-[15px] leading-6 text-ink-2">{insight}</p>
        <div className="mt-5 flex flex-wrap gap-3">
          {first ? (
            <Link to={`/customers/${first.id}?h=${summary.horizonDays}`} className="inline-flex min-h-11 items-center gap-2 rounded-field bg-accent px-4 py-2 text-[14px] font-semibold text-accent-ink hover:opacity-90">
              Открыть {first.email.split('@')[0]} <ArrowRight size={17} aria-hidden="true" />
            </Link>
          ) : null}
          <Link to={`/customers?h=${summary.horizonDays}`} className="inline-flex min-h-11 items-center rounded-field border border-line-strong px-4 py-2 text-[14px] font-semibold text-ink hover:bg-surface-2">Весь рейтинг</Link>
        </div>
      </div>
      <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
        <Metric label="Худший случай" value={formatEUR(worstCaseLossCents)} hint="Если клиенты с высоким риском уйдут" />
        <Metric label="MRR под риском" value={formatEUR(mrrAtRiskCents)} hint={`из ${formatEUR(totalMrrCents)} месячной выручки`} />
        <Metric label="Требуют внимания" value={`${attentionCustomers} из ${scoredCustomers}`} hint="Клиенты со средним и высоким риском" icon />
      </div>
    </section>
  );
}

function Metric({ label, value, hint, icon = false }: { label: string; value: string; hint: string; icon?: boolean }) {
  const changed = useRecentlyChanged(value);
  return (
    <div className="rounded-tile-lg bg-surface-2 p-4">
      <p className="flex items-center gap-2 text-[13px] font-medium text-ink-2">{icon ? <Users size={15} aria-hidden="true" /> : null}{label}</p>
      <p className={`mt-2 inline-block rounded px-1 font-display text-[22px] font-semibold transition-colors duration-300 ${changed ? 'bg-accent-soft' : 'bg-transparent'}`}>{value}</p>
      <p className="mt-1 text-[12px] text-ink-3">{hint}</p>
    </div>
  );
}
