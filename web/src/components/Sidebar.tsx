import { Layout, Menu } from 'antd'
import {
  DashboardOutlined,
  AppstoreOutlined,
  WarningOutlined,
  ClockCircleOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { useLocation, useNavigate } from 'react-router-dom'

const { Sider } = Layout

const items = [
  { key: '/dashboard', icon: <DashboardOutlined />, label: '看板' },
  { key: '/assets', icon: <AppstoreOutlined />, label: '资产' },
  { key: '/findings', icon: <WarningOutlined />, label: '发现' },
  { key: '/history', icon: <ClockCircleOutlined />, label: '历史' },
  { key: '/settings', icon: <SettingOutlined />, label: '设置' },
]

export function Sidebar() {
  const nav = useNavigate()
  const loc = useLocation()
  // '/' 等同 '/dashboard'
  const selected = loc.pathname === '/' ? '/dashboard' : loc.pathname
  return (
    <Sider width={208} breakpoint="lg" collapsedWidth={0} style={{ background: 'var(--bg-card)' }}>
      <div style={{ color: 'var(--accent)', fontWeight: 700, padding: '20px 24px', fontSize: 18 }}>
        Sentinel
      </div>
      <Menu
        mode="inline"
        selectedKeys={[selected]}
        onClick={({ key }) => nav(key)}
        items={items}
        style={{ background: 'var(--bg-card)', borderInlineEnd: 'none' }}
      />
    </Sider>
  )
}
