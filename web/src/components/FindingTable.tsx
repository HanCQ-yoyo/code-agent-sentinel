import { useState, type HTMLAttributes } from 'react'
import { Card, Table, Segmented, Typography, Empty, Tooltip, Tag } from 'antd'
import { useTranslation } from 'react-i18next'
import type { ColumnsType } from 'antd/es/table'
import type { Finding, Severity, DetectorMeta } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { SEVERITY_ORDER, SEVERITY_LABEL_KEY, SEVERITY_DOT } from '../lib/severity'
import { formatDateTime } from '../lib/format'
import { detectorNameById, ruleNameById } from '../lib/i18n-names'

// 筛选标签内的色点颜色(复用 sev token);「全部」用 accent。
// 级别筛选标签:左侧色点 + 文本 + 计数。色点颜色对应级别,选中时整块填该级别色(见 .sev-seg CSS),
// 与未选中的透明底+色点形成明显差别。
function SevSegLabel({ text, count, sev }: { text: string; count: number; sev?: Severity }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
      <span
        className="sev-seg-dot"
        style={{ width: 8, height: 8, borderRadius: '50%', background: sev ? SEVERITY_DOT[sev] : 'var(--accent)' }}
      />
      <span>{text}</span>
      <span className="sev-seg-count" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{count}</span>
    </span>
  )
}

// 抑制状态筛选:全部 | 活跃(未抑制)| 已抑制。与 sev 筛选 AND 组合。
type SupprFilter = 'all' | 'active' | 'suppressed'

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
  const { t } = useTranslation()
  const [filter, setFilter] = useState<Severity | 'all'>('all')
  const [supprFilter, setSupprFilter] = useState<SupprFilter>('all')
  const counts: Record<string, number> = { all: findings.length }
  for (const s of SEVERITY_ORDER) counts[s] = findings.filter((f) => f.severity === s).length
  const supprCounts = {
    all: findings.length,
    active: findings.filter((f) => !f.suppressed).length,
    suppressed: findings.filter((f) => f.suppressed).length,
  }

  // 合并筛选:sev × 抑制状态(AND)。
  let shown = filter === 'all' ? findings : findings.filter((f) => f.severity === filter)
  if (supprFilter === 'active') shown = shown.filter((f) => !f.suppressed)
  else if (supprFilter === 'suppressed') shown = shown.filter((f) => f.suppressed)
  const sorted = [...shown].sort((a, b) => SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity))

  // detector_id → 双语名(先查 i18n detectors.<id>,回退 detector.name,再回退 id)。
  const detName = (id: string): string => detectorNameById(detectors ?? [], id)

  const columns: ColumnsType<Finding> = [
    {
      // 风险名称:不设固定宽度,作为弹性主列占据剩余空间并省略;资产列加宽(280)后这里相应收窄,
      // 把空间预留给资产列(用户反馈:风险名称过宽、资产偏挤)。
      // 已抑制 finding:名称后附「已抑制」标签(Tooltip 展示抑制来源 + reason),行整体降透明度。
      // 名称取规则双语名(ruleNameById:先 i18n rules.<rule_id>,回退 f.message 后端原文)。
      title: t('findingTable.colName'), ellipsis: true, render: (_: unknown, f: Finding) => {
        const name = ruleNameById(f.rule_id, f.message)
        return (
          <Tooltip title={name}>
            <span>
              {name}
              {f.suppressed ? (
                <Tooltip title={t('findingTable.supprTooltip', { source: f.suppression ?? '--', reason: f.reason ? t('findingTable.reasonPart', { reason: f.reason }) : '' })}>
                  <Tag style={{ marginInlineEnd: 0, marginLeft: 6, fontSize: 10, lineHeight: '16px', padding: '0 5px', borderColor: 'var(--bg-border)', color: 'var(--text-muted)', background: 'var(--surface-2)' }}>
                    {t('findingTable.suppressedTag')}
                  </Tag>
                </Tooltip>
              ) : null}
            </span>
          </Tooltip>
        )
      },
    },
    {
      // 资产:文件名 + 类型两词,加宽到 280(预留给资产列);长名省略,Tooltip 兜底。
      title: t('findingTable.colAsset'), width: 280, ellipsis: true, render: (_: unknown, f: Finding) => (
        <Tooltip title={`${f.asset_name} ${f.asset_type}`}>
          <span>{f.asset_name} <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{f.asset_type}</Typography.Text></span>
        </Tooltip>
      ),
    },
    { title: t('findingTable.colSeverity'), width: 80, render: (_: unknown, f: Finding) => <SevBadge tone={`sev-${f.severity}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[f.severity])}</SevBadge> },
    {
      title: t('findingTable.colDetector'), width: 120, render: (_: unknown, f: Finding) => (
        <Typography.Text style={{ fontSize: 12 }}>{detName(f.detector_id)}</Typography.Text>
      ),
    },
    {
      // 规则列加宽 1 倍(160→320),容纳完整 rule_id mono 文本,不再截断;字体放大到 13 便于阅读。
      title: t('findingTable.colRule'), width: 320, render: (_: unknown, f: Finding) => (
        <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 13 }}>{f.rule_id}</Typography.Text>
      ),
    },
    {
      title: t('findingTable.colScanTime'), width: 150, render: () => (
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{startedAt ? formatDateTime(startedAt) : '--'}</span>
      ),
    },
  ]

  return (
    <Card>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, marginBottom: 12, alignItems: 'center' }}>
        <Segmented
          className="sev-seg"
          value={filter}
          onChange={(v) => setFilter(v as Severity | 'all')}
          options={[
            { value: 'all', label: <SevSegLabel text={t('findingTable.all')} count={counts.all} />, className: 'sev-tab-all' },
            ...SEVERITY_ORDER.map((s) => ({
              value: s,
              label: <SevSegLabel text={t(SEVERITY_LABEL_KEY[s])} count={counts[s]} sev={s} />,
              className: `sev-tab-${s}`,
            })),
          ]}
        />
        {/* 抑制状态筛选:与 sev 筛选 AND 组合。已抑制 finding 默认仍在「全部」中显示(降透明度)。 */}
        <Segmented
          size="small"
          value={supprFilter}
          onChange={(v) => setSupprFilter(v as SupprFilter)}
          options={[
            { value: 'all', label: `${t('findingTable.all')} ${supprCounts.all}` },
            { value: 'active', label: `${t('findingTable.active')} ${supprCounts.active}` },
            { value: 'suppressed', label: `${t('findingTable.suppressed')} ${supprCounts.suppressed}` },
          ]}
        />
      </div>
      <Table<Finding>
        rowKey={(_f, i) => String(i)}
        columns={columns}
        dataSource={sorted}
        pagination={false}
        size="middle"
        // 行点击打开抽屉;保留 finding-row testid(e2e [data-testid="finding-row"] 硬约束)。
        // onClick 经 onRow 注入;data-testid 同理(参考 AssetTable onRow 模式)。
        // 已抑制 finding 行降透明度(opacity 0.55),视觉上与活跃 finding 区分。
        onRow={(f) => ({
          'data-testid': 'finding-row',
          onClick: () => onSelect?.(f),
          style: { ...(onSelect ? { cursor: 'pointer' } : {}), ...(f.suppressed ? { opacity: 0.55 } : {}) },
        }) as HTMLAttributes<HTMLElement>}
        locale={{ emptyText: findings.length === 0 ? <Empty description={t('findingTable.empty')} /> : t('findingTable.noMatch') }}
      />
    </Card>
  )
}
