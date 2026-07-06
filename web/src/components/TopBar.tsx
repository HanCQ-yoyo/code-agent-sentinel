import { useTheme } from '../theme'
import type { DetectorStatus } from '../types'

interface Props {
  title: string
  onScan: () => void
  loading: boolean
  detectors: DetectorStatus[]
}
export function TopBar({ title, onScan, loading, detectors }: Props) {
  const { theme, toggle } = useTheme()
  const avail = detectors.filter(d => d.available).length
  const total = detectors.length
  return (
    <header className="flex items-center justify-between border-b border-bg-border px-6 py-3 bg-bg-card">
      <h1 className="text-lg font-semibold">{title}</h1>
      <div className="flex items-center gap-4">
        <span className="text-sm text-text-muted" data-testid="detector-summary">
          检测器 {avail}/{total}
        </span>
        <button
          onClick={toggle}
          className="px-3 py-1.5 rounded-md border border-bg-border text-sm hover:bg-bg-border"
          aria-label="切换主题"
        >
          {theme === 'dark' ? '☀ 浅色' : '☾ 深色'}
        </button>
        <button
          onClick={onScan}
          disabled={loading}
          className="px-4 py-1.5 rounded-md bg-accent text-white text-sm font-semibold disabled:opacity-50"
        >
          {loading ? '扫描中…' : '↻ 重新扫描'}
        </button>
      </div>
    </header>
  )
}
