import { useState, useMemo } from 'react'
import { Table, Segmented, Empty, Typography, Card, Tooltip } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { DetectorMeta, Severity } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'

const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }
type FlatRule = { id: string; severity: Severity; description: string; syntax?: string; detector: string; detector_id: string }

// 规则总览:汇总所有检测器的规则,按 sev + 检测器筛选。规则号 mono,sev 标签,检测器名,说明。
// detectorFilter(可选):外部胶囊行点击检测器后传入,只显示该检测器规则。
export function RulesTable({ detectors, detectorFilter }: { detectors: DetectorMeta[]; detectorFilter?: string }) {
  const [sev, setSev] = useState<string>('all')

  const allRules = useMemo<FlatRule[]>(
    () => detectors.flatMap((d) => (d.rules ?? []).map((r) => ({ ...r, detector: d.name, detector_id: d.id }))),
    [detectors]
  )

  // 先按检测器筛选(detectorFilter 来自胶囊行),再据此算 sev 分布 + 二次 sev 筛选。
  // 这样 Segmented 计数随检测器筛选联动,「全部 N」反映当前检测器规则数,而非全局。
  const byDetector = useMemo(
    () => allRules.filter((r) => !detectorFilter || r.detector_id === detectorFilter),
    [allRules, detectorFilter]
  )

  const counts = useMemo(() => {
    const c: Record<string, number> = { all: byDetector.length, critical: 0, high: 0, medium: 0, low: 0 }
    for (const r of byDetector) c[r.severity] = (c[r.severity] ?? 0) + 1
    return c
  }, [byDetector])

  // 合并筛选:在 byDetector 基础上再按 sev。
  const filtered = sev === 'all' ? byDetector : byDetector.filter((r) => r.severity === sev)

  // 列顺序与风险管理列表对齐:规则名称(说明)居首 → 级别 → 检测器 → 规则号 → 规则语法。
  const columns: ColumnsType<FlatRule> = [
    {
      title: '规则名称', ellipsis: true, render: (_: unknown, r: FlatRule) => (
        <Tooltip title={r.description}>
          <span>{r.description}</span>
        </Tooltip>
      ),
    },
    { title: '级别', width: 80, render: (_: unknown, r: FlatRule) => <SevBadge tone={`sev-${r.severity}` as BadgeTone}>{sevLabel[r.severity]}</SevBadge> },
    { title: '检测器', width: 120, dataIndex: 'detector' },
    { title: '规则号', width: 220, dataIndex: 'id', render: (id: string) => <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{id}</Typography.Text> },
    {
      // 规则语法:baseline 按 op 拼、injection 为正则原文;无则 '--'。与风险管理抽屉的「规则语法」一致。
      title: '规则语法', width: 280, render: (_: unknown, r: FlatRule) => (
        <Typography.Text style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>{r.syntax || '--'}</Typography.Text>
      ),
    },
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
    <Card>
      <Segmented style={{ marginBottom: 12 }} value={sev} onChange={(v) => setSev(v as string)} options={options} />
      <Table<FlatRule>
        rowKey={(r) => `${r.detector_id}:${r.id}`}
        columns={columns}
        dataSource={filtered}
        pagination={{ pageSize: 20, size: 'default' }}
        size="middle"
      />
    </Card>
  )
}
