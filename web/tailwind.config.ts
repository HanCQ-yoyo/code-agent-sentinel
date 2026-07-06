import type { Config } from 'tailwindcss'
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  darkMode: ['class', '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        bg: { DEFAULT: 'var(--bg)', card: 'var(--bg-card)', border: 'var(--bg-border)' },
        text: { DEFAULT: 'var(--text)', muted: 'var(--text-muted)', dim: 'var(--text-dim)' },
        sev: {
          critical: 'var(--sev-critical)',
          high: 'var(--sev-high)',
          medium: 'var(--sev-medium)',
          low: 'var(--sev-low)',
        },
        accent: 'var(--accent)',
      },
      fontFamily: { mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'] },
    },
  },
  plugins: [],
} satisfies Config
