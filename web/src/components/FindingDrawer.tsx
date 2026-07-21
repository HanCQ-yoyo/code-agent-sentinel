import { useEffect, useMemo, useState } from 'react'
import { Drawer, Descriptions, Typography, Alert, Spin, Empty, Button, Modal, Input, Popconfirm, Tag, message } from 'antd'
import { useTranslation } from 'react-i18next'
import type { Finding, DetectorMeta, Asset } from '../types'
import { apiGet } from '../api/client'
import { useStore } from '../store'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { AssetDetailPanel } from './AssetDetailPanel'
import { formatDateTime } from '../lib/format'
import { SEVERITY_LABEL_KEY } from '../lib/severity'
import { detectorNameById, ruleNameById } from '../lib/i18n-names'

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

// 资产区:按 finding.asset_id 拉完整 Asset(含 content),复用 AssetDetailPanel 展示路径/hash/文件内容。
// 直接走 apiGet(不经 store.wrap):wrap 吞所有错误返 undefined,会让 .catch 死代码、失败时误报「未找到资产」。
// 此处需细粒度错误,故与 AssetDetail.tsx 同模式自管 err。
//
// locations:从 finding 透传(后端 ruleengine.Location 序列化为 snake_case line/start_col/end_col,
// 仅 RulesDetector 填充;子进程检测器 finding 无此字段)。在此边界映射为 camelCase highlights
// 传给 AssetDetailPanel→AssetEditor→ContentArea→MonacoViewer(Monaco Range API 用 camelCase)。
function AssetSection({ assetId, locations }: { assetId: string, locations?: { line: number; start_col: number; end_col: number }[] }) {
  const { t } = useTranslation()
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
  return <AssetDetailPanel asset={asset} highlights={highlights} />
}

export function FindingDrawer({ finding, detectors, startedAt, onClose }: FindingDrawerProps) {
  const { t } = useTranslation()
  const detName = (id: string): string => detectorNameById(detectors, id)
  const { addSuppression, generateBaseline } = useStore()
  const [supprModalOpen, setSupprModalOpen] = useState(false)
  const [supprReason, setSupprReason] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // 切换 finding 时重置 Modal 状态(防脏数据残留)。
  useEffect(() => {
    setSupprModalOpen(false)
    setSupprReason('')
    setSubmitting(false)
  }, [finding?.id])

  // 添加到 suppressions:需 fingerprint(仅 RulesDetector 填充);无则按钮禁用 + Tooltip 说明。
  const hasFingerprint = !!finding?.fingerprint
  const handleAddSuppression = async () => {
    if (!finding?.fingerprint) return
    setSubmitting(true)
    const ok = await addSuppression({ fingerprint: finding.fingerprint, reason: supprReason.trim() })
    setSubmitting(false)
    if (ok) {
      message.success(t('findingDrawer.supprSuccess'))
      setSupprModalOpen(false)
      setSupprReason('')
    } else {
      message.error(t('findingDrawer.supprFailed'))
    }
  }

  // 加入 baseline:POST /api/baseline 跑全量扫描 + union 合并指纹。耗时操作,确认后执行。
  const [baselineLoading, setBaselineLoading] = useState(false)
  const handleGenerateBaseline = async () => {
    setBaselineLoading(true)
    const r = await generateBaseline()
    setBaselineLoading(false)
    if (r) {
      message.success(t('findingDrawer.baselineUpdated', { total: r.total_fps, added: r.added_fps }))
    } else {
      message.error(t('findingDrawer.baselineFailed'))
    }
  }

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
            {/* 抑制状态:suppressed=true 表示已被 baseline/inline 豁免,显示来源 + reason。 */}
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
            <AssetSection key={finding.asset_id} assetId={finding.asset_id} locations={finding.locations} />
          </div>

          {/* 抑制操作:添加到 suppressions(需 fingerprint)+ 加入 baseline(全量扫描合并)。
              成功后不自动重扫——suppressed 状态在用户下次手动扫描时才反映(brief 未要求自动重扫)。 */}
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {hasFingerprint ? (
              <Button size="small" onClick={() => setSupprModalOpen(true)}>{t('findingDrawer.addSuppr')}</Button>
            ) : (
              <Tag title={t('findingDrawer.noFingerprintTip')} style={{ color: 'var(--text-dim)', borderColor: 'var(--bg-border)', background: 'transparent' }}>
                {t('findingDrawer.noFingerprint')}
              </Tag>
            )}
            <Popconfirm
              title={t('findingDrawer.baselineConfirm')}
              description={t('findingDrawer.baselineConfirmDesc')}
              okText={t('findingDrawer.confirm')}
              cancelText={t('common.cancel')}
              onConfirm={handleGenerateBaseline}
              disabled={baselineLoading}
            >
              <Button size="small" loading={baselineLoading}>{t('findingDrawer.addBaseline')}</Button>
            </Popconfirm>
          </div>
        </div>
      ) : null}

      {/* 抑制原因输入 Modal:预填 fingerprint,用户填 reason 后提交 POST /api/suppressions。 */}
      <Modal
        title={t('findingDrawer.addSuppr')}
        open={supprModalOpen}
        onOk={handleAddSuppression}
        onCancel={() => { setSupprModalOpen(false); setSupprReason('') }}
        okText={t('findingDrawer.add')}
        cancelText={t('common.cancel')}
        confirmLoading={submitting}
        okButtonProps={{ disabled: !supprReason.trim() }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('findingDrawer.fingerprintLabel')}</Typography.Text>
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all', color: 'var(--text)', marginTop: 4 }}>
              {finding?.fingerprint ?? '--'}
            </div>
          </div>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('findingDrawer.reasonLabel')}</Typography.Text>
            <Input.TextArea
              style={{ marginTop: 4 }}
              rows={3}
              value={supprReason}
              onChange={(e) => setSupprReason(e.target.value)}
              placeholder={t('findingDrawer.reasonPlaceholder')}
              autoFocus
            />
          </div>
          <Typography.Text type="secondary" style={{ fontSize: 11 }}>
            {t('findingDrawer.reasonHint')}
          </Typography.Text>
        </div>
      </Modal>
    </Drawer>
  )
}
