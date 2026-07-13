import { Card, List, Badge, Typography } from 'antd'
import type { DetectorStatus } from '../types'

const names: Record<string, string> = {
  // P3 声明式规则引擎(合并基线 + 注入 + skill 规则,统一 RulesDetector)。
  rules: '声明式规则引擎',
  // 旧检测器 ID 保留:历史扫描记录可能携带 baseline/content.injection,需能正常显示中文名。
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
