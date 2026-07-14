import { Card, Statistic } from 'antd'
import {
  ToolOutlined, CodeOutlined, RobotOutlined, AppstoreOutlined,
  SnippetsOutlined, ThunderboltOutlined, ClusterOutlined, FileTextOutlined,
} from '@ant-design/icons'

// 资产类型 → 显示名 + 图标 + 顺序。
const specs: { type: string; label: string; icon: React.ReactNode }[] = [
  { type: 'skill', label: '技能', icon: <ToolOutlined /> },
  { type: 'command', label: '命令', icon: <CodeOutlined /> },
  { type: 'agent', label: 'Agent', icon: <RobotOutlined /> },
  { type: 'plugin', label: '插件', icon: <AppstoreOutlined /> },
  { type: 'script', label: '脚本', icon: <SnippetsOutlined /> },
  { type: 'hook', label: 'Hook', icon: <ThunderboltOutlined /> },
  { type: 'mcp_server', label: 'MCP', icon: <ClusterOutlined /> },
  { type: 'memory', label: '记忆', icon: <FileTextOutlined /> },
]

export function AssetStatTiles({ counts }: { counts: Record<string, number> }) {
  const present = specs.filter((s) => (counts[s.type] ?? 0) > 0)
  const tiles = present.length > 0 ? present : specs
  return (
    <Card title="资产统计">
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(110px, 1fr))', gap: 12 }}>
        {tiles.map((s) => (
          <Statistic key={s.type} title={s.label} value={counts[s.type] ?? 0} prefix={s.icon} />
        ))}
      </div>
    </Card>
  )
}
