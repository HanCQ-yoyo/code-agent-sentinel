import { Layout, Menu } from 'antd'
import {
  DashboardOutlined,
  AppstoreOutlined,
  WarningOutlined,
  ClockCircleOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { navItems } from '../lib/nav'

const { Sider } = Layout

const iconByPath: Record<string, React.ReactNode> = {
  '/dashboard': <DashboardOutlined />,
  '/assets': <AppstoreOutlined />,
  '/findings': <WarningOutlined />,
  '/history': <ClockCircleOutlined />,
  '/settings': <SettingOutlined />,
}

// navItems.label 存 i18n key,渲染时 t() 翻译。
const useNavItems = () => {
  const { t } = useTranslation()
  return navItems.map((i) => ({ key: i.path, icon: iconByPath[i.path], label: t(i.label) }))
}

export function Sidebar() {
  const nav = useNavigate()
  const loc = useLocation()
  const items = useNavItems()
  // '/' 等同 '/dashboard'
  const selected = loc.pathname === '/' ? '/dashboard' : loc.pathname
  return (
    <Sider width={208} breakpoint="lg" collapsedWidth={0} style={{ background: 'var(--bg-card)' }}>
      {/* 品牌:落侧边栏最上方 = 平台最左上角 */}
      <div
        data-testid="brand"
        style={{ display: 'flex', alignItems: 'center', padding: '20px 24px' }}
      >
        <span style={{ color: 'var(--accent)', fontWeight: 700, fontSize: 15, lineHeight: '20px' }}>
          Code Agent Sentinel
        </span>
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
