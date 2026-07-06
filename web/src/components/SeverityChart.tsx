import type { Finding, Severity } from '../types'

const order: Severity[] = ['critical', 'high', 'medium', 'low']
const labels: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }

export function SeverityChart({ findings }: { findings: Finding[] }) {
  const counts = order.reduce((acc, s) => {
    acc[s] = findings.filter(f => f.severity === s).length
    return acc
  }, {} as Record<Severity, number>)
  const total = findings.length || 1
  return (
    <div className="bg-bg-card border border-bg-border rounded-xl p-5">
      <div className="text-sm text-text-muted mb-3">严重度分布</div>
      <div className="space-y-2">
        {order.map(s => (
          <div key={s} className="flex items-center gap-3">
            <span className="w-10 text-sm text-text">{labels[s]}</span>
            <div className="flex-1 h-6 rounded bg-bg-border overflow-hidden">
              {/* sev 色仅作色条填充(标记色,非文本),配标签读取。 */}
              <div
                data-testid={`severity-${s}`}
                className="h-full"
                style={{ width: `${(counts[s] / total) * 100}%`, background: `var(--sev-${s})`, minWidth: counts[s] > 0 ? '8px' : 0 }}
              />
            </div>
            <span className="w-8 text-right text-sm tabular-nums text-text">{counts[s]}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
