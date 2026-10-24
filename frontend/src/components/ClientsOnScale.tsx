import type { components } from '@/api/schema';

type PreviewCustomer = components['schemas']['PreviewCustomer'];

interface Marker extends PreviewCustomer {
  lane: number;
}

export function ClientsOnScale({ customers, thresholds, selectedId, onSelect }: {
  customers: PreviewCustomer[];
  thresholds: { medium: number; high: number };
  selectedId: number | null;
  onSelect: (id: number) => void;
}) {
  const markers = placeMarkers(customers);
  const laneCount = Math.max(2, ...markers.map((marker) => marker.lane + 1));
  const medium = Math.min(100, Math.max(0, thresholds.medium));
  const high = Math.min(100, Math.max(medium, thresholds.high));

  return (
    <div>
      <div className="relative rounded-tile-lg bg-surface-2 px-2 pt-4" role="group" aria-label="Клиенты на шкале риска">
        <div className="relative h-3 rounded-full" style={{ background: `linear-gradient(to right, var(--zone-low) 0% ${medium}%, var(--zone-med) ${medium}% ${high}%, var(--zone-high) ${high}% 100%)` }} />
        <div className="relative mt-2" style={{ height: `${laneCount * 30 + 8}px` }}>
          {markers.map((marker) => {
            const position = Math.min(98, Math.max(2, marker.riskScoreRaw));
            const selected = marker.id === selectedId;
            return (
              <button
                key={marker.id}
                type="button"
                onClick={() => onSelect(marker.id)}
                title={`${marker.email}: ${marker.riskScoreRaw.toFixed(1)} балла`}
                aria-pressed={selected}
                className={`absolute z-10 -translate-x-1/2 whitespace-nowrap rounded-full border px-2 py-1 text-[11px] font-semibold shadow-sm transition-[left,top,background-color] duration-300 ${selected ? 'border-accent bg-accent text-accent-ink' : 'border-line-strong bg-surface text-ink hover:border-accent'}`}
                style={{ left: `${position}%`, top: `${marker.lane * 30}px` }}
              >#{marker.id} · {Math.round(marker.riskScoreRaw)}</button>
            );
          })}
        </div>
        <div className="relative h-5 text-[11px] text-ink-3">
          <span className="absolute left-0">0</span>
          <span className="absolute -translate-x-1/2" style={{ left: `${medium}%` }}>{medium}</span>
          <span className="absolute -translate-x-1/2" style={{ left: `${high}%` }}>{high}</span>
          <span className="absolute right-0">100</span>
        </div>
      </div>
      <p className="mt-2 text-[12px] text-ink-3">Кликните метку, чтобы посмотреть вклад признаков.</p>
    </div>
  );
}

function placeMarkers(customers: PreviewCustomer[]): Marker[] {
  const sorted = [...customers].sort((a, b) => a.riskScoreRaw - b.riskScoreRaw);
  const lastAtLane: number[] = [];
  return sorted.map((customer) => {
    const position = Math.min(98, Math.max(2, customer.riskScoreRaw));
    let lane = lastAtLane.findIndex((lastPosition) => position - lastPosition >= 11);
    if (lane < 0) lane = lastAtLane.length;
    lastAtLane[lane] = position;
    return { ...customer, lane };
  });
}
