import { useEffect, useState } from 'react'
import { Drawer, Descriptions, Typography, Alert, Spin, Empty } from 'antd'
import type { Finding, DetectorMeta, Severity, Asset } from '../types'
import { apiGet } from '../api/client'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { AssetDetailPanel } from './AssetDetailPanel'
import { formatDateTime } from '../lib/format'

const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }

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
  // key={assetId}:切换 finding 时 AssetSection 重挂载,重拉资产(防脏数据)。
  return (
    <Drawer
      title="风险详情"
      placement="right"
      width="50%"
      open={finding !== null}
      onClose={onClose}
      mask={false}
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
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{finding.rule_id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label="规则语法">
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>
                {findSyntax(detectors, finding.detector_id, finding.rule_id) ?? '--'}
              </Typography.Text>
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
          </Descriptions>

          <div>
            <Typography.Title level={5} style={{ marginTop: 8 }}>资产信息</Typography.Title>
            <AssetSection key={finding.asset_id} assetId={finding.asset_id} />
          </div>
        </div>
      ) : null}
    </Drawer>
  )
}
