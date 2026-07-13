import { useEffect, useState } from 'react'
import { Drawer, Descriptions, Typography, Alert, Spin, Empty, Button, Modal, Input, Popconfirm, Tag, message } from 'antd'
import type { Finding, DetectorMeta, Severity, Asset } from '../types'
import { apiGet } from '../api/client'
import { useStore } from '../store'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { AssetDetailPanel } from './AssetDetailPanel'
import { formatDateTime } from '../lib/format'

const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低', info: '信息' }

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
function AssetSection({ assetId }: { assetId: string }) {
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

  if (loading) return <Spin style={{ display: 'block', margin: '40px auto' }} />
  if (err) return <Alert type="error" message="资产读取失败" description={err} showIcon />
  if (!asset) return <Empty description="未找到资产" />
  return <AssetDetailPanel asset={asset} />
}

export function FindingDrawer({ finding, detectors, startedAt, onClose }: FindingDrawerProps) {
  const detName = (id: string): string => detectors.find((x) => x.id === id)?.name ?? id
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
      message.success('已添加豁免,下次扫描生效')
      setSupprModalOpen(false)
      setSupprReason('')
    } else {
      message.error('添加豁免失败')
    }
  }

  // 加入 baseline:POST /api/baseline 跑全量扫描 + union 合并指纹。耗时操作,确认后执行。
  const [baselineLoading, setBaselineLoading] = useState(false)
  const handleGenerateBaseline = async () => {
    setBaselineLoading(true)
    const r = await generateBaseline()
    setBaselineLoading(false)
    if (r) {
      message.success(`Baseline 已更新(总计 ${r.total_fps} 指纹,新增 ${r.added_fps}),下次扫描生效`)
    } else {
      message.error('Baseline 生成失败')
    }
  }

  // key={assetId}:切换 finding 时 AssetSection 重挂载,重拉资产(防脏数据)。
  return (
    <Drawer
      title="风险详情"
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
          <Descriptions title="风险信息" size="small" column={1} bordered>
            <Descriptions.Item label="风险名称">{finding.message}</Descriptions.Item>
            <Descriptions.Item label="级别">
              <SevBadge tone={`sev-${finding.severity}` as BadgeTone}>{sevLabel[finding.severity]}</SevBadge>
            </Descriptions.Item>
            <Descriptions.Item label="检测器">{detName(finding.detector_id)}</Descriptions.Item>
            <Descriptions.Item label="规则 ID">
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 14 }}>{finding.rule_id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label="规则语法">
              {/* 规则语法用纯代码格式(monospace 等宽、无标签背景框),字体放大到 14 便于阅读;长语法换行不撑破布局。 */}
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 14, wordBreak: 'break-all', color: 'var(--text)' }}>
                {findSyntax(detectors, finding.detector_id, finding.rule_id) ?? '--'}
              </span>
            </Descriptions.Item>
            <Descriptions.Item label="扫描时间">
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>
                {startedAt ? formatDateTime(startedAt) : '--'}
              </span>
            </Descriptions.Item>
            <Descriptions.Item label="命中证据">
              <Typography.Paragraph style={{ margin: 0, fontFamily: 'var(--font-mono)', fontSize: 12 }} ellipsis={{ rows: 3, expandable: true, symbol: '展开' }}>
                {finding.evidence || '(无)'}
              </Typography.Paragraph>
            </Descriptions.Item>
            <Descriptions.Item label="修复建议">{finding.remediation || '(无)'}</Descriptions.Item>
            {/* 抑制状态:suppressed=true 表示已被 baseline/inline 豁免,显示来源 + reason。 */}
            <Descriptions.Item label="抑制状态">
              {finding.suppressed ? (
                <Tag style={{ marginInlineEnd: 0, borderColor: 'var(--bg-border)', background: 'var(--surface-2)', color: 'var(--text-muted)' }}>
                  已抑制 · {finding.suppression ?? '--'}{finding.reason ? ` · ${finding.reason}` : ''}
                </Tag>
              ) : (
                <Typography.Text type="secondary">活跃(未抑制)</Typography.Text>
              )}
            </Descriptions.Item>
          </Descriptions>

          <div>
            <Typography.Title level={5} style={{ marginTop: 8 }}>资产信息</Typography.Title>
            <AssetSection key={finding.asset_id} assetId={finding.asset_id} />
          </div>

          {/* 抑制操作:添加到 suppressions(需 fingerprint)+ 加入 baseline(全量扫描合并)。
              成功后不自动重扫——suppressed 状态在用户下次手动扫描时才反映(brief 未要求自动重扫)。 */}
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {hasFingerprint ? (
              <Button size="small" onClick={() => setSupprModalOpen(true)}>添加到 suppressions</Button>
            ) : (
              <Tag title="该 finding 无规则指纹(非 RulesDetector 产生),无法按指纹抑制" style={{ color: 'var(--text-dim)', borderColor: 'var(--bg-border)', background: 'transparent' }}>
                无法抑制(无指纹)
              </Tag>
            )}
            <Popconfirm
              title="加入 baseline?"
              description="将跑一次全量扫描,把所有当前 finding 的指纹合并到 baseline。"
              okText="确认"
              cancelText="取消"
              onConfirm={handleGenerateBaseline}
              disabled={baselineLoading}
            >
              <Button size="small" loading={baselineLoading}>加入 baseline</Button>
            </Popconfirm>
          </div>
        </div>
      ) : null}

      {/* 抑制原因输入 Modal:预填 fingerprint,用户填 reason 后提交 POST /api/suppressions。 */}
      <Modal
        title="添加到 suppressions"
        open={supprModalOpen}
        onOk={handleAddSuppression}
        onCancel={() => { setSupprModalOpen(false); setSupprReason('') }}
        okText="添加"
        cancelText="取消"
        confirmLoading={submitting}
        okButtonProps={{ disabled: !supprReason.trim() }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>规则指纹</Typography.Text>
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all', color: 'var(--text)', marginTop: 4 }}>
              {finding?.fingerprint ?? '--'}
            </div>
          </div>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>豁免原因</Typography.Text>
            <Input.TextArea
              style={{ marginTop: 4 }}
              rows={3}
              value={supprReason}
              onChange={(e) => setSupprReason(e.target.value)}
              placeholder="如:已知风险,经评审接受"
              autoFocus
            />
          </div>
          <Typography.Text type="secondary" style={{ fontSize: 11 }}>
            豁免后该指纹的 finding 将标记为「已抑制」,不计入健康分。下次扫描生效。
          </Typography.Text>
        </div>
      </Modal>
    </Drawer>
  )
}
