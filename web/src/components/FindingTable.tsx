import { useState, type HTMLAttributes } from 'react'
import { Card, Table, Segmented, Badge, Typography, Empty } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { Finding, Severity } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'

const order: Severity[] = ['critical', 'high', 'medium', 'low']
const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }

export function FindingTable({ findings }: { findings: Finding[] }) {
  const [filter, setFilter] = useState<Severity | 'all'>('all')
  const counts: Record<string, number> = { all: findings.length }
  for (const s of order) counts[s] = findings.filter((f) => f.severity === s).length
  const shown = filter === 'all' ? findings : findings.filter((f) => f.severity === filter)
  const sorted = [...shown].sort((a, b) => order.indexOf(a.severity) - order.indexOf(b.severity))

  const columns: ColumnsType<Finding> = [
    { title: '级别', width: 80, render: (_: unknown, f: Finding) => <SevBadge tone={`sev-${f.severity}` as BadgeTone}>{sevLabel[f.severity]}</SevBadge> },
    { title: '资产', render: (_: unknown, f: Finding) => <span>{f.asset_name} <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{f.asset_type}</Typography.Text></span> },
    { title: '规则', width: 160, render: (_: unknown, f: Finding) => <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{f.rule_id}</Typography.Text> },
    { title: '说明', render: (_: unknown, f: Finding) => <span>{f.message}<br /><Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{f.evidence.slice(0, 120)}</Typography.Text></span> },
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
        // 保留 finding-row testid 供 e2e [data-testid="finding-row"] 选择器使用(计划全局约束)。
        // cast:@types/react 5.5 的 DOMAttributes 无 data-* 索引签名,弱类型校验需显式断言;
        // 参考 AssetTable onRow(带 onClick 时通过)。FindingTable 行不可点击,故用断言而非加伪 onClick。
        onRow={() => ({ 'data-testid': 'finding-row' }) as HTMLAttributes<HTMLElement>}
        locale={{ emptyText: findings.length === 0 ? <Empty description="暂无发现 · 扫描后显示" /> : '无该级别发现' }}
      />
    </Card>
  )
}
