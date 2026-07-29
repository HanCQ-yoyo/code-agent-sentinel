import { useState, useMemo, useEffect, type HTMLAttributes } from 'react'
import { Table, Segmented, Empty, Typography, Card, Tooltip, Tag, Space, Switch, Button, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import type { DetectorMeta, Severity, RuleDTO, RuleDomain } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { RuleDrawer } from './RuleDrawer'
import { SEVERITY_ORDER, SEVERITY_LABEL_KEY, SEVERITY_DOT } from '../lib/severity'
import { detectorName, ruleName } from '../lib/i18n-names'
import { useStore } from '../store'

// 旧 FlatRule 类型(RuleDrawer 仍依赖:import { sourceLabel, type FlatRule } from './RulesTable')。
// Task 16 将重写 RuleDrawer 改用 RuleDTO,届时 FlatRule 与 sourceLabel 导出将一并移除。
// 在此之前保留导出以维持 tsc 编译通过(Option A:RulesTable 行类型切到 RuleDTO,但保留 legacy 导出)。
export type FlatRule = {
  id: string; severity: Severity; description: string; syntax?: string
  asset_type?: string; remediation?: string; paths?: { include?: string[]; exclude?: string[] }
  post_exclude?: string[]; deobfuscation?: string[]; dotall?: boolean
  metadata?: Record<string, unknown>; source_file?: string; project_path?: string
  source?: string; valid?: boolean
  detector: string; detector_id: string
}

// 级别筛选配色与风险管理列表(FindingTable)共用 .sev-seg 体系:index.css 按 .sev-tab-* 给选中项填级别实色。

// RuleDrawer 仍 import sourceLabel 用于展示来源文案;Task 16 重写后移除。
export const sourceLabel: Record<string, string> = {
  baseline: 'ruleTable.sourceBaseline',
  injection: 'ruleTable.sourceInjection',
  skill: 'ruleTable.sourceSkill',
  custom: 'ruleTable.sourceCustom',
  other: 'ruleTable.sourceOther',
}

// RuleDTO → FlatRule 适配器:Task 16 将重写 RuleDrawer 改用 RuleDTO,在此之前用此适配器
// 把 RuleDTO 映射为 RuleDrawer 只读展示所需的 FlatRule 形状(detector/syntax/valid 等 RuleDTO 无字段填占位)。
// 这是保持 tsc 编译通过 + 不触碰 RuleDrawer(Task 16 职责)的最小方案。
function ruleDTOToFlatRule(r: RuleDTO, detectors: DetectorMeta[]): FlatRule {
  // RuleDTO 无 detector/detector_id:按 rule_id 前缀匹配检测器(baseline.* → rules 等),
  // 找不到则回退 'rules' 检测器名(只读展示,不参与逻辑)。
  const prefix = r.id.indexOf('.') > 0 ? r.id.slice(0, r.id.indexOf('.')) : ''
  const det = detectors.find((d) => d.id === prefix || d.id === 'rules') ?? detectors[0]
  return {
    id: r.id,
    severity: r.severity as Severity,
    description: r.description ?? '',
    syntax: undefined, // RuleDTO 无 syntax 字段;RuleDrawer 展示 '--'
    asset_type: r.asset_type,
    remediation: r.remediation,
    paths: r.paths ?? undefined,
    post_exclude: r.post_exclude,
    deobfuscation: r.deobfuscation,
    dotall: r.dotall,
    metadata: r.metadata,
    source: r.source, // 'builtin'|'custom'
    valid: true, // RuleDTO 无 valid 字段;后端已校验入库,默认 true
    detector: det ? detectorName(det) : '--',
    detector_id: det?.id ?? '',
    source_file: undefined,
    project_path: undefined,
  }
}

// 级别筛选标签:色点 + 文案 + 计数。「全部」用 accent 点,各级别用对应级别色点。
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
  // selectedFlat 为 RuleDrawer 只读展示用(适配自 RuleDTO);Task 16 重写 RuleDrawer 后改用 RuleDTO。
  const [selectedFlat, setSelectedFlat] = useState<FlatRule | null>(null)

  // store 数据源:按 domain 取对应域的 RuleDTO[](Task 14 已建好 state + actions)。
  const {
    detectRules, interceptRules, loadingRuleId,
    toggleRule, deleteRule, fetchDetectRules, fetchInterceptRules,
    detectors,
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
      title: t('ruleTable.colSource'), width: 90, render: (_: unknown, r: RuleDTO) => (
        <Tag
          color={r.source === 'custom' ? 'blue' : 'default'}
          style={{ marginInlineEnd: 0, fontSize: 11 }}
        >
          {r.source === 'custom' ? t('ruleTable.sourceCustom') : t('ruleTable.sourceBaseline')}
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
              <Button size="small" onClick={() => onEdit?.(r)}>{t('rulesManage.edit')}</Button>
              <Popconfirm
                title={t('rulesManage.confirmDelete')}
                onConfirm={() => { void deleteRule(domain, r.id) }}
              >
                <Button size="small" danger>{t('rulesManage.delete')}</Button>
              </Popconfirm>
            </>
          ) : (
            <Button size="small" onClick={() => onFork?.(r)}>{t('rulesManage.fork')}</Button>
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
              label: <SevSegLabel text={s === 'builtin' ? t('ruleTable.sourceBaseline') : t('ruleTable.sourceCustom')} count={sourceCounts[s] ?? 0} />,
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
          onClick: () => setSelectedFlat(ruleDTOToFlatRule(r, detectors)),
          style: { cursor: 'pointer' },
        }) as HTMLAttributes<HTMLElement>}
      />
      {/* RuleDrawer 仍用 FlatRule(Task 16 改 RuleDTO):用 ruleDTOToFlatRule 适配,只读展示。 */}
      <RuleDrawer rule={selectedFlat} detectors={detectors} onClose={() => setSelectedFlat(null)} />
    </Card>
  )
}
