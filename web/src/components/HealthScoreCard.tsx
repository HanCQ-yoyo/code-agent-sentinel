import { Card, Typography } from 'antd'
import type { HealthScore } from '../types'

function bandColor(score: number | undefined): string {
  if (score === undefined) return 'var(--text-dim)'
  if (score >= 80) return 'var(--sev-low)'
  if (score >= 60) return 'var(--sev-medium)'
  if (score >= 40) return 'var(--sev-high)'
  return 'var(--sev-critical)'
}

export function HealthScoreCard({ h }: { h?: HealthScore }) {
  const score = h?.score
  const pct = score === undefined ? 0 : Math.max(0, Math.min(100, score))
  const r = 52
  const c = 2 * Math.PI * r
  return (
    <Card title="健康分" style={{ flex: 1, height: '100%' }} styles={{ body: { display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 20, height: '100%' } }}>
      <svg width={120} height={120} viewBox="0 0 120 120">
        <circle cx={60} cy={60} r={r} fill="none" stroke="var(--bg-border)" strokeWidth={10} />
        <circle
          cx={60}
          cy={60}
          r={r}
          fill="none"
          stroke={bandColor(score)}
          strokeWidth={10}
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={c * (1 - pct / 100)}
          transform="rotate(-90 60 60)"
        />
      </svg>
      <div style={{ textAlign: 'center' }}>
        <div data-testid="health-score-value" style={{ fontSize: 36, fontWeight: 700, color: 'var(--text)', fontFamily: 'var(--font-mono)', lineHeight: 1.1 }}>
          {score === undefined ? '--' : score}
        </div>
        <Typography.Text type="secondary">{h?.band ?? '--'}</Typography.Text>
      </div>
    </Card>
  )
}
