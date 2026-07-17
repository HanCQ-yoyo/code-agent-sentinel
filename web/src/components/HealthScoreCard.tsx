import { Card, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import type { HealthScore } from '../types'

function bandColor(score: number | undefined): string {
  if (score === undefined) return 'var(--text-dim)'
  if (score >= 80) return 'var(--sev-low)'
  if (score >= 60) return 'var(--sev-medium)'
  if (score >= 40) return 'var(--sev-high)'
  return 'var(--sev-critical)'
}

export function HealthScoreCard({ h }: { h?: HealthScore }) {
  const { t } = useTranslation()
  const score = h?.score
  const pct = score === undefined ? 0 : Math.max(0, Math.min(100, score))
  const r = 52
  const c = 2 * Math.PI * r
  return (
    // 垂直居中根因:antd Card 根 .ant-card 默认非 flex,旧实现给 body 设 height:'100%'
    // 解析为整张卡片高度(含标题区),居中内容被推到中线下方 → 用户反馈"偏下"。
    // 修复:根 .ant-card 设 display:flex + flexDirection:column,使标题占自然高、
    // body 用 flex:1 占剩余高(减去标题),alignItems:'center' 在 body 剩余区内真正垂直居中。
    <Card
      title={t('health.title')}
      style={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column' }}
      styles={{ body: { display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 20, flex: 1, minHeight: 0 } }}
    >
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
