import type { components } from '@/api/schema';

export type RiskLevel = 'high' | 'medium' | 'low' | 'none';
export type Customer = components['schemas']['CustomerListItem'];

export const RISK_LEVELS: { value: RiskLevel; label: string }[] = [
  { value: 'high', label: 'Высокий' },
  { value: 'medium', label: 'Средний' },
  { value: 'low', label: 'Низкий' },
  { value: 'none', label: 'Нет оценки' },
];

export const RISK_LEVEL_STYLES: Record<Exclude<RiskLevel, 'none'>, { badge: string; dot: string }> = {
  high: { badge: 'bg-high-soft text-high-text', dot: 'bg-high' },
  medium: { badge: 'bg-med-soft text-med-text', dot: 'bg-med' },
  low: { badge: 'bg-low-soft text-low-text', dot: 'bg-low' },
};

export function isRiskLevel(value: string | null): value is RiskLevel {
  return value === 'high' || value === 'medium' || value === 'low' || value === 'none';
}

export function levelLabel(customer: Customer): string {
  if (customer.scoreStatus === 'no_score') return 'Нет оценки';
  if (customer.riskLevel === 'high') return 'Высокий';
  if (customer.riskLevel === 'medium') return 'Средний';
  return 'Низкий';
}
