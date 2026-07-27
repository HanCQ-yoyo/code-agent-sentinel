import { useEffect, useMemo, useState } from 'react'
import { Drawer, Descriptions, Typography, Alert, Spin, Empty, Radio, Input, Button, Space, Tag, message } from 'antd'
import { useTranslation } from 'react-i18next'
import type { Finding, DetectorMeta, Asset, Severity } from '../types'
import { apiGet } from '../api/client'
import { useStore } from '../store'
import { useTheme } from '../theme'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { ContentArea } from './ContentArea'
import { relativeClaudePath } from '../lib/path'
import { formatDateTime } from '../lib/format'
import { SEVERITY_LABEL_KEY } from '../lib/severity'
import { detectorNameById, ruleNameById } from '../lib/i18n-names'

// Severity → 优先级回退(无显式 priority 时按严重度派生)。info 归 P3(与 low 同档,均不影响健康分)。
// Task 14 在 FindingTable 里有同名 helper;此处按 brief 内联,避免过度重构跨文件抽取共享。
const severityToPrio = (s: Severity): string =>
  ({ critical: 'P0', high: 'P1', medium: 'P2', low: 'P3', info: 'P3' } as Record<Severity, string>)[s]

interface FindingDrawerProps {
  finding: Finding | null
  detectors: DetectorMeta[]
  // 整次扫描起始时间(同一次扫描所有行共享),透传自 scan.started_at。
  startedAt?: string
  onClose: () => void
}

// 找规则语法:在 detectors 里按 detector_id 定位检测器,再按 rule_id 找 RuleInfo.syntax。
// 子进程检测器 rules 为 null/无匹配 → 返回 undefined(抽屉显示 '--')。
function findSyntax(detectors: DetectorMeta[], detectorId: string, ruleId: string): string | undefined {
  const d = detectors.find((x) => x.id === detectorId)
  const r = d?.rules?.find((x) => x.id === ruleId)
  return r?.syntax
}

// 资产区:按 finding.asset_id 拉完整 Asset(含 content),仅展示「资产文件路径 + 资产内容」两项。
// 风险管理抽屉聚焦定位命中位置,不需要 AssetDetailPanel 的完整四分区(类型/scope/属性/风险列表/
// 安全检查按钮等)——那些是资产发现页的视角,此处属冗余信息。直接走 apiGet(不经 store.wrap):
// wrap 吞所有错误返 undefined,会让 .catch 死代码、失败时误报「未找到资产」。此处需细粒度错误,
// 故与 AssetDetail.tsx 同模式自管 err。
//
// locations:从 finding 透传(后端 ruleengine.Location 序列化为 snake_case line/start_col/end_col,
// 仅 RulesDetector 填充;子进程检测器 finding 无此字段)。在此边界映射为 camelCase highlights
// 传给 ContentArea→MonacoViewer(Monaco Range API 用 camelCase),命中行高亮定位风险位置。
function AssetSection({ assetId, locations, agentId }: { assetId: string, locations?: { line: number; start_col: number; end_col: number }[], agentId?: string }) {
  const { t } = useTranslation()
  const { theme } = useTheme()
  const [asset, setAsset] = useState<Asset | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let stale = false
    setLoading(true)
    setErr(null)
    setAsset(null)
    apiGet<Asset>(`/api/assets/${encodeURIComponent(assetId)}`)
      .then((a) => { if (!stale) setAsset(a) })
      .catch((e) => { if (!stale) setErr(String(e)) })
      .finally(() => { if (!stale) setLoading(false) })
    return () => { stale = true }
  }, [assetId])

  // snake_case → camelCase 映射(必须在 early return 之前,遵守 Hooks 顺序规则)。
  // 无 locations(undefined/空)→ highlights 为 undefined,MonacoViewer 不加装饰
  //(优雅降级:子进程检测器 finding 无 locations 不高亮、不报错)。
  // useMemo 稳定引用:FindingDrawer 因抑制 Modal 输入等状态变化重渲染时,locations 引用不变,
  // highlights 不重建 → MonacoViewer highlights effect 不重跑 → 避免 revealLineInCenter 在每次
  // 键盘输入时把编辑器滚回命中行。
  const highlights = useMemo(
    () => locations && locations.length > 0
      ? locations.map((l) => ({ line: l.line, startCol: l.start_col, endCol: l.end_col }))
      : undefined,
    [locations],
  )

  if (loading) return <Spin style={{ display: 'block', margin: '40px auto' }} />
  if (err) return <Alert type="error" message={t('findingDrawer.loadFailed')} description={err} showIcon />
  if (!asset) return <Empty description={t('findingDrawer.notFound')} />
  // 仅展示资产文件路径 + 资产内容(borderless,只读,带命中位置高亮)。
  // 去掉 AssetDetailPanel 的类型/scope 标签、属性、风险列表、安全检查按钮(风险管理抽屉不需要)。
  // agentId 保留透传意义不再(无安全检查),留参数不动以免改动调用方签名。
  void agentId
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
      <div>
        <div className="asset-section-title">{t('findingDrawer.assetPath')}</div>
        <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', wordBreak: 'break-all' }} title={asset.source_path}>{relativeClaudePath(asset.source_path)}</Typography.Text>
      </div>
      {/* .asset-detail-content 作用域:让 borderless ContentArea 的内容标题(.content-area-label)
          走 .asset-section-title 同款大标题,与上方「资产文件路径」标题统一。 */}
      <div className="asset-detail-content">
        <ContentArea asset={asset} theme={theme} highlights={highlights} borderless readOnly />
      </div>
    </div>
  )
}

