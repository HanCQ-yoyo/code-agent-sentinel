import { useState, useMemo } from 'react'
import { Table, Segmented, Empty, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { DetectorMeta, Severity } from '../types'
import { Badge, type BadgeTone } from './Badge'

type FlatRule = { id: string; severity: Severity; description: string; detector: string; detector_id: string }

// 规则总览:汇总所有检测器的规则,按 sev 筛选。规则号 mono,sev 标签,检测器名,说明。
export function RulesTable({ detectors }: { detectors: DetectorMeta[] }) {
  const [sev, setSev] = useState<string>('all')

  const allRules = useMemo<FlatRule[]>(
    () => detectors.flatMap((d) => (d.rules ?? []).map((r) => ({ ...r, detector: d.name, detector_id: d.id }))),
    [detectors]
  )

  const counts = useMemo(() => {
    const c: Record<string, number> = { all: allRules.length, critical: 0, high: 0, medium: 0, low: 0 }
    for (const r of allRules) c[r.severity] = (c[r.severity] ?? 0) + 1
    return c
  }, [allRules])

  const filtered = sev === 'all' ? allRules : allRules.filter((r) => r.severity === sev)

  const columns: ColumnsType<FlatRule> = [
    { title: '级别', width: 90, dataIndex: 'severity', render: (s: Severity) => <Badge tone={`sev-${s}` as BadgeTone}>{s}</Badge> },
    { title: '检测器', width: 120, dataIndex: 'detector' },
    { title: '规则号', width: 200, dataIndex: 'id', render: (id: string) => <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{id}</Typography.Text> },
    { title: '说明', dataIndex: 'description' },
  ]

  const options = [
    { value: 'all', label: `全部 ${counts.all}` },
    { value: 'critical', label: `严重 ${counts.critical ?? 0}` },
    { value: 'high', label: `高 ${counts.high ?? 0}` },
    { value: 'medium', label: `中 ${counts.medium ?? 0}` },
    { value: 'low', label: `低 ${counts.low ?? 0}` },
  ]

  if (allRules.length === 0) return <Empty description="暂无规则" />

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Segmented value={sev} onChange={(v) => setSev(v as string)} options={options} />
      <Table<FlatRule> rowKey={(r) => `${r.detector_id}:${r.id}`} columns={columns} dataSource={filtered} pagination={{ pageSize: 50, size: 'small' }} size="small" />
    </div>
  )
}
