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
    // 垂直居中根因:antd Card 根非 flex,旧 body 设 height:'100%' 解析为整卡高(含标题),
    // grid 的 alignContent:'center' 在超出实际可用高的容器里居中 → 视觉偏下。
    // 修复:根设 flex column,body 用 flex:1 占剩余高(减标题),alignContent:'center'
    // 在剩余区内真正垂直居中瓦片。minHeight:0 允许 flex 子项收缩。
    <Card
      title="资产统计"
      style={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column' }}
      styles={{ body: { flex: 1, minHeight: 0 } }}
    >
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(110px, 1fr))', gap: 12, height: '100%', alignContent: 'center', justifyItems: 'center', textAlign: 'center' }}>
        {tiles.map((s) => (
          <Statistic key={s.type} title={s.label} value={counts[s.type] ?? 0} prefix={s.icon} valueStyle={{ fontFamily: 'var(--font-mono)' }} />
        ))}
      </div>
    </Card>
  )
}
