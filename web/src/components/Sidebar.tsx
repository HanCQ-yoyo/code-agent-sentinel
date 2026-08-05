import type { MenuProps } from 'antd'
import { Layout, Menu, Switch } from 'antd'
import {
  DashboardOutlined,
  AppstoreOutlined,
  WarningOutlined,
  ClockCircleOutlined,
  BlockOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { navItems, type NavItem } from '../lib/nav'
import { useTheme } from '../theme'

const { Sider } = Layout

const iconByPath: Record<string, React.ReactNode> = {
  '/dashboard': <DashboardOutlined />,
  '/assets': <AppstoreOutlined />,
  '/findings': <WarningOutlined />,
  '/history': <ClockCircleOutlined />,
  '/intercept': <BlockOutlined />,
  '/settings': <SettingOutlined />,
}

// 将 NavItem[] 投影为 antd Menu items(递归 children)
type AntMenuItem = NonNullable<MenuProps['items']>[number]
function toMenuItem(i: NavItem): AntMenuItem {
  const { t } = useTranslation()
  const base = { key: i.path, icon: iconByPath[i.path], label: t(i.label) }
  if (i.children?.length) {
    return { ...base, icon: <SettingOutlined />, children: i.children.map(toMenuItem) }
  }
  return base
}

function useNavItems() {
  return navItems.map(toMenuItem)
}

export function Sidebar() {
  const nav = useNavigate()
  const loc = useLocation()
  const { theme, toggle } = useTheme()
  const { t } = useTranslation()
  const items = useNavItems()

  const selected = loc.pathname === '/' ? '/dashboard' : loc.pathname

  return (
    <Sider width={208} breakpoint="lg" collapsedWidth={0} style={{ background: 'var(--color-paper-2)', height: '100vh', position: 'sticky', top: 0, overflow: 'hidden' }}>
      <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        {/* 品牌 */}
        <div data-testid="brand" style={{ flexShrink: 0, display: 'flex', alignItems: 'center', padding: 'var(--space-xl) var(--space-2xl)' }}>
          <span style={{ color: 'var(--color-accent)', fontWeight: 700, fontSize: 'var(--fs-md)', lineHeight: '20px', letterSpacing: '-0.01em' }}>
            Code Agent Sentinel
          </span>
        </div>

        {/* 菜单: flex:1 撑满中间空白 */}
        <Menu
          mode="inline"
          selectedKeys={[selected]}
          defaultOpenKeys={selected.startsWith('/settings') ? ['/settings'] : undefined}
          onClick={({ key }) => nav(key)}
          items={items}
          style={{ background: 'var(--color-paper-2)', borderInlineEnd: 'none', flex: 1, overflow: 'auto' }}
        />

        {/* 主题切换: 贴底, 分隔线与菜单独立 */}
        <div style={{ flexShrink: 0, borderTop: '1px solid var(--color-rule)', padding: 'var(--space-md) var(--space-xl)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span style={{ fontSize: 'var(--fs-sm)', color: 'var(--color-muted)' }}>{t('topbar.theme')}</span>
          <Switch
            className="theme-switch"
            size="small"
            checked={theme === 'dark'}
            onChange={toggle}
            checkedChildren={t('topbar.dark')}
            unCheckedChildren={t('topbar.light')}
            aria-label={t('topbar.theme')}
          />
        </div>
      </div>
    </Sider>
  )
}
