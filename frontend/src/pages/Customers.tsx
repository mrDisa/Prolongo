import { EmptyState } from '@/components/EmptyState';
import { PageHeader } from '@/components/PageHeader';

export default function Customers() {
  return (
    <>
      <PageHeader title="Клиенты" subtitle="Рейтинг клиентов по риску ухода" />
      <EmptyState title="Экран в разработке" hint="Таблица клиентов появится в задаче F3." />
    </>
  );
}
