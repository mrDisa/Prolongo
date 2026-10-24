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
  const markerAreaHeight = laneCount * 30 + 4;

  return (
    <div>
      <div className="relative" role="group" aria-label="Клиенты на шкале риска">
        <div className="relative" style={{ height: `${markerAreaHeight}px` }}>
          {markers.map((marker) => {
            const position = Math.min(98, Math.max(2, marker.riskScoreRaw));
            const selected = marker.id === selectedId;
            return (
              <span key={marker.id}>
                <span aria-hidden="true" className={`absolute z-0 w-px transition-[left,top] duration-200 ${selected ? 'bg-accent' : 'bg-line-strong'}`} style={{ left: `${position}%`, top: `${marker.lane * 30 + 26}px`, height: `${markerAreaHeight - marker.lane * 30 - 26}px` }} />
                <button
                  type="button"
                  onClick={() => onSelect(marker.id)}
                  title={`${marker.email}: ${marker.riskScoreRaw.toFixed(1)} балла`}
                  aria-pressed={selected}
                  className={`absolute z-10 -translate-x-1/2 whitespace-nowrap rounded-full border px-2 py-[3px] text-[11px] font-semibold shadow-sm transition-[left,top,background-color,color] duration-200 ${selected ? 'border-accent bg-accent text-accent-ink' : markerBorder(marker.riskLevel)}`}
                  style={{ left: `${position}%`, top: `${marker.lane * 30}px` }}
                >#{marker.id} · {Math.round(marker.riskScoreRaw)}</button>
              </span>
            );
          })}
        </div>
        <div className="relative h-3 rounded-full" style={{ background: `linear-gradient(to right, var(--zone-low) 0% ${medium}%, var(--zone-med) ${medium}% ${high}%, var(--zone-high) ${high}% 100%)` }} />
        <div className="relative mt-2 h-5 text-[11px]">
          <span className="absolute left-0 text-ink-3">0</span>
          <span className="absolute -translate-x-1/2 font-semibold text-low-text" style={{ left: `${medium}%` }}>{medium}</span>
          <span className="absolute -translate-x-1/2 font-semibold text-high-text" style={{ left: `${high}%` }}>{high}</span>
          <span className="absolute right-0 text-ink-3">100</span>
        </div>
      </div>
    </div>
  );
}

function markerBorder(level: PreviewCustomer['riskLevel']): string {
  if (level === 'high') return 'border-high bg-surface text-ink hover:bg-high-soft';
  if (level === 'medium') return 'border-med bg-surface text-ink hover:bg-med-soft';
  return 'border-low bg-surface text-ink hover:bg-low-soft';
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
