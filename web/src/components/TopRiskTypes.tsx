import { Card, Empty } from 'antd'
import type { Finding, Severity } from '../types'
import { SEVERITY_LABEL } from '../lib/severity'

const sevWeight: Record<Severity, number> = { critical: 5, high: 4, medium: 3, low: 2, info: 1 }

export function TopRiskTypes({ findings, topN = 10 }: { findings: Finding[]; topN?: number }) {
  // 按 rule_id 分组:取组内最高严重度 + 计数。
  const groups = new Map<string, { sev: Severity; count: number }>()
  for (const f of findings) {
    const g = groups.get(f.rule_id)
    if (g) {
      g.count++
      if (sevWeight[f.severity] > sevWeight[g.sev]) g.sev = f.severity
    } else {
      groups.set(f.rule_id, { sev: f.severity, count: 1 })
    }
  }
  // 排序:严重度权重降序,次按计数降序,取 Top N。
  const rows = [...groups.entries()]
    .map(([id, g]) => ({ id, ...g }))
    .sort((a, b) => sevWeight[b.sev] - sevWeight[a.sev] || b.count - a.count)
    .slice(0, topN)
  const maxCount = rows.reduce((m, r) => Math.max(m, r.count), 1)
  return (
    <Card title="Top 风险类型(按严重度)">
      {rows.length === 0 ? <Empty description="暂无发现" /> : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {rows.map((r) => (
            <div key={r.id} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span title={r.id} style={{ width: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>{r.id}</span>
              <div style={{ flex: 1, background: 'var(--bg-border)', borderRadius: 4, height: 12, overflow: 'hidden' }}>
                <div data-testid={`toprisk-${r.id}`} style={{ width: `${(r.count / maxCount) * 100}%`, minWidth: 8, height: '100%', background: `var(--sev-${r.sev})` }} />
              </div>
              <span style={{ width: 60, textAlign: 'right', fontFamily: 'var(--font-mono)', fontSize: 11, color: `var(--sev-${r.sev})` }}>{SEVERITY_LABEL[r.sev]} {r.count}</span>
            </div>
          ))}
        </div>
      )}
    </Card>
  )
}
