import { Link } from 'react-router-dom';
import { EmptyState } from '@/components/EmptyState';
import { PageHeader } from '@/components/PageHeader';

export default function NotFound() {
  return (
    <>
      <PageHeader title="Страница не найдена" />
      <EmptyState
        title="Такого адреса нет"
        hint="Проверьте ссылку или вернитесь на главную."
        action={
          <Link
            to="/"
            className="inline-flex h-[46px] items-center rounded-field bg-accent px-5 font-semibold text-accent-ink"
          >
            На главную
          </Link>
        }
      />
    </>
  );
}
