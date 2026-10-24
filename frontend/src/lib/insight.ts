import type { components } from '@/api/schema';

type Summary = components['schemas']['Summary'];

export function topLossShare(summary: Summary): { count: number; percent: number } {
  const total = summary.money.expectedLossCents;
  if (total <= 0 || summary.topAtRisk.length === 0) return { count: 0, percent: 0 };

  let accumulated = 0;
  for (let index = 0; index < summary.topAtRisk.length; index += 1) {
    accumulated += summary.topAtRisk[index].expectedLossCents;
    if (accumulated >= total * 0.8) {
      return { count: index + 1, percent: Math.round((accumulated / total) * 100) };
    }
  }
  return { count: summary.topAtRisk.length, percent: Math.round((accumulated / total) * 100) };
}

export function customerCountPhrase(count: number): string {
  const lastTwo = count % 100;
  if (lastTwo >= 11 && lastTwo <= 14) return `${count} клиентов`;
  const last = count % 10;
  if (last === 1) return `${count} клиента`;
  if (last >= 2 && last <= 4) return `${count} клиента`;
  return `${count} клиентов`;
}

export function formatHorizonEnd(referenceTime: string, days: number): string {
  const date = new Date(new Date(referenceTime).getTime() + days * 24 * 60 * 60 * 1000);
  const parts = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' })
    .formatToParts(date)
    .reduce<Record<string, string>>((result, part) => ({ ...result, [part.type]: part.value }), {});
  return `до ${parts.day} ${parts.month.replace(/\.$/, '')} ${parts.year}`;
}
