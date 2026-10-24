import { EmptyState } from '@/components/EmptyState';
import { PageHeader } from '@/components/PageHeader';

export default function Data() {
  return (
    <>
      <PageHeader title="Данные" subtitle="Загрузка снимка, симулятор и лента изменений" />
      <EmptyState title="Экран в разработке" hint="Работа с данными появится в задаче F10." />
    </>
  );
}
