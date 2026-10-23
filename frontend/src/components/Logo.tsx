import { RefreshCw } from 'lucide-react';

export function Logo() {
  return (
    <div className="flex items-center gap-2.5">
      <div className="flex h-[38px] w-[38px] items-center justify-center rounded-[11px] bg-accent text-accent-ink">
        <RefreshCw size={20} strokeWidth={2.6} aria-hidden="true" />
      </div>
      <span className="font-display text-[19px] font-semibold">Prolongo</span>
    </div>
  );
}
