import { useState, type HTMLAttributes } from 'react'
import { Card, Table, Segmented, Typography, Empty, Tooltip, Tag, Select, Space, Button } from 'antd'
import { useTranslation } from 'react-i18next'
import type { ColumnsType } from 'antd/es/table'
import type { Finding, Severity, DetectorMeta } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { SEVERITY_ORDER, SEVERITY_LABEL_KEY, SEVERITY_DOT, STATUS_COLOR, PRIORITY_COLOR, severityToPrio } from '../lib/severity'
import { formatDateTime } from '../lib/format'
import { ruleNameById } from '../lib/i18n-names'
import { agentMetaById } from '../lib/agents'
import { AgentIcon } from './AgentIcon'
import { ASSET_TYPE_META } from '../lib/assetTypes'

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

// Task 14:平铺 / 按规则聚合 双视图。
type View = 'flat' | 'byRule'

interface FindingTableProps {
  findings: Finding[]
  // 整次扫描起始时间(同一次扫描所有行共享)。可选:无 scan 时间时不显示该列内容。
  startedAt?: string
  // 检测器元数据,供按 detector_id 查中文名;无则显示 detector_id。
  // 注:Task 8 删除了 colDetector 列,此 prop 在本组件内不再使用,但接口保留(调用方仍传,
  // Findings/History 未必同步改;保持兼容,避免破坏调用方)。Task 9 接手 Findings 页时统一清理。
  detectors?: DetectorMeta[]
  // 行点击 → 打开详情抽屉。
  onSelect?: (f: Finding) => void
  // 操作列「立即处置」按钮 → 父组件打开处置弹框(Task 9 接手)。可选:未传则按钮不触发(留空回调)。
  onDispose?: (f: Finding) => void
}

