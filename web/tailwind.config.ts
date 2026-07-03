import type { Config } from 'tailwindcss'
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: { DEFAULT: '#0b0f17', card: '#121826', border: '#1f2937' },
        sev: { critical: '#ef4444', high: '#f97316', medium: '#f59e0b', low: '#14b8a6' },
      },
      fontFamily: { mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'] },
    },
  },
  plugins: [],
} satisfies Config
