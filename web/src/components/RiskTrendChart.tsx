import { Card, Empty } from 'antd'
import { useTranslation } from 'react-i18next'
import type { ScanSummary } from '../types'
import { formatDateTimeShort } from '../lib/format'
import { agentMetaById } from '../lib/agents'

// 颜色板(固定 hex,需在多线间区分,不用 CSS 变量)。
// 与 antd tag 色系对齐,前 6 色覆盖典型 agent 数量,超出按模 6 轮询。
const COLORS = ['#1677ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2']

// 风险指数趋势:多线版本(Task 10)。history 按 agent_id groupBy,每组一条 <polyline>。
// 关键设计:共享时间轴 —— 取所有 agent 点的全局 min/max started_at,
// 每个点按其时间在 [min,max] 区间内的比例映射到 x。这样不同 agent 扫描次数不同、
// 时间点不同也能在时间轴上正确对齐(而非按各自索引排成等距,导致时间错位)。
// y 轴 0-100(健康分范围)。明暗仍用 CSS 变量(gridlines/text),线色用固定 hex。
export function RiskTrendChart({ history }: { history: ScanSummary[] }) {
  const { t } = useTranslation()
  // 按 agent_id 分组;agent_id 缺失归 'unknown'。
  const byAgent: Record<string, ScanSummary[]> = {}
  for (const s of history) {
    const key = s.agent_id || 'unknown'
    ;(byAgent[key] ??= []).push(s)
  }
  const agentIds = Object.keys(byAgent)
  if (history.length === 0 || agentIds.length === 0) {
    return <Card title={t('chart.riskTrendTitle')}><Empty description={t('chart.riskTrendEmpty')} /></Card>
  }
  // 每组内升序(旧→新)。
  for (const aid of agentIds) {
    byAgent[aid].sort((a, b) => a.started_at.localeCompare(b.started_at))
  }
  // 共享时间轴:全局 min/max started_at(字符串字典序 = ISO8601 时间序)。
  const allTimes = history.map((s) => s.started_at).filter(Boolean)
  const tMin = allTimes.reduce((m, x) => (x < m ? x : m), allTimes[0] ?? '')
  const tMax = allTimes.reduce((m, x) => (x > m ? x : m), allTimes[0] ?? '')
  const W = 600, H = 200, P = 32
  const innerW = W - 2 * P
  const innerH = H - 2 * P
  const y = (s: number) => H - P - (Math.max(0, Math.min(100, s)) / 100) * innerH
  // 时间 → x:范围内单调线性;min/max 相同(单点或同时间)居中。
  const xOf = (ts: string) => {
    if (tMax === tMin) return P + innerW / 2
    // 字符串字典序 = ISO8601 时间序;clamp 到 [0, innerW] 防边界溢出。
    if (ts <= tMin) return P
    if (ts >= tMax) return P + innerW
    const ratio = (Date.parse(ts) - Date.parse(tMin)) / (Date.parse(tMax) - Date.parse(tMin))
    return P + ratio * innerW
  }
  return (
    <Card title={t('chart.riskTrendTitle')}>
      <svg viewBox={`0 0 ${W} ${H}`} style={{ width: '100%', height: 220 }} role="img" aria-label={t('chart.riskTrendAria')}>
        {/* y 轴参考线 0/50/100 */}
        {[0, 50, 100].map((v) => (
          <g key={v}>
            <line x1={P} y1={y(v)} x2={W - P} y2={y(v)} stroke="var(--bg-border)" strokeWidth={1} />
            <text x={4} y={y(v) + 4} fontSize={10} fill="var(--text-muted)">{v}</text>
          </g>
        ))}
        {/* 每 agent 一条折线 + 点(带 <title> tooltip) */}
        {agentIds.map((aid, i) => {
          const color = COLORS[i % COLORS.length]
          const pts = byAgent[aid]
          const line = pts.map((p, idx) => `${idx === 0 ? 'M' : 'L'}${xOf(p.started_at)},${y(p.health_score)}`).join(' ')
          const label = agentMetaById(aid).label
          return (
            <g key={aid}>
              <path d={line} fill="none" stroke={color} strokeWidth={2} />
              {pts.map((p) => (
                <circle key={p.id} cx={xOf(p.started_at)} cy={y(p.health_score)} r={3} fill={color}>
                  <title>{`${label} · ${formatDateTimeShort(p.started_at)}: ${p.health_score}`}</title>
                </circle>
              ))}
            </g>
          )
        })}
        {/* legend:右上角,每 agent 一段色 + 名 */}
        {agentIds.map((aid, i) => {
          const color = COLORS[i % COLORS.length]
          const label = agentMetaById(aid).label
          // 水平排布,每项约 90px 宽,换行容错:P 起步,超宽折下一行。
          const colW = 90
          const perRow = Math.max(1, Math.floor(innerW / colW))
          const row = Math.floor(i / perRow)
          const col = i % perRow
          const lx = P + col * colW
          const ly = 6 + row * 14
          return (
            <g key={`lg-${aid}`}>
              <line x1={lx} y1={ly} x2={lx + 16} y2={ly} stroke={color} strokeWidth={2} />
              <text x={lx + 20} y={ly + 4} fontSize={11} fill="var(--text-secondary)">{label}</text>
            </g>
          )
        })}
        {/* 首尾时间标签(共享轴) */}
        <text x={P} y={H - 8} fontSize={10} fill="var(--text-muted)">{formatDateTimeShort(tMin)}</text>
        {tMax !== tMin ? <text x={W - P} y={H - 8} fontSize={10} fill="var(--text-muted)" textAnchor="end">{formatDateTimeShort(tMax)}</text> : null}
      </svg>
    </Card>
  )
}
