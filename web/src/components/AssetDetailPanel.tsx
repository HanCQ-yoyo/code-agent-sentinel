import { useState } from 'react'
import { Descriptions, Typography, Alert, Table, Button, Modal, Checkbox, Space } from 'antd'
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
// 共用此组件,签名 { asset, findings?, detectors? }。阶段 C 重排:
//  1. frontmatter 上浮:markdown 资产的 description 取自 fields.description,上移到 header 副标题(人语位置)。
//  2. 二合一:旧「解析字段 Card + 文件内容 Card」→ 单一 ContentArea(structured 字段即内容,二合一)。
//  3. 内容撑满:ContentArea flex:1。
// header h2 保留 data-testid="asset-detail-name"(e2e 钩子,阶段 A 硬规则延续)。
//
// 风险列表:findings(可选)筛选出本资产的 finding,在基础信息下方用 4 列表展示
// (风险名称/级别/检测器/规则)。一个资产可能存在多个风险,故列改为数量(见 AssetTable),
// 详情抽屉在基础信息下方罗列该资产全部风险。detectors 可选,供检测器列双语名;无则回退 id。
export function AssetDetailPanel({ asset, highlights, findings, detectors, agentID }: { asset: Asset, highlights?: { line: number; startCol: number; endCol: number }[], findings?: Finding[], detectors?: DetectorMeta[], agentID?: string }) {
  const { t } = useTranslation()
  const { runScan, detectors: storeDetectors } = useStore()
  const description = (asset.fields as Record<string, unknown> | undefined)?.description
  // 安全检查:只配检测器,scope=asset-id 按 Asset.ID 精确单扫(不动 scope=asset 的 sibling 语义)。
  // getContainer={false}:Modal 渲染进 Drawer DOM 树 → 位于 Drawer 之上,修复"弹框在抽屉下面"。
  const [checkOpen, setCheckOpen] = useState(false)
  const [checkDets, setCheckDets] = useState<string[]>([])
  const openCheck = () => {
    setCheckDets((storeDetectors ?? []).map(d => d.id)) // 默认全选
    setCheckOpen(true)
  }
  const startCheck = async () => {
    const det = checkDets.length === (storeDetectors ?? []).length ? undefined : checkDets.join(',')
    // agentID 空 → 不传 agents(后端回退 SelectedAgentID);scope=asset-id+path=asset.id 精确单扫。
    await runScan(agentID ? [agentID] : [], det, { type: 'asset-id', path: asset.id })
    setCheckOpen(false)
  }
  const isMarkdown = ['memory', 'skill', 'command', 'agent'].includes(asset.type)

  // 本资产的风险:按 asset_id 筛选,按严重度排序(critical→info)。
  const assetFindings = (findings ?? []).filter((f) => f.asset_id === asset.id)
  const sortedFindings = [...assetFindings].sort((a, b) => SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity))

  // 风险列表 4 列:风险名称(规则双语名)/级别/检测器(双语名)/规则(rule_id)。
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
      <div>
        <h2 data-testid="asset-detail-name" style={{ color: 'var(--text)', margin: '0 0 4px' }}>{asset.name}</h2>
        {isMarkdown && typeof description === 'string' && description ? (
          <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>{description}</Typography.Text>
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

      {/* 基础信息:路径/hash/修改时间 三字段垂直纵向摆放(column=1),每行一个字段。
          原 column=2 时 hash 与 mtime 横向并排,改为纵向后三行各自独占,信息更清晰、长路径/hash 不被挤。 */}
      <Descriptions size="small" column={1} bordered>
        <Descriptions.Item label={t('assetDetail.path')}>
          <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{relativeClaudePath(asset.source_path)}</Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label={t('assetDetail.hash')}>
          <Typography.Text code copyable style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{asset.hash}</Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label={t('assetDetail.mtime')}>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{asset.mtime ?? '--'}</span>
        </Descriptions.Item>
      </Descriptions>

      {/* 风险列表:基础信息下方,4 列(风险名称/级别/检测器/规则)。findings 未传(如树右栏无 scan
          上下文)时不渲染;传了但无风险则显示空态。 */}
      {findings ? (
        <div data-testid="asset-risk-list">
          <Typography.Title level={5} style={{ margin: '0 0 8px' }}>{t('assetDetail.riskListTitle')}</Typography.Title>
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

      {/* key={asset.id}:切资产时重挂载 AssetEditor(含 ContentArea),使其 Segmented view state
          和编辑态(editing/draft/preview)回默认,避免上一资产的草稿/视图泄漏到新资产。 */}
      <AssetEditor key={asset.id} asset={asset} highlights={highlights} />

      {/* 安全检查:只配检测器,scope=asset-id 按 ID 单扫。getContainer={false} 渲染进 Drawer DOM 树,
          修复原 RescanModal(根级)被 AssetDrawer 同 z-index mask 盖住的"弹框在抽屉下面"问题。 */}
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
