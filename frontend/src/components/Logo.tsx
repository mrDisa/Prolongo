// Prolongo mark: the ring is the renewal cycle, the gap is the moment a customer may not renew,
// the dot is Prolongo catching it. Same drawing as public/favicon.svg, but in theme colors.
export function LogoMark({ size = 38 }: { size?: number }) {
  return (
    <svg viewBox="0 0 32 32" width={size} height={size} aria-hidden="true" className="shrink-0 text-accent-ink">
      <rect width="32" height="32" rx="9" className="fill-accent" />
      <path
        d="M23.57 16.66 A7.6 7.6 0 1 1 15.34 8.43"
        fill="none"
        stroke="currentColor"
        strokeWidth="4.4"
        strokeLinecap="round"
      />
      <circle cx="21.37" cy="10.63" r="2.2" fill="currentColor" />
    </svg>
  );
}

export function Logo() {
  return (
    <div className="flex items-center gap-2.5">
      <LogoMark />
      <span className="font-display text-[19px] font-semibold">Prolongo</span>
    </div>
  );
}
