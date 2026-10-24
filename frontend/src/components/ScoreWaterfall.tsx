import type { components } from '@/api/schema';

type Contribution = components['schemas']['Contribution'];

interface ScoreWaterfallProps {
  contributions: Contribution[];
  score: number;
  level: 'high' | 'medium' | 'low';
}

const LEVEL_COLORS = { high: 'bg-high', medium: 'bg-med', low: 'bg-low' } as const;

export function ScoreWaterfall({ contributions, score, level }: ScoreWaterfallProps) {
  let cumulative = 0;
  const rows = contributions.map((contribution) => {
    const start = cumulative;
    cumulative = Math.min(100, Math.max(0, cumulative + contribution.points));
    return { contribution, start, end: cumulative };
  });

  return (
    <section className="rounded-block bg-surface p-6" aria-labelledby="score-breakdown-title">
      <h2 id="score-breakdown-title" className="font-display text-[20px] font-semibold">Из чего сложились {score} баллов</h2>
      <div className="mt-5 space-y-5">
        {rows.map(({ contribution, start, end }, index) => {
          const left = Math.min(start, end);
          const width = Math.abs(end - start);
          return (
            <div key={`${contribution.feature}-${contribution.serviceId ?? 'account'}-${index}`}>
              <div className="flex items-baseline justify-between gap-3">
                <span className="font-semibold">{contribution.title}</span>
                <span className={`shrink-0 font-display text-[14px] font-semibold ${contribution.points < 0 ? 'text-low-text' : 'text-high-text'}`}>
                  {contribution.points > 0 ? '+' : ''}{contribution.points}
                </span>
              </div>
              <div className="relative mt-2 h-2 overflow-hidden rounded-full bg-track" aria-hidden="true">
                <span className={`absolute top-0 h-full rounded-full ${contribution.points < 0 ? 'bg-low' : 'bg-high'}`} style={{ left: `${left}%`, width: `${width}%` }} />
              </div>
              <p className="mt-1 text-[13px] text-ink-2">{contribution.text}</p>
            </div>
          );
        })}
        <div className="border-t border-line pt-4">
          <div className="flex justify-between font-semibold"><span>Итоговый риск</span><span>{score} / 100</span></div>
          <div className="mt-2 h-2 overflow-hidden rounded-full bg-track" aria-label={`Итоговый риск ${score} из 100`}>
            <span className={`block h-full rounded-full ${LEVEL_COLORS[level]}`} style={{ width: `${score}%` }} />
          </div>
          <div className="mt-1 flex justify-between text-[12px] text-ink-3"><span>0</span><span>50</span><span>100</span></div>
        </div>
      </div>
    </section>
  );
}
