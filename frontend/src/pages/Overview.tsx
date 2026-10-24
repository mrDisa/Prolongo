import { EmptyState } from '@/components/EmptyState';
import { PageHeader } from '@/components/PageHeader';

export default function Overview() {
  return (
    <>
      <PageHeader title="Обзор" subtitle="Кто может уйти и сколько это стоит" />
      <EmptyState title="Экран в разработке" hint="Сводка по клиентам и деньгам появится в задаче F5." />
    </>
  );
}
