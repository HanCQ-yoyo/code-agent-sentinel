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
    <Sider width={208} breakpoint="lg" collapsedWidth={0} style={{ background: 'var(--color-paper-2)' }}>
      {/* 品牌:落侧边栏最上方 = 平台最左上角。Inter Tight 700 + accent + 紧字距(design.md CTA voice)。 */}
      <div
        data-testid="brand"
        style={{ display: 'flex', alignItems: 'center', padding: 'var(--space-xl) var(--space-2xl)' }}
      >
        <span style={{ color: 'var(--color-accent)', fontWeight: 700, fontSize: 'var(--fs-md)', lineHeight: '20px', letterSpacing: '-0.01em' }}>
          Code Agent Sentinel
        </span>
      </div>
      <Menu
        mode="inline"
        selectedKeys={[selected]}
        onClick={({ key }) => nav(key)}
        items={items}
        style={{ background: 'var(--color-paper-2)', borderInlineEnd: 'none' }}
      />
    </Sider>
  )
}
