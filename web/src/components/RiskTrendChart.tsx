import { Card, Empty } from 'antd'
import { useTranslation } from 'react-i18next'
import type { ScanSummary } from '../types'
import { formatDateTimeShort } from '../lib/format'

// 风险指数趋势:历史扫描 health_score 随时间折线。纯前端聚合 store.history(已按时间倒序)。
// SVG 自绘:不引依赖。明暗用 CSS 变量。y 轴 0-100(健康分范围),x 轴时间。
export function RiskTrendChart({ history }: { history: ScanSummary[] }) {
  const { t } = useTranslation()
  // 升序(旧→新)绘趋势。
  const pts = [...history].sort((a, b) => a.started_at.localeCompare(b.started_at))
  if (pts.length === 0) {
    return <Card title={t('chart.riskTrendTitle')}><Empty description={t('chart.riskTrendEmpty')} /></Card>
  }
  const W = 600, H = 180, P = 32
  const xStep = pts.length > 1 ? (W - 2 * P) / (pts.length - 1) : 0
  const y = (s: number) => H - P - (s / 100) * (H - 2 * P)
  const x = (i: number) => P + i * xStep
  const line = pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i)},${y(p.health_score)}`).join(' ')
  const area = `${line} L${x(pts.length - 1)},${H - P} L${x(0)},${H - P} Z`
  return (
    <Card title={t('chart.riskTrendTitle')}>
      <svg viewBox={`0 0 ${W} ${H}`} style={{ width: '100%', height: 200 }} role="img" aria-label={t('chart.riskTrendAria')}>
        {/* y 轴参考线 0/50/100 */}
        {[0, 50, 100].map((v) => (
          <g key={v}>
            <line x1={P} y1={y(v)} x2={W - P} y2={y(v)} stroke="var(--bg-border)" strokeWidth={1} />
            <text x={4} y={y(v) + 4} fontSize={10} fill="var(--text-muted)">{v}</text>
          </g>
        ))}
        <path d={area} fill="var(--accent)" opacity={0.12} />
        <path d={line} fill="none" stroke="var(--accent)" strokeWidth={2} />
        {pts.map((p, i) => (
          <circle key={p.id} cx={x(i)} cy={y(p.health_score)} r={3} fill="var(--accent)">
            <title>{`${formatDateTimeShort(p.started_at)}: ${p.health_score}`}</title>
          </circle>
        ))}
        {/* 首尾时间标签 */}
        <text x={P} y={H - 8} fontSize={10} fill="var(--text-muted)">{formatDateTimeShort(pts[0].started_at)}</text>
        {pts.length > 1 ? <text x={W - P} y={H - 8} fontSize={10} fill="var(--text-muted)" textAnchor="end">{formatDateTimeShort(pts[pts.length - 1].started_at)}</text> : null}
      </svg>
    </Card>
  )
}
