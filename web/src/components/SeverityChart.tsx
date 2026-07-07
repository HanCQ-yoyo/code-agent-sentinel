import { Card } from 'antd'
import type { Finding, Severity } from '../types'

const order: Severity[] = ['critical', 'high', 'medium', 'low']
const label: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }

export function SeverityChart({ findings }: { findings: Finding[] }) {
  const counts: Record<Severity, number> = { critical: 0, high: 0, medium: 0, low: 0 }
  for (const f of findings) counts[f.severity] = (counts[f.severity] ?? 0) + 1
  const total = findings.length || 1
  return (
    <Card title="严重度分布">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {order.map((s) => (
          <div key={s} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={{ width: 32, color: 'var(--text-muted)' }}>{label[s]}</span>
            <div style={{ flex: 1, background: 'var(--bg-border)', borderRadius: 4, height: 12, overflow: 'hidden' }}>
              <div
                data-testid={`severity-${s}`}
                style={{ width: `${(counts[s] / total) * 100}%`, minWidth: counts[s] > 0 ? 8 : 0, height: '100%', background: `var(--sev-${s})` }}
              />
            </div>
            <span style={{ width: 28, textAlign: 'right', color: 'var(--text)', fontFamily: 'var(--font-mono)' }}>{counts[s]}</span>
          </div>
        ))}
      </div>
    </Card>
  )
}
