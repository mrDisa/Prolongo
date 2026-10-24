import { EmptyState } from '@/components/EmptyState';
import { PageHeader } from '@/components/PageHeader';

export default function Model() {
  return (
    <>
      <PageHeader title="Модель риска" subtitle="Веса признаков, пороги и горизонт прогноза" />
      <EmptyState title="Экран в разработке" hint="Настройки модели появятся в задаче F6." />
    </>
  );
}
