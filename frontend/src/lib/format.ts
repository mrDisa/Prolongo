const euroFormatter = new Intl.NumberFormat('ru-RU', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

/** Format integer euro cents as a localized euro amount. */
export function formatEUR(cents: number): string {
  return `€${euroFormatter.format(cents / 100)}`;
}

const dateFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'short',
  year: 'numeric',
});
const timeFormatter = new Intl.DateTimeFormat('ru-RU', {
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
});

export function formatDate(value: string): string {
  const date = new Date(value);
  const dateParts = Object.fromEntries(dateFormatter.formatToParts(date).map(({ type, value: part }) => [type, part]));
  const timeParts = Object.fromEntries(timeFormatter.formatToParts(date).map(({ type, value: part }) => [type, part]));
  const month = dateParts.month.replace(/\.$/, '');
  const year = dateParts.year.replace(/\s*г\.?$/, '');
  return `${dateParts.day} ${month} ${year}, ${timeParts.hour}:${timeParts.minute}`;
}

export function formatDaysLeft(days: number): string {
  if (days < 0) return `Истекла ${Math.round(Math.abs(days))} дн. назад`;
  if (days < 1) {
    const minutes = Math.max(1, Math.round(days * 24 * 60));
    if (minutes < 60) return `Через ${minutes} мин`;
    return `Через ${Math.round(minutes / 60)} ч.`;
  }
  return `Через ${Math.round(days)} дн.`;
}

export function formatRelative(value: string): string {
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
  if (seconds < 60) return `${seconds} с назад`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} мин назад`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} ч назад`;
  return `${Math.floor(hours / 24)} дн назад`;
}
