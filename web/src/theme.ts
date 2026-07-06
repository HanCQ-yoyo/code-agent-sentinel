import { useEffect, useState } from 'react'

type Theme = 'light' | 'dark'
const KEY = 'sentinel_theme'

function detect(): Theme {
  const saved = localStorage.getItem(KEY)
  if (saved === 'light' || saved === 'dark') return saved
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => detect())
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem(KEY, theme)
  }, [theme])
  const toggle = () => setTheme(t => (t === 'dark' ? 'light' : 'dark'))
  return { theme, toggle }
}
