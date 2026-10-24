import type { components } from '@/api/schema';

type FeatureSetting = components['schemas']['FeatureSetting'];
type PreviewFeature = components['schemas']['PreviewFeature'];

export function FeatureWeightRow({ feature, saved, preview, onWeightChange, onEnabledChange }: {
  feature: FeatureSetting;
  saved: FeatureSetting;
  preview?: PreviewFeature;
  onWeightChange: (weight: number) => void;
  onEnabledChange: (enabled: boolean) => void;
}) {
  const changed = feature.weight !== saved.weight || feature.enabled !== saved.enabled;
  const signLabel = feature.weight < 0 ? 'снижает риск' : 'повышает риск';

  return (
    <article className={`rounded-tile-lg border p-4 ${changed ? 'border-accent bg-surface' : 'border-line bg-surface'}`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="font-semibold">{feature.title}</h3>
            <span className={`rounded-full px-2.5 py-1 text-[11px] font-semibold ${feature.weight < 0 ? 'bg-low-soft text-low-text' : 'bg-high-soft text-high-text'}`}>{signLabel}</span>
            {changed ? <span className="text-[12px] font-medium text-accent-text">изменено · было {saved.weight}</span> : null}
          </div>
          <p className="mt-1 text-[13px] leading-5 text-ink-2">{feature.description}</p>
        </div>
        <label className="inline-flex shrink-0 items-center gap-2 text-[13px] font-medium text-ink-2">
          <input type="checkbox" checked={feature.enabled} onChange={(event) => onEnabledChange(event.target.checked)} className="h-4 w-4 accent-accent" />
          Учитывать
        </label>
      </div>

      <div className="mt-4 grid grid-cols-[minmax(0,1fr)_76px] items-center gap-4">
        <input
          type="range"
          min={feature.min}
          max={feature.max}
          step={1}
          value={feature.weight}
          disabled={!feature.enabled}
          aria-label={`Вес признака «${feature.title}»`}
          onChange={(event) => onWeightChange(Number(event.target.value))}
          className="h-2 w-full cursor-pointer accent-accent disabled:cursor-not-allowed disabled:opacity-40"
        />
        <output className="rounded-seg bg-surface-2 px-3 py-2 text-right font-display text-[16px] font-semibold">{feature.weight}</output>
      </div>
      <p className="mt-2 text-[12px] text-ink-3">
        {feature.enabled
          ? `Сейчас срабатывает у ${preview?.customersHit ?? '—'} клиентов, до ${preview?.maxPoints !== undefined && preview.maxPoints > 0 ? '+' : ''}${preview?.maxPoints ?? '—'} балла.`
          : 'Признак отключён и не влияет на оценку.'}
      </p>
    </article>
  );
}
