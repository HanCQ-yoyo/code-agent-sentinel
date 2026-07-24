import { useState, useMemo, type HTMLAttributes } from 'react'
import { Table, Segmented, Empty, Typography, Card, Tooltip, Alert, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import type { DetectorMeta, Severity } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { RuleDrawer } from './RuleDrawer'
import { SEVERITY_ORDER, SEVERITY_LABEL_KEY, SEVERITY_DOT } from '../lib/severity'
import { detectorName, ruleName } from '../lib/i18n-names'

export type FlatRule = {
  id: string; severity: Severity; description: string; syntax?: string
  asset_type?: string; remediation?: string; paths?: { include?: string[]; exclude?: string[] }
  post_exclude?: string[]; deobfuscation?: string[]; dotall?: boolean
  metadata?: Record<string, unknown>; source_file?: string; project_path?: string
  source?: string; valid?: boolean
  detector: string; detector_id: string
}

// 级别筛选配色与风险管理列表(FindingTable)共用 .sev-seg 体系:index.css 按 .sev-tab-* 给选中项填级别实色。

// 按 rule_id 前缀推导来源分组(baseline./injection./skill./custom.)。
// 后端 RuleInfo 目前未带 source 字段;前端按前缀推导,后端补充后优先用 r.source。
function ruleSource(r: FlatRule): string {
  if (r.source) return r.source
  const i = r.id.indexOf('.')
  return i > 0 ? r.id.slice(0, i) : 'other'
}
export const sourceLabel: Record<string, string> = {
  baseline: 'ruleTable.sourceBaseline',
  injection: 'ruleTable.sourceInjection',
  skill: 'ruleTable.sourceSkill',
  custom: 'ruleTable.sourceCustom',
  other: 'ruleTable.sourceOther',
}
const sourceOrder = ['baseline', 'injection', 'skill', 'custom', 'other']

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

// 规则总览:汇总所有检测器的规则,按 sev + 检测器筛选。规则号 mono,sev 标签,检测器名,说明。
// detectorFilter(可选):外部胶囊行点击检测器后传入,只显示该检测器规则。
export function RulesTable({ detectors, detectorFilter }: { detectors: DetectorMeta[]; detectorFilter?: string }) {
  const { t } = useTranslation()
  const [sev, setSev] = useState<Severity | 'all'>('all')
  const [src, setSrc] = useState<string>('all')
  const [selected, setSelected] = useState<FlatRule | null>(null)

  const allRules = useMemo<FlatRule[]>(
    () => detectors.flatMap((d) => (d.rules ?? []).map((r) => {
      // detector 字段用双语名(先 i18n detectors.<id>,回退 d.name);description 原样保留,
      // 展示时再经 ruleName() 取双语名(先 i18n rules.<id>,回退 description)。
      const fr: FlatRule = { ...r, detector: detectorName(d), detector_id: d.id }
      // 后端 RuleInfo 未带 source:前端按 rule_id 前缀推导并写入 FlatRule.source,
      // 使列表列、来源筛选与 RuleDrawer 来源展示共用同一推导结果(单一推导点)。
      fr.source = fr.source ?? ruleSource(fr)
      return fr
    })),
    [detectors]
  )

  // 先按检测器筛选(detectorFilter 来自胶囊行),再据此算 sev 分布 + 二次 sev 筛选。
  // 这样 Segmented 计数随检测器筛选联动,「全部 N」反映当前检测器规则数,而非全局。
  const byDetector = useMemo(
    () => allRules.filter((r) => !detectorFilter || r.detector_id === detectorFilter),
    [allRules, detectorFilter]
  )

  // 来源分布:按 rule_id 前缀分组(baseline/injection/skill/custom/other),计数随检测器筛选联动。
  const sourceCounts = useMemo(() => {
    const c: Record<string, number> = { all: byDetector.length }
    for (const r of byDetector) {
      const s = ruleSource(r)
      c[s] = (c[s] ?? 0) + 1
    }
    return c
  }, [byDetector])

  // 来源筛选:在 byDetector 基础上再按来源前缀过滤,实现"按来源分组"。
  const bySource = useMemo(
    () => src === 'all' ? byDetector : byDetector.filter((r) => ruleSource(r) === src),
    [byDetector, src]
  )

  // 级别分布:在来源筛选基础上算,使 Segmented 计数随来源筛选联动(检测器 → 来源 → 级别)。
  const counts = useMemo(() => {
    const c: Record<string, number> = { all: bySource.length, critical: 0, high: 0, medium: 0, low: 0, info: 0 }
    for (const r of bySource) c[r.severity] = (c[r.severity] ?? 0) + 1
    return c
  }, [bySource])

  // 合并筛选:在 bySource 基础上再按 sev(检测器 → 来源 → 级别三级过滤)。
  const filtered = sev === 'all' ? bySource : bySource.filter((r) => r.severity === sev)

  // 无效规则(valid === false)置顶横幅:后端 Meta() 目前只返回已 Validate 的规则(全 valid),
  // 此横幅为防御性能力——后端补充 valid 字段后即生效。
  const invalidRules = useMemo(() => byDetector.filter((r) => r.valid === false), [byDetector])

  // 列顺序:规则号 → 规则名称 → 级别 → 来源 → 校验 → 检测器 → 规则语法。
  // 规则号/规则语法加宽并 mono;规则名称作弹性列(ellipsis 截断),收窄其占比。
  // 行可点击 → 打开规则详情抽屉(展示完整语法 + 所属检测器上下文)。
  const columns: ColumnsType<FlatRule> = [
    { title: t('ruleTable.colRuleId'), width: 260, dataIndex: 'id', render: (id: string) => <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{id}</Typography.Text> },
    {
      title: t('ruleTable.colRuleName'), ellipsis: true, render: (_: unknown, r: FlatRule) => {
        // 规则名称取双语名(ruleName:先 i18n rules.<id>,回退 r.description 后端原文)。
        const name = ruleName(r)
        return (
          <Tooltip title={name}>
            <span>{name}</span>
          </Tooltip>
        )
      },
    },
    { title: t('ruleTable.colSeverity'), width: 80, render: (_: unknown, r: FlatRule) => <SevBadge tone={`sev-${r.severity}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[r.severity])}</SevBadge> },
    {
      // 来源:按 rule_id 前缀推导(baseline./injection./skill./custom.),后端带 source 则优先。
      title: t('ruleTable.colSource'), width: 90, render: (_: unknown, r: FlatRule) => (
        <Tag style={{ marginInlineEnd: 0, fontSize: 11, borderColor: 'var(--bg-border)', color: 'var(--text-muted)', background: 'transparent' }}>
          {t(sourceLabel[ruleSource(r)] ?? '') || ruleSource(r)}
        </Tag>
      ),
    },
    {
      // 校验:valid 默认 true(Meta() 只返回已 Validate 的规则);false → 红色标记。
      // 用 Badge 实色填充 + 白字(design.md #3:原 antd Tag color=success 在浅色下黑底黑字,
      // 改 Badge tone=sev-low 绿底白字 / tone=sev-critical 红底白字,与风险级别标签统一)。
      title: t('ruleTable.colValidity'), width: 70, render: (_: unknown, r: FlatRule) => (
        r.valid === false
          ? <SevBadge tone="sev-critical">{t('ruleTable.invalid')}</SevBadge>
          : <SevBadge tone="sev-low">{t('ruleTable.valid')}</SevBadge>
      ),
    },
    { title: t('ruleTable.colDetector'), width: 120, dataIndex: 'detector' },
    {
      // 规则语法:baseline 按 op 拼、injection 为正则原文;无则 '--'。列表截断,详情抽屉展示完整。
      title: t('ruleTable.colRuleSyntax'), width: 320, ellipsis: true, render: (_: unknown, r: FlatRule) => (
        <Tooltip title={r.syntax || '--'}>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>{r.syntax || '--'}</span>
        </Tooltip>
      ),
    },
  ]

  if (allRules.length === 0) return <Empty description={t('ruleTable.empty')} />

  return (
    <Card>
      {/* 无效规则横幅:valid === false 的规则置顶提示。后端 Meta() 目前只返回已校验规则,
          此横幅为防御性能力(后端补充 valid 字段后即生效)。 */}
      {invalidRules.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('ruleTable.invalidAlert', { count: invalidRules.length })}
          description={invalidRules.map((r) => r.id).join('、')}
        />
      ) : null}
      {/* 筛选工具栏行(design.md #2:统一模式——框在结果 Card 内顶部 + 底部 hairline 分隔)。
          来源 + 级别两组筛选同一行(flex-wrap),复用 sev-seg 配色,组合:检测器 → 来源 → 级别。 */}
      <div className="filter-toolbar">
        <Segmented
          className="sev-seg"
          value={src}
          onChange={(v) => setSrc(v as string)}
          options={[
            { value: 'all', label: <SevSegLabel text={t('ruleTable.all')} count={sourceCounts.all} />, className: 'sev-tab-all' },
            ...sourceOrder.map((s) => ({
              value: s,
              label: <SevSegLabel text={t(sourceLabel[s])} count={sourceCounts[s] ?? 0} />,
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
      <Table<FlatRule>
        rowKey={(r) => `${r.detector_id}:${r.id}`}
        columns={columns}
        dataSource={filtered}
        // 分页:defaultPageSize(非受控)而非 pageSize(受控)——同 AssetTable,避免页大小选择器
        // 改动被受控 pageSize 重置回 20(详见 AssetTable 注释)。规则较多时(63 条)分页避免长列表。
        pagination={{ defaultPageSize: 20, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'], size: 'default' }}
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
