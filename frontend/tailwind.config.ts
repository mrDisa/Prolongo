import type { Config } from 'tailwindcss';

// Every color points to a CSS variable from src/styles/tokens.css,
// so components never contain raw hex values.
const v = (name: string) => `var(--${name})`;

const level = (name: string) => ({
  DEFAULT: v(name),
  soft: v(`${name}-soft`),
  text: v(`${name}-text`),
});

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: v('bg'),
        surface: v('surface'),
        'surface-2': v('surface-2'),
        line: v('line'),
        'line-strong': v('line-strong'),
        ink: v('text'),
        'ink-2': v('text-2'),
        'ink-3': v('text-3'),
        accent: {
          DEFAULT: v('accent'),
          ink: v('accent-ink'),
          soft: v('accent-soft'),
          text: v('accent-text'),
        },
        track: v('track'),
        hatch: v('hatch'),
        stripe: v('stripe'),
        high: level('high'),
        med: level('med'),
        low: level('low'),
        // "none" would clash with Tailwind's built-in bg-none utility.
        unrated: level('none'),
        zone: {
          low: v('zone-low'),
          med: v('zone-med'),
          high: v('zone-high'),
        },
      },
      fontFamily: {
        sans: ['Onest', 'system-ui', 'sans-serif'],
        display: ['Unbounded', 'Onest', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
      },
      borderRadius: {
        field: '14px',
        tile: '16px',
        'tile-lg': '20px',
        block: '24px',
        hero: '28px',
        seg: '10px',
      },
      spacing: {
        sidebar: '232px',
      },
      maxWidth: {
        content: '1440px',
      },
    },
  },
  plugins: [],
} satisfies Config;
