import { useState, useMemo, type HTMLAttributes } from 'react'
import { Table, Segmented, Empty, Typography, Card, Tooltip } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { DetectorMeta, Severity } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { RuleDrawer } from './RuleDrawer'

const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }
type FlatRule = { id: string; severity: Severity; description: string; syntax?: string; detector: string; detector_id: string }

// 级别筛选配色与风险管理列表(FindingTable)共用 .sev-seg 体系:index.css 按 .sev-tab-* 给选中项填级别实色。
const order: Severity[] = ['critical', 'high', 'medium', 'low']
const sevDot: Record<Severity, string> = {
  critical: 'var(--sev-critical)', high: 'var(--sev-high)', medium: 'var(--sev-medium)', low: 'var(--sev-low)',
}

// 级别筛选标签:色点 + 文案 + 计数。「全部」用 accent 点,各级别用对应级别色点。
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

// 规则总览:汇总所有检测器的规则,按 sev + 检测器筛选。规则号 mono,sev 标签,检测器名,说明。
// detectorFilter(可选):外部胶囊行点击检测器后传入,只显示该检测器规则。
export function RulesTable({ detectors, detectorFilter }: { detectors: DetectorMeta[]; detectorFilter?: string }) {
  const [sev, setSev] = useState<Severity | 'all'>('all')
  const [selected, setSelected] = useState<FlatRule | null>(null)

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

  // 列顺序:规则号 → 规则名称 → 级别 → 检测器 → 规则语法。
  // 规则号/规则语法加宽并 mono;规则名称作弹性列(ellipsis 截断),收窄其占比。
  // 行可点击 → 打开规则详情抽屉(展示完整语法 + 所属检测器上下文)。
  const columns: ColumnsType<FlatRule> = [
    { title: '规则号', width: 260, dataIndex: 'id', render: (id: string) => <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{id}</Typography.Text> },
    {
      title: '规则名称', ellipsis: true, render: (_: unknown, r: FlatRule) => (
        <Tooltip title={r.description}>
          <span>{r.description}</span>
        </Tooltip>
      ),
    },
    { title: '级别', width: 80, render: (_: unknown, r: FlatRule) => <SevBadge tone={`sev-${r.severity}` as BadgeTone}>{sevLabel[r.severity]}</SevBadge> },
    { title: '检测器', width: 120, dataIndex: 'detector' },
    {
      // 规则语法:baseline 按 op 拼、injection 为正则原文;无则 '--'。列表截断,详情抽屉展示完整。
      title: '规则语法', width: 360, ellipsis: true, render: (_: unknown, r: FlatRule) => (
        <Tooltip title={r.syntax || '--'}>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>{r.syntax || '--'}</span>
        </Tooltip>
      ),
    },
  ]

  if (allRules.length === 0) return <Empty description="暂无规则" />

  return (
    <Card>
      {/* 级别筛选:与风险管理列表同一套 sev-seg 配色(色点 + 选中实色填充),复用 index.css 的 .sev-seg。 */}
      <Segmented
        className="sev-seg"
        style={{ marginBottom: 12 }}
        value={sev}
        onChange={(v) => setSev(v as Severity | 'all')}
        options={[
          { value: 'all', label: <SevSegLabel text="全部" count={counts.all} />, className: 'sev-tab-all' },
          ...order.map((s) => ({
            value: s,
            label: <SevSegLabel text={sevLabel[s]} count={counts[s] ?? 0} sev={s} />,
            className: `sev-tab-${s}`,
          })),
        ]}
      />
      <Table<FlatRule>
        rowKey={(r) => `${r.detector_id}:${r.id}`}
        columns={columns}
        dataSource={filtered}
        pagination={{ pageSize: 20, size: 'default' }}
        size="middle"
        onRow={(r) => ({
          onClick: () => setSelected(r),
          style: { cursor: 'pointer' },
        }) as HTMLAttributes<HTMLElement>}
      />
      <RuleDrawer rule={selected} detectors={detectors} onClose={() => setSelected(null)} />
    </Card>
  )
}
