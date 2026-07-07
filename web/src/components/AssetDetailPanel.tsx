import { Card, Descriptions, Typography, Alert } from 'antd'
import type { Asset } from '../types'
import { Badge, type BadgeTone } from './Badge'
import { relativeClaudePath } from '../lib/path'

export function AssetDetailPanel({ asset }: { asset: Asset }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '100%' }}>
      <div>
        <h2 data-testid="asset-detail-name" style={{ color: 'var(--text)', margin: '0 0 8px' }}>{asset.name}</h2>
        <div style={{ display: 'flex', gap: 8 }}>
          <Badge tone="neutral">{asset.type}</Badge>
          <Badge tone={`scope-${asset.scope}` as BadgeTone}>{asset.scope}</Badge>
        </div>
      </div>

      {asset.parse_error ? (
        <Alert type="error" message="解析失败" description={asset.parse_error} showIcon />
      ) : null}

      <Descriptions size="small" column={2} bordered>
        <Descriptions.Item label="路径" span={2}>
          <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{relativeClaudePath(asset.source_path)}</Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label="hash">
          <Typography.Text code copyable style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{asset.hash}</Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label="修改时间">
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{asset.mtime ?? '--'}</span>
        </Descriptions.Item>
      </Descriptions>

      {asset.fields && Object.keys(asset.fields).length > 0 ? (
        <Card size="small" title="解析字段" styles={{ body: { padding: 0 } }}>
          <pre style={{ margin: 0, padding: 12, maxHeight: 320, overflow: 'auto', fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text)' }}>
{JSON.stringify(asset.fields, null, 2)}
          </pre>
        </Card>
      ) : null}

      {asset.content ? (
        <Card size="small" title="文件内容" style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }} styles={{ body: { flex: 1, padding: 0, overflow: 'hidden' } }}>
          <pre style={{ margin: 0, padding: 12, height: '100%', overflow: 'auto', fontFamily: 'var(--font-mono)', fontSize: 12.5, color: 'var(--text)' }}>
{asset.content}
          </pre>
        </Card>
      ) : null}
    </div>
  )
}
