import type { ReactNode } from 'react';

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}

/** First row of every page: title on the left, page controls on the right. */
export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 className="font-display text-[34px] font-semibold leading-[42px] tracking-[-0.02em]">
          {title}
        </h1>
        {subtitle ? <p className="mt-1 text-[15px] text-ink-2">{subtitle}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-3">{actions}</div> : null}
    </div>
  );
}
