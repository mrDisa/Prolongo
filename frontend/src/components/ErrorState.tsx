interface ErrorStateProps {
  title?: string;
  message?: string;
  onRetry?: () => void;
}

export function ErrorState({ title = 'Не удалось загрузить данные', message, onRetry }: ErrorStateProps) {
  return (
    <div role="alert" className="rounded-block bg-surface p-8">
      <div className="rounded-tile-lg bg-surface-2 p-6">
        <h2 className="font-display text-[20px] font-semibold leading-7">{title}</h2>
        {message ? <p className="mt-1 text-[15px] text-ink-2">{message}</p> : null}
        {onRetry ? (
          <button
            type="button"
            onClick={onRetry}
            className="mt-4 inline-flex h-[46px] items-center rounded-field bg-accent px-5 font-semibold text-accent-ink"
          >
            Повторить
          </button>
        ) : null}
      </div>
    </div>
  );
}
