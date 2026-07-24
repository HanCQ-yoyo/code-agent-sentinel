import { useState } from 'react'
import { Typography, Alert, Table, Button, Modal, Checkbox, Space } from 'antd'
import { useTranslation } from 'react-i18next'
import type { ColumnsType } from 'antd/es/table'
import type { Asset, Finding, DetectorMeta, Severity } from '../types'
import { Badge, type BadgeTone } from './Badge'
import { relativeClaudePath } from '../lib/path'
import { AssetEditor } from './AssetEditor'
import { SEVERITY_ORDER, SEVERITY_LABEL_KEY } from '../lib/severity'
import { detectorNameById, ruleNameById } from '../lib/i18n-names'
import { useStore } from '../store'

// AssetDetailPanel:资产详情。三消费方(Assets 列表抽屉 50% / 树右栏 480px sticky / /assets/:id 全页)
// 共用此组件,签名 { asset, findings?, detectors? }。
//
// 四分区层次重排(design.md #3):身份 → 属性 → 风险 → 内容。
// 用 .section-label(muted uppercase + hairline)串联四块,消除原 4 块平铺的层次塌平。
//  - meta 区弃 Descriptions bordered 重表格,改 <dl> 轻量键值对(label muted + value mono)。
//  - risk section 保留 <div data-testid="asset-risk-list"> 容器(e2e testid 钩子,容器元素不动),
//    仅把 Typography.Title 换成 section-label,与 meta 区标题统一。
// 注:e2e 516(资产风险列)在 main HEAD 上即为本就存在的扫描时序 flake(findings 在 drawer 打开时
// 未及时就绪),与本组件结构无关 —— 见排查记录。故此处放开做完整四分区,不为此牺牲层次。
export function AssetDetailPanel({ asset, highlights, findings, detectors, agentID }: { asset: Asset, highlights?: { line: number; startCol: number; endCol: number }[], findings?: Finding[], detectors?: DetectorMeta[], agentID?: string }) {
  const { t } = useTranslation()
  const { runScan, detectors: storeDetectors } = useStore()
  const description = (asset.fields as Record<string, unknown> | undefined)?.description
  const [checkOpen, setCheckOpen] = useState(false)
  const [checkDets, setCheckDets] = useState<string[]>([])
  const openCheck = () => {
    setCheckDets((storeDetectors ?? []).map(d => d.id))
    setCheckOpen(true)
  }
  const startCheck = async () => {
    const det = checkDets.length === (storeDetectors ?? []).length ? undefined : checkDets.join(',')
    await runScan(agentID ? [agentID] : [], det, { type: 'asset-id', path: asset.id })
    setCheckOpen(false)
  }
  const isMarkdown = ['memory', 'skill', 'command', 'agent'].includes(asset.type)

  const assetFindings = (findings ?? []).filter((f) => f.asset_id === asset.id)
  const sortedFindings = [...assetFindings].sort((a, b) => SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity))

  const riskColumns: ColumnsType<Finding> = [
    {
      title: t('assetDetail.riskColName'), ellipsis: true, render: (_: unknown, f: Finding) => (
        <Typography.Text title={ruleNameById(f.rule_id, f.message)}>{ruleNameById(f.rule_id, f.message)}</Typography.Text>
      ),
    },
    { title: t('assetDetail.riskColSeverity'), width: 80, render: (_: unknown, f: Finding) => <Badge tone={`sev-${f.severity}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[f.severity])}</Badge> },
    { title: t('assetDetail.riskColDetector'), width: 140, ellipsis: true, render: (_: unknown, f: Finding) => (
      <Typography.Text style={{ fontSize: 12 }}>{detectorNameById(detectors ?? [], f.detector_id)}</Typography.Text>
    ) },
    { title: t('assetDetail.riskColRule'), width: 200, ellipsis: true, render: (_: unknown, f: Finding) => (
      <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{f.rule_id}</Typography.Text>
    ) },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '100%' }}>
      {/* ① 身份区:资产名 + type/scope badge + description 副标题 + 右侧「安全检测」操作(操作归位 header)。 */}
      <div>
        <h2 data-testid="asset-detail-name" style={{ color: 'var(--color-ink)', margin: '0 0 4px', fontSize: 'var(--fs-xl)', fontWeight: 700, letterSpacing: '-0.01em' }}>{asset.name}</h2>
        {isMarkdown && typeof description === 'string' && description ? (
          <Typography.Text style={{ display: 'block', marginBottom: 8, color: 'var(--color-muted)' }}>{description}</Typography.Text>
        ) : null}
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <Badge tone="neutral">{asset.type}</Badge>
          <Badge tone={`scope-${asset.scope}` as BadgeTone}>{asset.scope}</Badge>
          <Button size="small" style={{ marginLeft: 'auto' }} onClick={openCheck}>{t('rescan.check')}</Button>
        </div>
      </div>

      {asset.parse_error ? (
        <Alert type="error" message={t('assetDetail.parseError')} description={asset.parse_error} showIcon />
      ) : null}

      {/* ② 属性区:路径/hash/mtime 轻量键值对。弃 Descriptions bordered 重表格,改 <dl> + section-label,
          label muted 小字 + value mono,像文档 meta 块,层次在文字本身而非容器框。 */}
      <section>
        <div className="section-label">{t('assetDetail.metaTitle')}</div>
        <dl style={{ margin: '4px 0 0', display: 'flex', flexDirection: 'column', gap: 4 }}>
          <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
            <dt style={{ width: 64, flexShrink: 0, fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', textTransform: 'uppercase', letterSpacing: '0.04em' }}>{t('assetDetail.path')}</dt>
            <dd style={{ margin: 0, flex: 1, minWidth: 0 }}><Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', wordBreak: 'break-all' }}>{relativeClaudePath(asset.source_path)}</Typography.Text></dd>
          </div>
          <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
            <dt style={{ width: 64, flexShrink: 0, fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', textTransform: 'uppercase', letterSpacing: '0.04em' }}>{t('assetDetail.hash')}</dt>
            <dd style={{ margin: 0, flex: 1, minWidth: 0 }}><Typography.Text code copyable style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)' }}>{asset.hash}</Typography.Text></dd>
          </div>
          <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
            <dt style={{ width: 64, flexShrink: 0, fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', textTransform: 'uppercase', letterSpacing: '0.04em' }}>{t('assetDetail.mtime')}</dt>
            <dd style={{ margin: 0, flex: 1, minWidth: 0 }}><span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', color: 'var(--color-muted)', fontVariantNumeric: 'tabular-nums' }}>{asset.mtime ?? '--'}</span></dd>
          </div>
        </dl>
      </section>

      {/* ③ 风险区:保留 <div data-testid="asset-risk-list"> 容器(e2e testid 钩子,容器元素不变),
          标题用 section-label 与 meta 区统一。findings 未传时不渲染;传了但无风险显示空态。 */}
      {findings ? (
        <div data-testid="asset-risk-list">
          <div className="section-label" style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
            <span>{t('assetDetail.riskListTitle')}</span>
            {assetFindings.length > 0 ? (
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', fontVariantNumeric: 'tabular-nums' }}>{assetFindings.length}</span>
            ) : null}
          </div>
          <Table<Finding>
            rowKey={(_f, i) => String(i)}
            columns={riskColumns}
            dataSource={sortedFindings}
            pagination={false}
            size="small"
            locale={{ emptyText: t('assetDetail.riskEmpty') }}
          />
        </div>
      ) : null}

      {/* ④ 内容区:资产文件内容 ContentArea。key={asset.id} 切资产重挂载,避免视图/草稿泄漏。 */}
      <AssetEditor key={asset.id} asset={asset} highlights={highlights} />

      {/* 安全检查:scope=asset-id 按 ID 单扫。getContainer={false} 渲染进 Drawer DOM 树。 */}
      <Modal
        open={checkOpen}
        title={t('rescan.checkTitle')}
        onCancel={() => setCheckOpen(false)}
        onOk={startCheck}
        okText={t('rescan.start')}
        cancelText={t('common.cancel')}
        getContainer={false}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Typography.Text type="secondary">{t('rescan.checkHint')}</Typography.Text>
          <div>
            <Typography.Text strong>{t('rescan.detectors')}</Typography.Text>
            <Checkbox.Group
              value={checkDets}
              onChange={(v) => setCheckDets(v as string[])}
              options={(storeDetectors ?? []).map(d => ({ label: d.name ?? d.id, value: d.id, disabled: d.available === false }))}
              style={{ display: 'block', marginTop: 4 }}
            />
          </div>
        </Space>
      </Modal>
    </div>
  )
}