// 处置面板:对带 fingerprint 的 finding 设状态/优先级/备注,落盘到 ~/.claude-sentinel/finding_states.yaml。
// Task 15:取代旧的「添加到 suppressions」+「加入 baseline」两按钮区块。
//   - 旧 addSuppression → POST /api/suppressions(Task 11 已删端点)
//   - 旧 generateBaseline → POST /api/baseline(Task 11 重定义为 bulk-accept;Task 12 的 bulkAccept action 取代)
// 改用 Task 12 的 setFindingState/resetFindingState:统一处置生命周期(status/priority/note),
//   后端 /api/finding-state POST/DELETE,API 读时把治理字段合并到 Finding 上。
// severityToPrio 内联(同 FindingTable,不抽共享以免过度重构)。
function DispositionPanel({ finding }: { finding: Finding }) {
  const { t } = useTranslation()
  const [status, setStatus] = useState(finding.status ?? 'open')
  const [priority, setPriority] = useState(finding.priority ?? severityToPrio(finding.severity))
  const [note, setNote] = useState(finding.note ?? '')
  const setFindingState = useStore((s) => s.setFindingState)
  const resetFindingState = useStore((s) => s.resetFindingState)

  const save = async () => {
    await setFindingState(finding.fingerprint!, status, priority, note)
    message.success(t('findingDrawer.saved'))
  }
  const reset = async () => {
    await resetFindingState(finding.fingerprint!)
    setStatus('open'); setPriority(severityToPrio(finding.severity)); setNote('')
    message.success(t('findingDrawer.reset'))
  }

  return (
    <div style={{ marginTop: 16 }}>
      <div className="asset-section-title">{t('findingDrawer.disposition')}</div>
      <Space direction="vertical" style={{ width: '100%' }}>
        <div>
          <Typography.Text type="secondary">{t('findingDrawer.status')}</Typography.Text>
          <Radio.Group value={status} onChange={(e) => setStatus(e.target.value)} style={{ marginLeft: 8 }}>
            {['open', 'in_progress', 'resolved', 'false_positive', 'accepted'].map((s) => (
              <Radio.Button key={s} value={s}>{t(`findingTable.status.${s}`)}</Radio.Button>
            ))}
          </Radio.Group>
        </div>
        <div>
          <Typography.Text type="secondary">{t('findingDrawer.priority')}</Typography.Text>
          <Radio.Group value={priority} onChange={(e) => setPriority(e.target.value)} style={{ marginLeft: 8 }}>
            {['P0', 'P1', 'P2', 'P3'].map((p) => <Radio.Button key={p} value={p}>{p}</Radio.Button>)}
          </Radio.Group>
        </div>
        <Input.TextArea value={note} onChange={(e) => setNote(e.target.value)} placeholder={t('findingDrawer.notePlaceholder')} rows={2} />
        <Space>
          <Button type="primary" onClick={save}>{t('findingDrawer.save')}</Button>
          <Button onClick={reset}>{t('findingDrawer.resetToOpen')}</Button>
        </Space>
      </Space>
    </div>
  )
}

