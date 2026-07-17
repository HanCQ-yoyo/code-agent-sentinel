import { Descriptions, Typography, Alert } from 'antd'
import { useTranslation } from 'react-i18next'
import type { Asset } from '../types'
import { Badge, type BadgeTone } from './Badge'
import { relativeClaudePath } from '../lib/path'
import { AssetEditor } from './AssetEditor'

// AssetDetailPanel:资产详情。三消费方(Assets 列表抽屉 50% / 树右栏 480px sticky / /assets/:id 全页)
// 共用此组件,签名 { asset } 不变。阶段 C 重排:
//  1. frontmatter 上浮:markdown 资产的 description 取自 fields.description,上移到 header 副标题(人语位置)。
//  2. 二合一:旧「解析字段 Card + 文件内容 Card」→ 单一 ContentArea(structured 字段即内容,二合一)。
//  3. 内容撑满:ContentArea flex:1。
// header h2 保留 data-testid="asset-detail-name"(e2e 钩子,阶段 A 硬规则延续)。
export function AssetDetailPanel({ asset }: { asset: Asset }) {
  const { t } = useTranslation()
  const description = (asset.fields as Record<string, unknown> | undefined)?.description
  const isMarkdown = ['memory', 'skill', 'command', 'agent'].includes(asset.type)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '100%' }}>
      <div>
        <h2 data-testid="asset-detail-name" style={{ color: 'var(--text)', margin: '0 0 4px' }}>{asset.name}</h2>
        {isMarkdown && typeof description === 'string' && description ? (
          <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>{description}</Typography.Text>
        ) : null}
        <div style={{ display: 'flex', gap: 8 }}>
          <Badge tone="neutral">{asset.type}</Badge>
          <Badge tone={`scope-${asset.scope}` as BadgeTone}>{asset.scope}</Badge>
        </div>
      </div>

      {asset.parse_error ? (
        <Alert type="error" message={t('assetDetail.parseError')} description={asset.parse_error} showIcon />
      ) : null}

      <Descriptions size="small" column={2} bordered>
        <Descriptions.Item label={t('assetDetail.path')} span={2}>
          <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{relativeClaudePath(asset.source_path)}</Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label={t('assetDetail.hash')}>
          <Typography.Text code copyable style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{asset.hash}</Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label={t('assetDetail.mtime')}>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{asset.mtime ?? '--'}</span>
        </Descriptions.Item>
      </Descriptions>

      {/* key={asset.id}:切资产时重挂载 AssetEditor(含 ContentArea),使其 Segmented view state
          和编辑态(editing/draft/preview)回默认,避免上一资产的草稿/视图泄漏到新资产。 */}
      <AssetEditor key={asset.id} asset={asset} />
    </div>
  )
}
