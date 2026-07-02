import type { Finding } from '../types'

// Tailwind JIT 只扫描源码里的静态完整字符串来决定生成哪些类名,
// 动态拼接的 `bg-sev-${s}` / `text-sev-${s}` 不会被生成 → 条形和图例会无色。
// 故用静态类映射替换动态拼接,视觉设计保持不变。
const SEV_BG: Record<string, string> = {
  critical: 'bg-sev-critical',
  high: 'bg-sev-high',
  medium: 'bg-sev-medium',
  low: 'bg-sev-low',
}
const SEV_TEXT: Record<string, string> = {
  critical: 'text-sev-critical',
  high: 'text-sev-high',
  medium: 'text-sev-medium',
  low: 'text-sev-low',
}

export function SeverityChart({ findings }: { findings: Finding[] }) {
  const counts = { critical: 0, high: 0, medium: 0, low: 0 } as Record<string, number>
  for (const f of findings) counts[f.severity] = (counts[f.severity] ?? 0) + 1
  const total = findings.length || 1
  return (
    <div className="bg-bg-card border border-bg-border rounded-lg p-4">
      <div className="text-sm text-slate-400 mb-2">风险摘要</div>
      <div className="flex h-6 rounded overflow-hidden">
        {['critical','high','medium','low'].map(s => (
          <div key={s} className={SEV_BG[s]} style={{ width: `${(counts[s]/total)*100}%` }} title={`${s}: ${counts[s]}`} />
        ))}
      </div>
      <div className="flex gap-4 mt-2 text-xs">
        {['critical','high','medium','low'].map(s => <span key={s} className={SEV_TEXT[s]}>{s}: {counts[s]}</span>)}
      </div>
    </div>
  )
}
