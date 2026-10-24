interface ScoreScaleProps {
  score: number;
  loyalty: number;
  probability: number;
}

export function ScoreScale({ score, loyalty, probability }: ScoreScaleProps) {
  const position = Math.min(100, Math.max(0, score));

  return (
    <div aria-label={`Шкала риска: ${score} из 100`}>
      <div className="relative h-3 rounded-full" style={{ background: 'linear-gradient(to right, var(--zone-low) 0% 30%, var(--zone-med) 30% 60%, var(--zone-high) 60% 100%)' }}>
        <span className="absolute top-1/2 h-5 w-[3px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-ink" style={{ left: `${position}%` }} />
      </div>
      <div className="mt-2 flex justify-between text-[12px] text-ink-3">
        <span>0 · низкий</span><span>30 · средний</span><span>60 · высокий</span><span>100</span>
      </div>
      <div className="mt-4 flex flex-wrap gap-x-5 gap-y-1 text-[14px] text-ink-2">
        <span>Лояльность <strong className="text-ink">{loyalty} из 100</strong></span>
        <span>Шанс ухода <strong className="text-ink">{Math.round(probability * 100)}%</strong></span>
      </div>
    </div>
  );
}
