import { useEffect, useMemo, useState } from 'react'
import { Drawer, Descriptions, Typography, Alert, Spin, Empty, Modal, Input, Button, Space, Tag, message } from 'antd'
import { useTranslation } from 'react-i18next'
import type { Finding, DetectorMeta, Asset } from '../types'
import { apiGet } from '../api/client'
import { useStore } from '../store'
import { useTheme } from '../theme'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { ContentArea } from './ContentArea'
import { relativeClaudePath } from '../lib/path'
import { formatDateTime } from '../lib/format'
import { SEVERITY_LABEL_KEY, severityToPrio } from '../lib/severity'
import { detectorNameById, ruleNameById } from '../lib/i18n-names'

// 处置状态 → 背景色 token 映射(替代旧 TAG_HEX 的 antd 原色 hex,统一到 OKLCH 体系)。
// 状态语义:open=中性、in_progress=cat-1 蓝、resolved=sev-low 绿、false_positive=cat-5 紫、accepted=cat-3 黄。
const STATUS_BG: Record<string, string> = {
  open: 'var(--color-rule-2)',
  in_progress: 'var(--cat-1)',
  resolved: 'var(--sev-low-solid)',
  false_positive: 'var(--cat-5)',
  accepted: 'var(--cat-3)',
}
// 优先级 → 背景色 token:P0 红(critical-solid)→ P3 蓝(cat-1),复用 severity/category 实色。
const PRIO_BG: Record<string, string> = {
  P0: 'var(--sev-critical-solid)',
  P1: 'var(--sev-high-solid)',
  P2: 'var(--sev-medium-solid)',
  P3: 'var(--cat-1)',
}

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

