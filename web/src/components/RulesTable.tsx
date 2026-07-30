import { useState, useMemo, useEffect, type HTMLAttributes } from 'react'
import { Table, Segmented, Empty, Typography, Card, Tooltip, Tag, Space, Switch, Button, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { EditOutlined, ForkOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import type { Severity, RuleDTO, RuleDomain } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { RuleDrawer } from './RuleDrawer'
import { SEVERITY_ORDER, SEVERITY_LABEL_KEY, SEVERITY_DOT } from '../lib/severity'
import { ruleName } from '../lib/i18n-names'
import { useStore } from '../store'

// 级别筛选配色与风险管理列表(FindingTable)共用 .sev-seg 体系:index.css 按 .sev-tab-* 给选中项填级别实色。

// 级别筛选标签:色点 + 文案 + 计数。「全部」用 accent 点,各级别用对应级别色点。
function SevSegLabel({ text, count, sev }: { text: string; count: number; sev?: Severity }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
      <span
        className="sev-seg-dot"
        style={{ width: 8, height: 8, borderRadius: '50%', background: sev ? SEVERITY_DOT[sev] : 'var(--color-accent)' }}
      />
      <span>{text}</span>
      <span className="sev-seg-count" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{count}</span>
    </span>
  )
}

// 来源筛选选项(builtin/custom/all):RuleDTO.source 只有 builtin|custom 两值。
const SOURCE_OPTIONS = ['builtin', 'custom'] as const

interface RulesTableProps {
  domain: RuleDomain
  onEdit?: (r: RuleDTO) => void
  onFork?: (r: RuleDTO) => void
}

// 规则总览:按域(detect/intercept)从 store 读 RuleDTO[],按 sev + 来源筛选。
// 操作列:启停 Switch(custom/builtin 都可)+ custom 规则的 Edit/Delete + builtin 规则的 Fork。
// onEdit/onFork 回调由 Settings 页传入(Task 17 接 Segmented 域切换 + Task 16 接 RuleDrawer 编辑模式)。
// Task 15:Settings 暂硬编码 domain="detect",onEdit/onFork 为 no-op(占位,Task 16/17 接线)。
export function RulesTable({ domain, onEdit, onFork }: RulesTableProps) {
  const { t } = useTranslation()
  const [sev, setSev] = useState<Severity | 'all'>('all')
  const [src, setSrc] = useState<string>('all')
  // selectedRule 为行点击只读详情抽屉(RuleDrawer mode='view')用,直接存 RuleDTO。
  // edit/create 抽屉由 Settings(Task 17)拥有;此处仅 view-only。
  const [selectedRule, setSelectedRule] = useState<RuleDTO | null>(null)

  // store 数据源:按 domain 取对应域的 RuleDTO[](Task 14 已建好 state + actions)。
  const {
    detectRules, interceptRules, loadingRuleId,
    toggleRule, deleteRule, fetchDetectRules, fetchInterceptRules,
  } = useStore()

  // domain 切换时拉对应域规则(detect/intercept 对称)。
  useEffect(() => {
    if (domain === 'detect') fetchDetectRules()
    else fetchInterceptRules()
  }, [domain, fetchDetectRules, fetchInterceptRules])

  const rawRules: RuleDTO[] = useMemo(() => {
    const list = domain === 'detect' ? detectRules : interceptRules
    return list ?? []
  }, [domain, detectRules, interceptRules])

  // 来源分布:按 rule.source(builtin/custom)计数。
  const sourceCounts = useMemo(() => {
    const c: Record<string, number> = { all: rawRules.length, builtin: 0, custom: 0 }
    for (const r of rawRules) c[r.source] = (c[r.source] ?? 0) + 1
    return c
  }, [rawRules])

  // 来源筛选:在 rawRules 基础上按 source 过滤(来源 → 级别两级过滤)。
  const bySource = useMemo(
    () => src === 'all' ? rawRules : rawRules.filter((r) => r.source === src),
    [rawRules, src]
  )

  // 级别分布:在来源筛选基础上算,使 Segmented 计数随来源筛选联动。
  const counts = useMemo(() => {
    const c: Record<string, number> = { all: bySource.length, critical: 0, high: 0, medium: 0, low: 0, info: 0 }
    for (const r of bySource) c[r.severity] = (c[r.severity] ?? 0) + 1
    return c
  }, [bySource])

  // 合并筛选:在 bySource 基础上再按 sev(来源 → 级别两级过滤)。
  const filtered = sev === 'all' ? bySource : bySource.filter((r) => r.severity === sev)

  // 列顺序:规则号 → 规则名称 → 级别 → 来源 → 操作。
  // RuleDTO 无 detector/syntax/valid 字段(旧 FlatRule 才有),故去掉检测器列与规则语法列。
  // 行可点击 → 打开规则详情抽屉(只读,Task 16 改编辑模式)。
  const columns: ColumnsType<RuleDTO> = [
    { title: t('ruleTable.colRuleId'), width: 260, dataIndex: 'id', render: (id: string) => <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{id}</Typography.Text> },
    {
      title: t('ruleTable.colRuleName'), ellipsis: true, render: (_: unknown, r: RuleDTO) => {
        // 规则名称取双语名(ruleName:先 i18n rules.<id>,回退 r.description 后端原文)。
        const name = ruleName({ id: r.id, description: r.description ?? '' })
        return (
          <Tooltip title={name}>
            <span>{name}</span>
          </Tooltip>
        )
      },
    },
    { title: t('ruleTable.colSeverity'), width: 80, render: (_: unknown, r: RuleDTO) => <SevBadge tone={`sev-${r.severity}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[r.severity as Severity])}</SevBadge> },
    {
      // 来源:RuleDTO.source 只有 builtin(灰 Tag)/ custom(蓝 Tag)。
      // Task 17:改用 rulesManage.sourceBuiltin/sourceCustom(内置/自定义),比 ruleTable.sourceBaseline
      //   (基线,旧 FlatRule 语义)更贴合 RuleDTO 的 builtin/custom 二分。
      title: t('ruleTable.colSource'), width: 90, render: (_: unknown, r: RuleDTO) => (
        <Tag
          style={{ marginInlineEnd: 0, fontSize: 11, background: r.source === 'custom' ? 'var(--cat-1)' : 'transparent', color: r.source === 'custom' ? 'var(--badge-text)' : 'var(--color-muted)', border: r.source === 'custom' ? 'none' : '1px solid var(--color-rule)' }}
        >
          {r.source === 'custom' ? t('rulesManage.sourceCustom') : t('rulesManage.sourceBuiltin')}
        </Tag>
      ),
    },
    {
      // 操作列:启停 Switch + custom→Edit/Delete + builtin→Fork。
      // rulesManage.* i18n key 由 Task 17 添加,缺失时 t() 返回 key 字符串(可接受,build 不受影响)。
      title: t('rulesManage.actions'), key: 'actions', width: 220, render: (_: unknown, r: RuleDTO) => (
        <Space size="small" onClick={(e) => e.stopPropagation()}>
          <Switch
            size="small"
            checked={r.enabled}
            loading={loadingRuleId === r.id}
            onChange={(checked) => { void toggleRule(domain, r.id, checked) }}
          />
          {r.source === 'custom' ? (
            <>
              <Button type="text" size="small" icon={<EditOutlined />} aria-label={t('rulesManage.edit')} onClick={() => onEdit?.(r)} />
              <Popconfirm
                title={t('rulesManage.confirmDelete')}
                onConfirm={() => { void deleteRule(domain, r.id) }}
              >
                <Button type="text" danger size="small" icon={<DeleteOutlined />} aria-label={t('rulesManage.delete')} />
              </Popconfirm>
            </>
          ) : (
            <Button type="text" size="small" icon={<ForkOutlined />} aria-label={t('rulesManage.fork')} onClick={() => onFork?.(r)} />
          )}
        </Space>
      ),
    },
  ]

  if (rawRules.length === 0) return <Empty description={t('ruleTable.empty')} />

  return (
    <Card>
      {/* 筛选工具栏行(design.md #2:统一模式——框在结果 Card 内顶部 + 底部 hairline 分隔)。
          来源 + 级别两组筛选同一行(flex-wrap),复用 sev-seg 配色,组合:来源 → 级别。 */}
      <div className="filter-toolbar">
        <Segmented
          className="sev-seg"
          value={src}
          onChange={(v) => setSrc(v as string)}
          options={[
            { value: 'all', label: <SevSegLabel text={t('ruleTable.all')} count={sourceCounts.all} />, className: 'sev-tab-all' },
            ...SOURCE_OPTIONS.map((s) => ({
              value: s,
              label: <SevSegLabel text={s === 'builtin' ? t('rulesManage.sourceBuiltin') : t('rulesManage.sourceCustom')} count={sourceCounts[s] ?? 0} />,
              className: 'sev-tab-all',
            })),
          ]}
        />
        <Segmented
          className="sev-seg"
          value={sev}
          onChange={(v) => setSev(v as Severity | 'all')}
          options={[
            { value: 'all', label: <SevSegLabel text={t('ruleTable.all')} count={counts.all} />, className: 'sev-tab-all' },
            ...SEVERITY_ORDER.map((s) => ({
              value: s,
              label: <SevSegLabel text={t(SEVERITY_LABEL_KEY[s])} count={counts[s] ?? 0} sev={s} />,
              className: `sev-tab-${s}`,
            })),
          ]}
        />
      </div>
      <Table<RuleDTO>
        rowKey={(r) => r.id}
        columns={columns}
        dataSource={filtered}
        // 分页:defaultPageSize(非受控)而非 pageSize(受控)——同 AssetTable,避免页大小选择器
        // 改动被受控 pageSize 重置回 20(详见 AssetTable 注释)。规则较多时(63 条)分页避免长列表。
        pagination={{ defaultPageSize: 20, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'], size: 'default' }}
        size="middle"
        onRow={(r) => ({
          onClick: () => setSelectedRule(r),
          style: { cursor: 'pointer' },
        }) as HTMLAttributes<HTMLElement>}
      />
      {/* 行点击只读详情抽屉(mode='view')。edit/create 抽屉由 Settings(Task 17)拥有:
          RulesTable 的 onEdit/onFork 回调转发给 Settings,Settings 切到编辑抽屉。
          两抽屉不同时打开(selectedRule 与 Settings 的 editingRule 互斥:用户点编辑按钮前 view 抽屉可先关,
          或 Settings 在打开 edit 时清空 view 选择——Task 17 接线时处理)。此处仅 view-only,无冲突。 */}
      <RuleDrawer
        rule={selectedRule}
        mode="view"
        domain={domain}
        onClose={() => setSelectedRule(null)}
      />
    </Card>
  )
}
