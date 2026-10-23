import { Logo } from '@/components/Logo';

export default function Login() {
  return (
    <div className="flex flex-col gap-6">
      <Logo />
      <div>
        <h1 className="font-display text-[30px] font-semibold leading-[38px] tracking-[-0.02em]">Вход</h1>
        <p className="mt-1.5 text-ink-2">Форма входа появится в задаче F2.</p>
      </div>
    </div>
  );
}
