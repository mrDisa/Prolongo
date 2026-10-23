import type { ReactNode } from 'react';

interface EmptyStateProps {
  title: string;
  hint?: string;
  action?: ReactNode;
}

export function EmptyState({ title, hint, action }: EmptyStateProps) {
  return (
    <div className="rounded-block bg-surface p-8">
      <div className="rounded-tile-lg bg-surface-2 p-6">
        <h2 className="font-display text-[20px] font-semibold leading-7">{title}</h2>
        {hint ? <p className="mt-1 text-[15px] text-ink-2">{hint}</p> : null}
        {action ? <div className="mt-4">{action}</div> : null}
      </div>
    </div>
  );
}
