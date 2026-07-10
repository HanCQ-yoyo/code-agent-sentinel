import { useState, type HTMLAttributes } from 'react'
import { Card, Table, Segmented, Typography, Empty, Tooltip } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { Finding, Severity, DetectorMeta } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { formatDateTime } from '../lib/format'

const order: Severity[] = ['critical', 'high', 'medium', 'low']
const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }
// 筛选标签内的色点颜色(复用 sev token);「全部」用 accent。
const sevDot: Record<Severity, string> = {
  critical: 'var(--sev-critical)', high: 'var(--sev-high)', medium: 'var(--sev-medium)', low: 'var(--sev-low)',
}

// 级别筛选标签:左侧色点 + 文本 + 计数。色点颜色对应级别,选中时整块填该级别色(见 .sev-seg CSS),
// 与未选中的透明底+色点形成明显差别。
function SevSegLabel({ text, count, sev }: { text: string; count: number; sev?: Severity }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
      <span
        className="sev-seg-dot"
        style={{ width: 8, height: 8, borderRadius: '50%', background: sev ? sevDot[sev] : 'var(--accent)' }}
      />
      <span>{text}</span>
      <span className="sev-seg-count" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{count}</span>
    </span>
  )
}

interface FindingTableProps {
  findings: Finding[]
  // 整次扫描起始时间(同一次扫描所有行共享)。可选:无 scan 时间时不显示该列内容。
  startedAt?: string
  // 检测器元数据,供按 detector_id 查中文名;无则显示 detector_id。
  detectors?: DetectorMeta[]
  // 行点击 → 打开详情抽屉。
  onSelect?: (f: Finding) => void
}

export function FindingTable({ findings, startedAt, detectors, onSelect }: FindingTableProps) {
  const [filter, setFilter] = useState<Severity | 'all'>('all')
  const counts: Record<string, number> = { all: findings.length }
  for (const s of order) counts[s] = findings.filter((f) => f.severity === s).length
  const shown = filter === 'all' ? findings : findings.filter((f) => f.severity === filter)
  const sorted = [...shown].sort((a, b) => order.indexOf(a.severity) - order.indexOf(b.severity))

  // detector_id → 中文名(供检测器列显示;无匹配则回退 id)。
  const detName = (id: string): string => {
    const d = detectors?.find((x) => x.id === id)
    return d?.name ?? id
  }

  const columns: ColumnsType<Finding> = [
    {
      // 风险名称:不设固定宽度,作为弹性主列占据剩余空间并省略;资产列收窄后这里更宽。
      title: '风险名称', ellipsis: true, render: (_: unknown, f: Finding) => (
        <Tooltip title={f.message}>
          <span>{f.message}</span>
        </Tooltip>
      ),
    },
    {
      // 资产:文件名 + 类型两词,收窄到 140;长名省略,Tooltip 兜底。
      title: '资产', width: 140, ellipsis: true, render: (_: unknown, f: Finding) => (
        <Tooltip title={`${f.asset_name} ${f.asset_type}`}>
          <span>{f.asset_name} <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{f.asset_type}</Typography.Text></span>
        </Tooltip>
      ),
    },
    { title: '级别', width: 80, render: (_: unknown, f: Finding) => <SevBadge tone={`sev-${f.severity}` as BadgeTone}>{sevLabel[f.severity]}</SevBadge> },
    {
      title: '检测器', width: 120, render: (_: unknown, f: Finding) => (
        <Typography.Text style={{ fontSize: 12 }}>{detName(f.detector_id)}</Typography.Text>
      ),
    },
    {
      // 规则列加宽 1 倍(160→320),容纳完整 rule_id mono 文本,不再截断;字体放大到 13 便于阅读。
      title: '规则', width: 320, render: (_: unknown, f: Finding) => (
        <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 13 }}>{f.rule_id}</Typography.Text>
      ),
    },
    {
      title: '扫描时间', width: 150, render: () => (
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{startedAt ? formatDateTime(startedAt) : '--'}</span>
      ),
    },
  ]

  return (
    <Card>
      <Segmented
        className="sev-seg"
        style={{ marginBottom: 12 }}
        value={filter}
        onChange={(v) => setFilter(v as Severity | 'all')}
        options={[
          { value: 'all', label: <SevSegLabel text="全部" count={counts.all} />, className: 'sev-tab-all' },
          ...order.map((s) => ({
            value: s,
            label: <SevSegLabel text={sevLabel[s]} count={counts[s]} sev={s} />,
            className: `sev-tab-${s}`,
          })),
        ]}
      />
      <Table<Finding>
        rowKey={(_f, i) => String(i)}
        columns={columns}
        dataSource={sorted}
        pagination={false}
        size="middle"
        // 行点击打开抽屉;保留 finding-row testid(e2e [data-testid="finding-row"] 硬约束)。
        // onClick 经 onRow 注入;data-testid 同理(参考 AssetTable onRow 模式)。
        onRow={(f) => ({
          'data-testid': 'finding-row',
          onClick: () => onSelect?.(f),
          style: onSelect ? { cursor: 'pointer' } : undefined,
        }) as HTMLAttributes<HTMLElement>}
        locale={{ emptyText: findings.length === 0 ? <Empty description="暂无发现 · 扫描后显示" /> : '无该级别发现' }}
      />
    </Card>
  )
}
