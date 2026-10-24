import { cn } from '@/lib/utils';

export type LiveStatus = 'live' | 'reconnecting' | 'idle';

interface LiveDotProps {
  status?: LiveStatus;
  label?: string;
}

const DEFAULT_LABEL: Record<LiveStatus, string> = {
  live: 'Live',
  reconnecting: 'Переподключение…',
  idle: 'Подключаемся…',
};

/** Connection state for live recalculation updates. */
export function LiveDot({ status = 'idle', label }: LiveDotProps) {
  return (
    <div className="flex items-center gap-2 text-[13px] font-medium text-ink-2" role="status">
      <span
        aria-hidden="true"
        className={cn('h-2 w-2 shrink-0 rounded-full', status === 'live' ? 'bg-accent' : 'bg-unrated')}
      />
      <span>{label ?? DEFAULT_LABEL[status]}</span>
    </div>
  );
}
