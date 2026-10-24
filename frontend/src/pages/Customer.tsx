import { Link, useParams } from 'react-router-dom';
import { EmptyState } from '@/components/EmptyState';
import { PageHeader } from '@/components/PageHeader';

export default function Customer() {
  const { id } = useParams<{ id: string }>();

  return (
    <>
      <Link to="/customers" className="mb-4 inline-block text-[14px] font-semibold text-accent-text">
        ← Все клиенты
      </Link>
      <PageHeader title={`Клиент ${id ?? ''}`} />
      <EmptyState title="Экран в разработке" hint="Карточка клиента появится в задаче F4." />
    </>
  );
}
