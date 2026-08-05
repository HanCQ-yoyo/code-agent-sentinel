import { useState } from 'react'
import { Typography, Alert, Table, Button, Modal, Checkbox, Space } from 'antd'
import { useTranslation } from 'react-i18next'
import type { ColumnsType } from 'antd/es/table'
import type { Asset, Finding, DetectorMeta } from '../types'
import { Badge, type BadgeTone } from './Badge'
import { relativeClaudePath } from '../lib/path'
import { AssetEditor } from './AssetEditor'
import { SEVERITY_ORDER, SEVERITY_LABEL_KEY } from '../lib/severity'
import { detectorNameById, ruleNameById } from '../lib/i18n-names'
import { useStore } from '../store'
import { CapabilityPanel } from './CapabilityPanel'

// AssetDetailPanel:资产详情。三消费方(Assets 列表抽屉 50% / 树右栏 480px sticky / /assets/:id 全页)
// 共用此组件,签名 { asset, findings?, detectors? }。
//
// 五段层次(用户指定):基本信息 → 属性 → 能力面板 → 风险列表 → 资产内容。
// 段间用足够间距(var(--space-xl))区分,体现「不同内容块」的层次感(原版间距太小、层次塌平)。
// 三段内容标题(属性/风险列表/资产内容)用同一套 .asset-section-title(fs-lg 20px + 700 加粗),
// 比「安全检查」按钮文字(fs-base 14px)大,作为可识别的分区标题。内容区 borderless 让标题统一。
//
// 注:testid(asset-detail-name / asset-risk-list)保留不动(e2e 钩子)。
export function AssetDetailPanel({ asset, highlights, findings, detectors, agentID, hideCheckButton }: { asset: Asset, highlights?: { line: number; startCol: number; endCol: number }[], findings?: Finding[], detectors?: DetectorMeta[], agentID?: string, hideCheckButton?: boolean }) {
  const { t } = useTranslation()
  const metaLabel: React.CSSProperties = { fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', textTransform: 'uppercase', letterSpacing: '0.04em' }
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
      <Typography.Text style={{ fontSize: 'var(--fs-xs)' }}>{detectorNameById(detectors ?? [], f.detector_id)}</Typography.Text>
    ) },
    { title: t('assetDetail.riskColRule'), width: 200, ellipsis: true, render: (_: unknown, f: Finding) => (
      <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)' }}>{f.rule_id}</Typography.Text>
    ) },
  ]

  // scope badge 文案走翻译(global 显示「用户级」,与 tab 文案一致,而非裸 scope 字符串)。
  const scopeLabelKey: Record<string, string> = {
    global: 'assetDetail.scopeGlobal',
    project: 'assetDetail.scopeProject',
    managed: 'assetDetail.scopeManaged',
    plugin: 'assetDetail.scopePlugin',
  }
  const scopeText = scopeLabelKey[asset.scope] ? t(scopeLabelKey[asset.scope]) : asset.scope

  return (
    <div className="asset-detail asset-detail-flow" style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* ① 基本信息:资产名(左)+ 类型/scope 标签(名右)+ 「安全检查」按钮(最右,default 描边同顶栏);
          名下一行:资产文件路径(mono);再下一行:描述(若有)。 */}
      <header style={{ paddingBottom: 'var(--space-xl)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)', flexWrap: 'wrap' }}>
          <h2 data-testid="asset-detail-name" style={{ color: 'var(--color-ink)', margin: 0, fontSize: 'var(--fs-lg)', fontWeight: 700, letterSpacing: '-0.01em', lineHeight: 1.3, minWidth: 0, wordBreak: 'break-word' }}>{asset.name}</h2>
          {/* 类型 + scope 标签紧贴资产名右侧。 */}
          <Badge tone="neutral">{asset.type}</Badge>
          <Badge tone={`scope-${asset.scope}` as BadgeTone}>{scopeText}</Badge>
          {/* 安全检查:secondary 操作,default 描边(design.md CTA voice:次要操作 transparent+rule 边框)。
              非规避暗色对比——此页主操作是资产内容编辑,安全检查为次要,故 default 而非 primary。
              hideCheckButton 时由父级 Drawer extra header 渲染,此处不重复。 */}
          {!hideCheckButton ? <Button size="small" style={{ marginLeft: 'auto', whiteSpace: 'nowrap' }} onClick={openCheck}>{t('rescan.check')}</Button> : null}
        </div>
        {/* 资产文件路径:名下一行,mono 小字 dim。 */}
        <Typography.Text style={{ display: 'block', marginTop: 'var(--space-xs)', fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', wordBreak: 'break-all' }} title={asset.source_path}>{relativeClaudePath(asset.source_path)}</Typography.Text>
        {/* 描述:路径下一行(若有)。 */}
        {typeof description === 'string' && description ? (
          <Typography.Text style={{ display: 'block', marginTop: 'var(--space-xs)', color: 'var(--color-muted)' }}>{description}</Typography.Text>
        ) : null}
      </header>

      {asset.parse_error ? (
        <Alert type="error" message={t('assetDetail.parseError')} description={asset.parse_error} showIcon style={{ marginBottom: 'var(--space-xl)' }} />
      ) : null}

      {/* ② 属性:hash + 修改时间一行内联 mono 条(保持现格式)。标题 .asset-section-title 放大加粗。 */}
      <section style={{ paddingBottom: 'var(--space-xl)' }}>
        <div className="asset-section-title">{t('assetDetail.metaTitle')}</div>
        <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 'var(--space-md)', rowGap: 'var(--space-xs)' }}>
          <div style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
            <span style={metaLabel}>{t('assetDetail.path')}</span>
            <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', wordBreak: 'break-all' }}>{relativeClaudePath(asset.source_path)}</Typography.Text>
          </div>
          <span style={{ color: 'var(--color-rule-2)' }}>·</span>
          <div style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
            <span style={metaLabel}>{t('assetDetail.hash')}</span>
            <Typography.Text code copyable style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)' }}>{asset.hash}</Typography.Text>
          </div>
          <span style={{ color: 'var(--color-rule-2)' }}>·</span>
          <div style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
            <span style={metaLabel}>{t('assetDetail.mtime')}</span>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', color: 'var(--color-muted)', fontVariantNumeric: 'tabular-nums' }}>{asset.mtime ?? '--'}</span>
          </div>
        </div>
      </section>

      {/* ③ 能力面板:按 asset.type 分发结构化展示(skill 的 allowed-tools / hook 的 event/command / mcp 的 env 等)。
          memory 走 fields.outline(Task 9);script 无 fields 用 name/content 推导。空 → <Empty>。 */}
      <section style={{ paddingBottom: 'var(--space-xl)' }}>
        <div className="asset-section-title">{t('assetDetail.capability')}</div>
        <CapabilityPanel asset={asset} />
      </section>

      {/* ④ 风险列表:4 列表格(风险名称/级别/检测器/规则)。标题与属性同一套 .asset-section-title。
          保留 <div data-testid="asset-risk-list"> 容器(e2e 钩子不动)。findings 未传不渲染。 */}
      {findings ? (
        <div data-testid="asset-risk-list" style={{ paddingBottom: 'var(--space-xl)' }}>
          <div className="asset-section-title" style={{ display: 'flex', alignItems: 'baseline', gap: 'var(--space-sm)' }}>
            <span>{t('assetDetail.riskListTitle')}</span>
            {assetFindings.length > 0 ? (
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-sm)', color: 'var(--color-dim)', fontVariantNumeric: 'tabular-nums' }}>{assetFindings.length}</span>
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

      {/* ⑤ 资产内容:borderless ContentArea。AssetEditor 透传 borderless=true,内容区标题走
          .asset-section-title(与属性/风险同款,见 index.css .content-area-label 覆盖),
          消除 box-in-box。key={asset.id} 切资产重挂载,避免视图/草稿泄漏。 */}
      <div className="asset-detail-content" style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}>
        <AssetEditor key={asset.id} asset={asset} highlights={highlights} borderless />
      </div>

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
