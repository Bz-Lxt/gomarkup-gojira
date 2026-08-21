/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: 'var(--ink)',
        'ink-2': 'var(--ink-2)',
        'ink-3': 'var(--ink-3)',
        paper: 'var(--paper)',
        'paper-dim': 'var(--paper-dim)',
        copper: 'var(--copper)',
        'copper-deep': 'var(--copper-deep)',
        teal: 'var(--teal)',
        gold: 'var(--gold)',
        olive: 'var(--olive)',
        rose: 'var(--rose)',
        line: 'var(--line)',
      },
      fontFamily: {
        display: ['Fraunces', 'Georgia', 'serif'],
        sans: ['"Source Sans 3"', 'ui-sans-serif', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'monospace'],
      },
      borderRadius: {
        card: '10px',
      },
      boxShadow: {
        lift: '0 8px 24px rgba(0,0,0,0.28)',
        paper: '0 10px 28px rgba(18,21,28,0.35)',
      },
      transitionDuration: {
        page: '180ms',
        settle: '160ms',
        drawer: '220ms',
      },
    },
  },
  plugins: [],
}
