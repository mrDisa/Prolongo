const euroFormatter = new Intl.NumberFormat('ru-RU', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

/** Format integer euro cents as a localized euro amount. */
export function formatEUR(cents: number): string {
  return `€${euroFormatter.format(cents / 100)}`;
}