// 处置弹框:对带 fingerprint 的 finding 设状态/优先级/备注,落盘到 ~/.claude-sentinel/finding_states.yaml。
// Task 9:由原 DispositionPanel(Radio.Group 内联表单)改为 Modal 弹框,状态/优先级改用带色
// Tag.CheckableTag 选择器;由 FindingDrawer「立即处置」按钮 + Findings.tsx 列表操作列复用触发。
//   - 旧 addSuppression/generateBaseline(Task 11/12 已删)
//   - setFindingState/resetFindingState:统一处置生命周期,后端 /api/finding-state POST/DELETE
// severityToPrio 从 lib/severity 导入(消除与 FindingTable 的重复 helper)。
// CheckableTag 选中态用 STATUS_BG/PRIO_BG token 映射取 OKLCH 实色背景(统一到项目 token 体系)。
export function DispositionModal({ finding, open, onClose }: { finding: Finding; open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const [status, setStatus] = useState(finding.status ?? 'open')
  const [priority, setPriority] = useState(finding.priority ?? severityToPrio(finding.severity))
  const [note, setNote] = useState(finding.note ?? '')
  const setFindingState = useStore((s) => s.setFindingState)
  const resetFindingState = useStore((s) => s.resetFindingState)

  const save = async () => {
    await setFindingState(finding.fingerprint!, status, priority, note)
    message.success(t('findingDrawer.saved'))
    onClose()
  }
  const reset = async () => {
    await resetFindingState(finding.fingerprint!)
    setStatus('open'); setPriority(severityToPrio(finding.severity)); setNote('')
    message.success(t('findingDrawer.reset'))
    onClose()
  }

  return (
    <Modal open={open} title={t('findingDrawer.disposition')} onCancel={onClose}
      footer={[
        <Button key="reset" onClick={reset}>{t('findingDrawer.resetToOpen')}</Button>,
        <Button key="cancel" onClick={onClose}>{t('common.cancel')}</Button>,
        <Button key="save" type="primary" onClick={save}>{t('findingDrawer.save')}</Button>,
      ]}>
      <Space direction="vertical" style={{ width: '100%' }}>
        <div>
          <Typography.Text type="secondary">{t('findingDrawer.status')}</Typography.Text>
          <div style={{ marginTop: 4, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {['open', 'in_progress', 'resolved', 'false_positive', 'accepted'].map((s) => (
              <Tag.CheckableTag key={s} checked={status === s} onChange={() => setStatus(s)}
                style={status === s ? { background: STATUS_BG[s] ?? 'var(--color-rule-2)', borderColor: 'transparent', color: 'var(--badge-text)' } : {}}>
                {t(`findingTable.status.${s}`)}
              </Tag.CheckableTag>
            ))}
          </div>
        </div>
        <div>
          <Typography.Text type="secondary">{t('findingDrawer.priority')}</Typography.Text>
          <div style={{ marginTop: 4, display: 'flex', gap: 8 }}>
            {['P0', 'P1', 'P2', 'P3'].map((p) => (
              <Tag.CheckableTag key={p} checked={priority === p} onChange={() => setPriority(p)}
                style={priority === p ? { background: PRIO_BG[p] ?? 'var(--color-rule-2)', borderColor: 'transparent', color: 'var(--badge-text)' } : {}}>
                {p}
              </Tag.CheckableTag>
            ))}
          </div>
        </div>
        <Input.TextArea value={note} onChange={(e) => setNote(e.target.value)} placeholder={t('findingDrawer.notePlaceholder')} rows={2} />
      </Space>
    </Modal>
  )
}

export function FindingDrawer({ finding, detectors, startedAt, onClose }: FindingDrawerProps) {
  const { t } = useTranslation()
  const detName = (id: string): string => detectorNameById(detectors, id)
  // Task 15:移除旧的 addSuppression/generateBaseline(useStore destructure)与
  // supprModalOpen/supprReason/submitting/baselineLoading 状态——DispositionPanel 自管 local state。
  // DispositionPanel 用 key={finding.fingerprint} 强制在 finding 切换时重挂载:其 useState
  // 初始值依赖 finding prop,不重挂载会保留旧 finding 的 status/priority/note(脏状态)。
  // Task 9:DispositionPanel → DispositionModal,由「立即处置」按钮触发(disposeOpen state)。
  const [disposeOpen, setDisposeOpen] = useState(false)

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
          {/* Task 9:「立即处置」按钮(有 fingerprint 才显示)。已处置(status≠open)显示「已处置」文案,
              否则「立即处置」。点击打开 DispositionModal(底部 key={finding.fingerprint} 重挂载保证 state 重置)。 */}
          {finding.fingerprint ? (
            <Button type="primary" onClick={() => setDisposeOpen(true)} style={{ marginBottom: 12, alignSelf: 'flex-start' }}>
              {finding.status && finding.status !== 'open' ? t('findingDrawer.disposed') : t('findingDrawer.disposeNow')}
            </Button>
          ) : null}
          {/* #9:label 列定宽 120 + nowrap,值列 word-break,table-layout:fixed 防止标签长短不一导致值列错位。
              className + index.css 的 .risk-desc table 规则为兜底(antd Descriptions 包 div,inline style 不一定生效)。 */}
          <Descriptions
            title={t('findingDrawer.infoTitle')}
            size="small"
            column={2}
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
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-base)' }}>{finding.rule_id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.ruleSyntax')} span={2}>
              {/* 规则语法用纯代码格式(monospace 等宽、无标签背景框),字体放大到 14 便于阅读;长语法换行不撑破布局。 */}
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-base)', wordBreak: 'break-all', color: 'var(--color-ink)' }}>
                {findSyntax(detectors, finding.detector_id, finding.rule_id) ?? '--'}
              </span>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.scanTime')}>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-sm)' }}>
                {startedAt ? formatDateTime(startedAt) : '--'}
              </span>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.evidence')} span={2}>
              <Typography.Paragraph style={{ margin: 0, fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-sm)' }} ellipsis={{ rows: 3, expandable: true, symbol: t('common.expand') }}>
                {finding.evidence || t('common.none')}
              </Typography.Paragraph>
            </Descriptions.Item>
            <Descriptions.Item label={t('findingDrawer.remediation')} span={2}>{finding.remediation || t('common.none')}</Descriptions.Item>
            {/* 抑制状态显示段(保留):suppressed=true 表示已被 baseline/inline 豁免;Task 11 后 applyFindingState
                也把 Suppression 设为 "state"。仍展示来源 + reason,DispositionPanel 在下方负责改写。 */}
            <Descriptions.Item label={t('findingDrawer.supprStatus')} span={2}>
              {finding.suppressed ? (
                <Tag style={{ marginInlineEnd: 0, borderColor: 'var(--color-rule)', background: 'var(--color-surface)', color: 'var(--color-muted)' }}>
                  {t('findingDrawer.suppressedTag')} · {finding.suppression ?? '--'}{finding.reason ? ` · ${finding.reason}` : ''}
                </Tag>
              ) : (
                <Typography.Text type="secondary">{t('findingDrawer.active')}</Typography.Text>
              )}
            </Descriptions.Item>
            {/* Task 9:处置状态只读摘要(status≠open 显示带色 Tag,否则 secondary 文案)+ 优先级 Tag。
                改写入口在上方「立即处置」按钮(DispositionModal),此处仅展示当前值。 */}
            <Descriptions.Item label={t('findingDrawer.disposition')} span={2}>
              {finding.status && finding.status !== 'open' ? (
                <Tag style={{ background: STATUS_BG[finding.status] ?? 'var(--color-rule-2)', color: 'var(--badge-text)', border: 'none' }}>{t(`findingTable.status.${finding.status}`)}</Tag>
              ) : <Typography.Text type="secondary">{t('findingTable.status.open')}</Typography.Text>}
              {finding.priority ? <Tag style={{ marginLeft: 6, background: PRIO_BG[finding.priority] ?? 'var(--color-rule-2)', color: 'var(--badge-text)', border: 'none' }}>{finding.priority}</Tag> : null}
            </Descriptions.Item>
          </Descriptions>

          <div>
            <Typography.Title level={5} style={{ marginTop: 8 }}>{t('findingDrawer.assetInfo')}</Typography.Title>
            <AssetSection key={finding.asset_id} assetId={finding.asset_id} locations={finding.locations} agentId={finding.agent_id} />
          </div>

          {/* Task 9:处置弹框(需 fingerprint,仅 RulesDetector 填充)。
              key={finding.fingerprint} 强制 DispositionModal 在 finding 切换时重挂载:
              其 useState(status/priority/note)初始值从 finding prop 取,不重挂载会保留旧 finding
              的处置状态(脏数据,甚至可能把 A 的状态写到 B 的 fingerprint)。
              无 fingerprint 显示提示(子进程检测器 finding 无法按指纹处置)。 */}
          {finding.fingerprint ? (
            <DispositionModal key={finding.fingerprint} finding={finding} open={disposeOpen} onClose={() => setDisposeOpen(false)} />
          ) : (
            <Typography.Text type="secondary">{t('findingDrawer.noFingerprint')}</Typography.Text>
          )}
        </div>
      ) : null}
    </Drawer>
  )
}