export function FindingTable({ findings, startedAt, detectors, onSelect, onDispose }: FindingTableProps) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<Severity | 'all'>('all')
  const [supprFilter, setSupprFilter] = useState<SupprFilter>('all')
  // Task 14:治理字段筛选(默认 'all' = 不过滤);view 控制平铺/聚合视图。
  const [view, setView] = useState<View>('flat')
  const [catFilter, setCatFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [prioFilter, setPrioFilter] = useState<string>('all')
  const [typeFilter, setTypeFilter] = useState<string>('all')
  const counts: Record<string, number> = { all: findings.length }
  for (const s of SEVERITY_ORDER) counts[s] = findings.filter((f) => f.severity === s).length
  const supprCounts = {
    all: findings.length,
    active: findings.filter((f) => !f.suppressed).length,
    suppressed: findings.filter((f) => f.suppressed).length,
  }

  // 合并筛选:sev × 抑制状态 × 治理字段(全部 AND 组合)。
  let shown = filter === 'all' ? findings : findings.filter((f) => f.severity === filter)
  if (supprFilter === 'active') shown = shown.filter((f) => !f.suppressed)
  else if (supprFilter === 'suppressed') shown = shown.filter((f) => f.suppressed)
  // Task 14:治理字段筛选(在 sev/suppr 之后追加;status 缺省视为 'open',priority 缺省回退 severityToPrio)。
  if (catFilter !== 'all') shown = shown.filter((f) => f.category === catFilter)
  if (statusFilter !== 'all') shown = shown.filter((f) => (f.status ?? 'open') === statusFilter)
  if (prioFilter !== 'all') shown = shown.filter((f) => (f.priority ?? severityToPrio(f.severity)) === prioFilter)
  if (typeFilter !== 'all') shown = shown.filter((f) => f.asset_type === typeFilter)
  const sorted = [...shown].sort((a, b) => SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity))

  // 选项从当前 findings 派生(只列出实际存在的类别 / 类型)。
  const cats = [...new Set(findings.map((f) => f.category).filter(Boolean))] as string[]
  const types = [...new Set(findings.map((f) => f.asset_type).filter(Boolean))] as string[]

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
                  <Tag style={{ marginInlineEnd: 0, marginLeft: 6, fontSize: 10, lineHeight: '16px', padding: '0 5px', borderColor: 'var(--color-rule)', color: 'var(--color-muted)', background: 'var(--color-surface)' }}>
                    {t('findingTable.suppressedTag')}
                  </Tag>
                </Tooltip>
              ) : null}
              {/* Task 14:combo 规则的 contributing chips —— 在规则名后附 +N(命中子规则数)。 */}
              {f.contributing_rule_ids?.length ? (
                <Tag style={{ marginInlineEnd: 0, marginLeft: 6, fontSize: 10, lineHeight: '16px', padding: '0 5px', background: 'var(--cat-5)', color: 'var(--badge-text)', border: 'none' }}>
                  +{f.contributing_rule_ids.length}
                </Tag>
              ) : null}
            </span>
          </Tooltip>
        )
      },
    },
    {
      title: t('findingTable.colAsset'), width: 280, ellipsis: true, render: (_: unknown, f: Finding) => {
        const meta = ASSET_TYPE_META.find((m) => m.type === f.asset_type)
        return (
          <Tooltip title={f.source_path ?? f.asset_name}>
            <span>
              {f.asset_name}{' '}
              <Tag style={{ marginInlineEnd: 0, fontSize: 10, lineHeight: '16px', padding: '0 5px' }}>
                {meta ? t(meta.labelKey) : f.asset_type}
              </Tag>
            </span>
          </Tooltip>
        )
      },
    },
    { title: t('findingTable.colSeverity'), width: 80, render: (_: unknown, f: Finding) => <SevBadge tone={`sev-${f.severity}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[f.severity])}</SevBadge> },
    {
      // Task 12:Agent 列(Severity 之后)。聚合视图下每条 finding 带 agent_id;
      // 单 agent / 旧记录缺省 → '-'。用 agentMeta 图标 + 名(无 Tag 背景,与 Dashboard/History 统一)。
      title: t('findingTable.colAgent'), dataIndex: 'agent_id', width: 120,
      render: (id?: string) => {
        if (!id) return '-'
        const m = agentMetaById(id)
        return <span style={{ whiteSpace: 'nowrap' }}><AgentIcon id={id} /> {m.label}</span>
      },
    },
    {
      title: t('findingTable.colCategory', { defaultValue: '风险类型' }), width: 120, render: (_: unknown, f: Finding) => (
        <Typography.Text style={{ fontSize: 12 }}>{f.category ? t(`category.${f.category}`, { defaultValue: f.category }) : '-'}</Typography.Text>
      ),
    },
    {
      title: t('findingTable.colStatus', { defaultValue: '处置状态' }), width: 110, render: (_: unknown, f: Finding) => (
        f.status && f.status !== 'open' ? (
          <Tag style={{ fontSize: 10, lineHeight: '16px', padding: '0 5px', background: STATUS_COLOR[f.status] ?? 'var(--color-rule-2)', color: 'var(--badge-text)', border: 'none' }}>
            {t(`findingTable.status.${f.status}`, { defaultValue: f.status })}
          </Tag>
        ) : <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('findingTable.status.open')}</Typography.Text>
      ),
    },
    {
      title: t('findingTable.colScanTime'), width: 150, render: (_: unknown, f: Finding) => (
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{(f.started_at ?? startedAt) ? formatDateTime(f.started_at ?? startedAt!) : '--'}</span>
      ),
    },
    {
      title: t('findingTable.colAction', { defaultValue: '操作' }), width: 110, render: (_: unknown, f: Finding) => (
        <Button size="small" onClick={(e) => { e.stopPropagation(); onDispose?.(f) }}>
          {f.status && f.status !== 'open' ? t('findingDrawer.disposed', { defaultValue: '已处置' }) : t('findingDrawer.disposeNow', { defaultValue: '立即处置' })}
        </Button>
      ),
    },
  ]

  // filter-toolbar 抽成共享 JSX:平铺视图和按规则聚合视图共用同一套筛选(避免重复)。
  // 包含:视图开关 Segmented(最前,主切换)、sev Segmented、抑制状态 Segmented、Category/Status/Priority/Type 4 个 Select。
  const filterToolbar = (
    <div className="filter-toolbar">
      {/* Task 14:平铺 / 按规则聚合 双视图开关(移至最前,主切换入口)。 */}
      <Segmented size="small" value={view} onChange={(v) => setView(v as View)}
        options={[{ value: 'flat', label: t('findingTable.viewFlat') }, { value: 'byRule', label: t('findingTable.viewByRule') }]} />
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
      <Segmented
        className="sev-seg"
        value={supprFilter}
        onChange={(v) => setSupprFilter(v as SupprFilter)}
        options={[
          { value: 'all', label: <SevSegLabel text={t('findingTable.all')} count={supprCounts.all} />, className: 'sev-tab-all' },
          { value: 'active', label: <SevSegLabel text={t('findingTable.active')} count={supprCounts.active} />, className: 'sev-tab-all' },
          { value: 'suppressed', label: <SevSegLabel text={t('findingTable.suppressed')} count={supprCounts.suppressed} />, className: 'sev-tab-all' },
        ]}
      />
      {/* Task 14:治理字段筛选下拉(Category / Status / Priority / Type),与现有 sev/suppr AND 组合。 */}
      <Space>
        <Select size="small" value={catFilter} onChange={setCatFilter} style={{ width: 130 }}
          options={[{ value: 'all', label: t('findingTable.catAll') }, ...cats.map((c) => ({ value: c, label: t(`category.${c}`, { defaultValue: c }) }))]} />
        <Select size="small" value={statusFilter} onChange={setStatusFilter} style={{ width: 110 }}
          options={['all', 'open', 'in_progress', 'resolved', 'false_positive', 'accepted'].map((s) => ({ value: s, label: s === 'all' ? t('findingTable.statusAll') : t(`findingTable.status.${s}`) }))} />
        <Select size="small" value={prioFilter} onChange={setPrioFilter} style={{ width: 120 }}
          options={['all', 'P0', 'P1', 'P2', 'P3'].map((p) => ({ value: p, label: p === 'all' ? t('findingTable.prioAll') : <Tag style={{ marginInlineEnd: 0, fontSize: 10, padding: '0 5px', background: PRIORITY_COLOR[p], color: 'var(--badge-text)', border: 'none' }}>{p}</Tag> }))} />
        <Select size="small" value={typeFilter} onChange={setTypeFilter} style={{ width: 130 }}
          options={[{ value: 'all', label: t('findingTable.typeAll') }, ...types.map((ty) => {
            const meta = ASSET_TYPE_META.find((m) => m.type === ty)
            return { value: ty, label: meta ? t(meta.labelKey) : ty }
          })]} />
      </Space>
    </div>
  )

  // 按规则聚合视图:按 rule_id 分组,按命中数降序,展开行复用平铺 columns。
  if (view === 'byRule') {
    const groups = new Map<string, Finding[]>()
    for (const f of sorted) {
      const key = f.rule_id
      if (!groups.has(key)) groups.set(key, [])
      groups.get(key)!.push(f)
    }
    const grouped = [...groups.entries()].sort((a, b) => b[1].length - a[1].length)
    return (
      <Card>
        {filterToolbar}
        <Table
          rowKey={([ruleId]) => ruleId}
          dataSource={grouped}
          pagination={false}
          size="middle"
          columns={[
            {
              title: t('findingTable.colName'),
              render: ([ruleId, fs]: [string, Finding[]]) => (
                <span>
                  {ruleNameById(ruleId, fs[0].message)}{' '}
                  <Tag>{fs.length} {t('findingTable.hits')}</Tag>
                  {fs[0].contributing_rule_ids?.length ? (
                    <Tag style={{ marginInlineEnd: 0, marginLeft: 6, fontSize: 10, lineHeight: '16px', padding: '0 5px', background: 'var(--cat-5)', color: 'var(--badge-text)', border: 'none' }}>+{fs[0].contributing_rule_ids.length}</Tag>
                  ) : null}
                </span>
              ),
            },
            {
              title: t('findingTable.colSeverity'),
              width: 80,
              render: ([, fs]: [string, Finding[]]) => (
                <SevBadge tone={`sev-${fs[0].severity}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[fs[0].severity])}</SevBadge>
              ),
            },
          ]}
          expandable={{
            expandedRowRender: ([, fs]: [string, Finding[]]) => (
              <Table<Finding> rowKey={(_, i) => String(i)} dataSource={fs} pagination={false} size="small" columns={columns}
                onRow={(f) => ({ onClick: () => onSelect?.(f), style: { cursor: onSelect ? 'pointer' : 'default' } })} />
            ),
          }}
        />
      </Card>
    )
  }

  return (
    <Card>
      {filterToolbar}
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