export function FindingDrawer({ finding, detectors, startedAt, onClose }: FindingDrawerProps) {
  const { t } = useTranslation()
  const detName = (id: string): string => detectorNameById(detectors, id)
  // Task 15:移除旧的 addSuppression/generateBaseline(useStore destructure)与
  // supprModalOpen/supprReason/submitting/baselineLoading 状态——DispositionPanel 自管 local state。
  // DispositionPanel 用 key={finding.fingerprint} 强制在 finding 切换时重挂载:其 useState
  // 初始值依赖 finding prop,不重挂载会保留旧 finding 的 status/priority/note(脏状态)。

  // key={assetId}:切换 finding 时 AssetSection 重挂载,重拉资产(防脏数据)。
  return (
    <Drawer
      title={t('findingDrawer.title')}
      placement="right"
      width="50%"
      open={finding !== null}
      onClose={onClose}
      maskClosable
      keyboard
      rootClassName="finding-drawer"
      styles={{ body: { padding: 16, overflow: 'auto' } }}
    >
      {finding ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* #9:label 列定宽 120 + nowrap,值列 word-break,table-layout:fixed 防止标签长短不一导致值列错位。
              className + index.css 的 .risk-desc table 规则为兜底(antd Descriptions 包 div,inline style 不一定生效)。 */}
          <Descriptions
            title={t('findingDrawer.infoTitle')}
            size="small"
            column={1}
            bordered
            labelStyle={{ width: 120, minWidth: 120, whiteSpace: 'nowrap' }}
            contentStyle={{ wordBreak: 'break-all' }}
            style={{ tableLayout: 'fixed' }}
            className="risk-desc"
          >
            <Descriptions.Item label={t('findingDrawer.name')}>{ruleNameById(finding.rule_id, finding.message)}</Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.severity')}>
              <SevBadge tone={`sev-${finding.severity}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[finding.severity])}</SevBadge>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.detector')}>{detName(finding.detector_id)}</Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.ruleId')}>
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 14 }}>{finding.rule_id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.ruleSyntax')}>
              {/* 规则语法用纯代码格式(monospace 等宽、无标签背景框),字体放大到 14 便于阅读;长语法换行不撑破布局。 */}
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 14, wordBreak: 'break-all', color: 'var(--text)' }}>
                {findSyntax(detectors, finding.detector_id, finding.rule_id) ?? '--'}
              </span>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.scanTime')}>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>
                {startedAt ? formatDateTime(startedAt) : '--'}
              </span>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.evidence')}>
              <Typography.Paragraph style={{ margin: 0, fontFamily: 'var(--font-mono)', fontSize: 12 }} ellipsis={{ rows: 3, expandable: true, symbol: t('common.expand') }}>
                {finding.evidence || t('common.none')}
              </Typography.Paragraph>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.remediation')}>{finding.remediation || t('common.none')}</Descriptions.Item>
            {/* 抑制状态显示段(保留):suppressed=true 表示已被 baseline/inline 豁免;Task 11 后 applyFindingState
                也把 Suppression 设为 "state"。仍展示来源 + reason,DispositionPanel 在下方负责改写。 */}
            <Descriptions.Item label={t('findingDrawer.supprStatus')}>
              {finding.suppressed ? (
                <Tag style={{ marginInlineEnd: 0, borderColor: 'var(--bg-border)', background: 'var(--surface-2)', color: 'var(--text-muted)' }}>
                  {t('findingDrawer.suppressedTag')} · {finding.suppression ?? '--'}{finding.reason ? ` · ${finding.reason}` : ''}
                </Tag>
              ) : (
                <Typography.Text type="secondary">{t('findingDrawer.active')}</Typography.Text>
              )}
            </Descriptions.Item>
          </Descriptions>

          <div>
            <Typography.Title level={5} style={{ marginTop: 8 }}>{t('findingDrawer.assetInfo')}</Typography.Title>
            <AssetSection key={finding.asset_id} assetId={finding.asset_id} locations={finding.locations} agentId={finding.agent_id} />
          </div>

          {/* 处置面板:需 fingerprint(仅 RulesDetector 填充)。无 fingerprint 显示提示。
              key={finding.fingerprint} 强制 DispositionPanel 在 finding 切换时重挂载:
              其 useState(status/priority/note)初始值从 finding prop 取,不重挂载会保留旧 finding
              的处置状态(脏数据,甚至可能把 A 的状态写到 B 的 fingerprint)。 */}
          {finding.fingerprint ? (
            <DispositionPanel key={finding.fingerprint} finding={finding} />
          ) : (
            <Typography.Text type="secondary">{t('findingDrawer.noFingerprint')}</Typography.Text>
          )}
        </div>
      ) : null}
    </Drawer>
  )
}
