import { Card, List, Badge, Typography } from 'antd'
import type { DetectorStatus } from '../types'

const names: Record<string, string> = {
  baseline: '基线',
  'content.injection': '提示注入',
  secret: '密钥(gitleaks)',
  dep: '依赖',
}

export function DetectorStatusList({ list, bare = false }: { list: DetectorStatus[]; bare?: boolean }) {
  const body = (
    <List
      dataSource={list}
      renderItem={(d) => (
        <List.Item style={{ border: 'none', padding: '6px 0' }}>
          <List.Item.Meta
            avatar={<Badge status={d.available ? 'success' : 'error'} />}
            title={<span style={{ color: 'var(--text)' }}>{names[d.id] ?? d.id}</span>}
            description={
              !d.available && d.reason ? (
                <Typography.Text type="secondary" style={{ fontSize: 12 }} title={d.reason}>{d.reason}</Typography.Text>
              ) : null
            }
          />
        </List.Item>
      )}
    />
  )
  if (bare) return <div style={{ paddingTop: 16, borderTop: '1px solid var(--bg-border)' }}>{body}</div>
  return <Card title="检测器状态">{body}</Card>
}
