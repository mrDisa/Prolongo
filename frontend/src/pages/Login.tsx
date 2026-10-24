import { zodResolver } from '@hookform/resolvers/zod';
import { Eye, EyeOff, Lock } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { z } from 'zod';
import { ApiError } from '@/api/errors';
import { useLogin } from '@/api/mutations';
import { useAuthConfig, useMe } from '@/api/queries';
import { Logo } from '@/components/Logo';
import { ThemeToggle } from '@/components/ThemeToggle';
import { safeNext } from '@/lib/auth';
import { cn } from '@/lib/utils';

const schema = z.object({
  username: z.string().trim().min(1, 'Введите логин'),
  password: z.string().min(1, 'Введите пароль'),
});

type FormValues = z.infer<typeof schema>;

const FIELD_BASE =
  'h-[52px] w-full rounded-field border bg-surface px-4 text-[16px] text-ink placeholder:text-ink-3';

function describeError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401 || error.status === 422) {
      return error.message;
    }
    return 'Сервер ответил ошибкой. Попробуйте ещё раз чуть позже.';
  }
  return 'Не удалось связаться с сервером. Проверьте соединение.';
}

export default function Login() {
  const [params] = useSearchParams();
  const next = safeNext(params.get('next'));
  const navigate = useNavigate();
  const me = useMe();
  // The server says whether SSO is on (OIDC_ENABLED), so the image needs no rebuild to switch it.
  const oidcEnabled = useAuthConfig().data?.oidcEnabled ?? false;
  const login = useLogin();
  const [showPassword, setShowPassword] = useState(false);
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema) });

  // Already signed in: nothing to do here.
  if (me.data) {
    return <Navigate to={next} replace />;
  }

  const onSubmit = handleSubmit((values) => {
    login.mutate(values, { onSuccess: () => navigate(next, { replace: true }) });
  });

  return (
    <div className="flex min-h-screen">
      <section className="relative hidden flex-1 basis-[560px] flex-col justify-between gap-12 overflow-hidden border-r border-line bg-surface px-14 py-12 min-[900px]:flex">
        <div aria-hidden="true" className="absolute bottom-0 right-12 top-0 flex gap-[18px]">
          <span className="w-[22px] bg-stripe" />
          <span className="w-[22px] bg-stripe" />
          <span className="w-[22px] bg-stripe" />
        </div>
        <div className="relative">
          <Logo />
        </div>
        <div className="relative flex max-w-[560px] flex-col gap-7 pr-[120px]">
          <h2 className="font-display text-[42px] font-bold uppercase leading-[50px] tracking-[-0.03em]">
            Удержать клиента до того, как он уйдёт
          </h2>
          <div className="flex flex-col gap-3.5 text-[17px] text-ink-2">
            <span>
              <b className="text-accent-text">Кому</b> нужно внимание — рейтинг по risk-score
            </span>
            <span>
              <b className="text-accent-text">Почему</b> — каждая причина в баллах
            </span>
            <span>
              <b className="text-accent-text">Сколько</b> потеряем — в евро, с учётом балансов
            </span>
          </div>
        </div>
        <span className="relative text-[13px] text-ink-3">
          Хакатон РНД · кейс «Анализатор рисков оттока»
        </span>
      </section>

      <div className="relative flex flex-1 basis-[480px] items-center justify-center px-6 pb-12 pt-24 min-[900px]:py-12">
        <ThemeToggle className="absolute right-6 top-6" />

        <form noValidate onSubmit={onSubmit} className="flex w-full max-w-[400px] flex-col gap-5">
          <div className="min-[900px]:hidden">
            <Logo />
          </div>

          <div>
            <h1 className="font-display text-[30px] font-semibold leading-[38px] tracking-[-0.02em]">
              Вход
            </h1>
            <p className="mt-1.5 text-ink-2">Панель для маркетологов. Доступ — только сотрудникам</p>
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="username" className="text-[14px] font-semibold">
              Логин
            </label>
            <input
              id="username"
              type="text"
              autoComplete="username"
              aria-invalid={errors.username ? true : undefined}
              aria-describedby={errors.username ? 'username-error' : undefined}
              className={cn(FIELD_BASE, errors.username ? 'border-high' : 'border-line-strong')}
              {...register('username')}
            />
            {errors.username ? (
              <span id="username-error" className="text-[13px] font-medium text-high-text">
                {errors.username.message}
              </span>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="password" className="text-[14px] font-semibold">
              Пароль
            </label>
            <span
              className={cn(
                'flex h-[52px] items-center rounded-field border bg-surface pl-4 pr-2 focus-within:outline focus-within:outline-2 focus-within:outline-accent',
                errors.password ? 'border-high' : 'border-line-strong',
              )}
            >
              <input
                id="password"
                type={showPassword ? 'text' : 'password'}
                autoComplete="current-password"
                aria-invalid={errors.password ? true : undefined}
                aria-describedby={errors.password ? 'password-error' : undefined}
                className="min-w-0 flex-1 bg-transparent text-[16px] text-ink focus-visible:outline-none"
                {...register('password')}
              />
              <button
                type="button"
                aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
                aria-pressed={showPassword}
                onClick={() => setShowPassword((shown) => !shown)}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-seg text-ink-3 hover:bg-surface-2"
              >
                {showPassword ? (
                  <EyeOff size={18} strokeWidth={2} aria-hidden="true" />
                ) : (
                  <Eye size={18} strokeWidth={2} aria-hidden="true" />
                )}
              </button>
            </span>
            {errors.password ? (
              <span id="password-error" className="text-[13px] font-medium text-high-text">
                {errors.password.message}
              </span>
            ) : null}
          </div>

          <button
            type="submit"
            disabled={login.isPending}
            className="flex h-[54px] items-center justify-center rounded-field bg-accent text-[16px] font-semibold text-accent-ink disabled:cursor-not-allowed disabled:opacity-60"
          >
            {login.isPending ? 'Входим…' : 'Войти'}
          </button>

          {login.isError ? (
            <p
              role="alert"
              className="rounded-tile bg-high-soft px-4 py-3 text-[14px] font-medium text-high-text"
            >
              {describeError(login.error)}
            </p>
          ) : null}

          {oidcEnabled ? (
            <>
              <div className="flex items-center gap-3 text-[13px] text-ink-3">
                <span className="h-px flex-1 bg-line" />
                или
                <span className="h-px flex-1 bg-line" />
              </div>
              <a
                href="/api/v1/auth/oidc/login"
                className="flex h-[54px] items-center justify-center gap-2 rounded-field border border-line-strong bg-surface text-[16px] font-semibold text-ink"
              >
                <Lock size={18} strokeWidth={2} aria-hidden="true" />
                Войти через SSO
              </a>
            </>
          ) : null}
        </form>
      </div>
    </div>
  );
}
