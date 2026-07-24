import { Card, Empty, Tooltip } from 'antd'
import { useTranslation } from 'react-i18next'
import type { Finding, Severity } from '../types'
import { SEVERITY_LABEL_KEY } from '../lib/severity'
import { ruleNameById } from '../lib/i18n-names'

const sevWeight: Record<Severity, number> = { critical: 5, high: 4, medium: 3, low: 2, info: 1 }

export function TopRiskTypes({ findings, topN = 10 }: { findings: Finding[]; topN?: number }) {
  const { t } = useTranslation()
  // 按 rule_id 分组:取组内最高严重度 + 计数 + 保留一条 message(供双语名回退)。
  const groups = new Map<string, { sev: Severity; count: number; msg: string }>()
  for (const f of findings) {
    const g = groups.get(f.rule_id)
    if (g) {
      g.count++
      if (sevWeight[f.severity] > sevWeight[g.sev]) g.sev = f.severity
    } else {
      groups.set(f.rule_id, { sev: f.severity, count: 1, msg: f.message })
    }
  }
  // 排序:严重度权重降序,次按计数降序,取 Top N。
  const rows = [...groups.entries()]
    .map(([id, g]) => ({ id, ...g }))
    .sort((a, b) => sevWeight[b.sev] - sevWeight[a.sev] || b.count - a.count)
    .slice(0, topN)
  const maxCount = rows.reduce((m, r) => Math.max(m, r.count), 1)
  return (
    <Card title={t('chart.topRiskTitle')}>
      {rows.length === 0 ? <Empty description={t('chart.topRiskEmpty')} /> : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {rows.map((r) => (
            <div key={r.id} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              {/* 风险类型名:取规则双语名(ruleNameById:先 i18n rules.<rule_id>,回退 finding.message);
                  Tooltip 兜底显示原始 rule_id 便于定位规则。 */}
              <Tooltip title={r.id}>
                <span style={{ width: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 'var(--fs-xs)', color: 'var(--color-muted)' }}>{ruleNameById(r.id, r.msg)}</span>
              </Tooltip>
              <div style={{ flex: 1, background: 'var(--color-rule)', borderRadius: 'var(--radius-sm)', height: 12, overflow: 'hidden' }}>
                <div data-testid={`toprisk-${r.id}`} style={{ width: `${(r.count / maxCount) * 100}%`, minWidth: 8, height: '100%', background: `var(--sev-${r.sev})` }} />
              </div>
              <span style={{ width: 60, textAlign: 'right', fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', color: `var(--sev-${r.sev})`, fontVariantNumeric: 'tabular-nums' }}>{t(SEVERITY_LABEL_KEY[r.sev])} {r.count}</span>
            </div>
          ))}
        </div>
      )}
    </Card>
  )
}
