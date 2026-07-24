import { Card, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import type { HealthScore } from '../types'
import { AgentIcon } from './AgentIcon'

function bandColor(score: number | undefined): string {
  if (score === undefined) return 'var(--color-dim)'
  if (score >= 80) return 'var(--sev-low)'
  if (score >= 60) return 'var(--sev-medium)'
  if (score >= 40) return 'var(--sev-high)'
  return 'var(--sev-critical)'
}

// 健康分卡(design.md component-scope · 方向 C):大数字为主角 + 底部水平进度条,环去掉。
// 拥挤根因:原「环 + 数字」并排,两个主角争抢同一水平线 → 改成单一视觉重心(数字),
// 进度条退到底部 hairline 轨上,既保留「健康度」隐喻又不抢地盘。Linear/Notion KPI 卡形态。
// 可选 agentId/agentName → 卡标题即 agent 身份(design.md #8:去掉卡中卡)。
// 固定宽度(flex:0 1,不拉满行),多 agent 并排、单 agent 仅占一列(design.md #1)。
export function HealthScoreCard({
  h,
  agentId,
  agentName,
  lastScanAt,
  notScannedHint,
}: {
  h?: HealthScore
  agentId?: string
  agentName?: string
  lastScanAt?: string
  notScannedHint?: string
}) {
  const { t } = useTranslation()
  const score = h?.score
  // 未扫描(score=undefined)→ 进度 0,数字显示 '--';否则 clamp 到 [0,100]。
  const pct = score === undefined ? 0 : Math.max(0, Math.min(100, score))
  const color = bandColor(score)
  // 有 agent 信息 → 标题放 agent logo+名;否则用通用「健康分」标题(单 agent 无聚合时)。
  const title = agentName ? (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
      {agentId ? <AgentIcon id={agentId} /> : null}
      <span>{agentName}</span>
    </span>
  ) : t('health.title')
  return (
    <Card
      title={title}
      style={{ flex: '0 1 240px', width: 240, display: 'flex', flexDirection: 'column' }}
      styles={{ body: { display: 'flex', flexDirection: 'column', gap: 'var(--space-md)', flex: 1, padding: 'var(--space-md) var(--space-lg)' } }}
    >
      {/* 主区:大数字 + band 同行,数字独占视觉重心;band 作为同色次级标签贴在数字旁。 */}
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 'var(--space-sm)' }}>
        <div data-testid="health-score-value" style={{ fontSize: 'var(--fs-2xl)', fontWeight: 700, color: 'var(--color-ink)', fontFamily: 'var(--font-mono)', lineHeight: 1, fontVariantNumeric: 'tabular-nums' }}>
          {score === undefined ? '--' : score}
        </div>
        <Typography.Text style={{ color, fontWeight: 600, fontSize: 'var(--fs-sm)' }}>
          {h?.band ?? '--'}
        </Typography.Text>
      </div>
      {/* 上次扫描时间:agent 名已在标题,这里作为次级元信息。 */}
      {lastScanAt || notScannedHint ? (
        <div style={{ fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', fontFamily: 'var(--font-mono)', fontVariantNumeric: 'tabular-nums' }}>
          {lastScanAt ?? notScannedHint}
        </div>
      ) : null}
      {/* 底部水平进度条:整条 rule 色轨 + 按分数比例填 band 色。无动画(motion-cut)。
          未扫描时 pct=0 → 仅显示空轨,数字已是 '--',视觉一致。 */}
      <div style={{ marginTop: 'auto', height: 6, borderRadius: 'var(--radius-pill)', background: 'var(--color-rule)', overflow: 'hidden' }}>
        <div data-testid="health-score-bar" style={{ width: `${pct}%`, height: '100%', borderRadius: 'var(--radius-pill)', background: color }} />
      </div>
    </Card>
  )
}
