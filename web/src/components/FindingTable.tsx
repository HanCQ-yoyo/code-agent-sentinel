import { useState, type HTMLAttributes } from 'react'
import { Card, Table, Segmented, Typography, Empty, Tooltip } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { Finding, Severity, DetectorMeta } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { formatDateTime } from '../lib/format'

const order: Severity[] = ['critical', 'high', 'medium', 'low']
const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }

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
      title: '风险名称', render: (_: unknown, f: Finding) => (
        <Tooltip title={f.message}>
          <span>{f.message}</span>
        </Tooltip>
      ),
    },
    {
      title: '资产', render: (_: unknown, f: Finding) => (
        <span>{f.asset_name} <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{f.asset_type}</Typography.Text></span>
      ),
    },
    { title: '级别', width: 80, render: (_: unknown, f: Finding) => <SevBadge tone={`sev-${f.severity}` as BadgeTone}>{sevLabel[f.severity]}</SevBadge> },
    {
      title: '检测器', width: 120, render: (_: unknown, f: Finding) => (
        <Typography.Text style={{ fontSize: 12 }}>{detName(f.detector_id)}</Typography.Text>
      ),
    },
    {
      title: '规则', width: 160, render: (_: unknown, f: Finding) => (
        <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{f.rule_id}</Typography.Text>
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
        style={{ marginBottom: 12 }}
        value={filter}
        onChange={(v) => setFilter(v as Severity | 'all')}
        options={[{ value: 'all', label: `全部 ${counts.all}` }, ...order.map((s) => ({ value: s, label: `${sevLabel[s]} ${counts[s]}` }))]}
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
