// 导航标签单一来源:侧栏 Menu label 与内容区顶栏面包屑分段均引用,防漂移。
// route key 不变(仅 label 文案变化)。
// label 存 i18n key(如 'nav.dashboard'),由 Sidebar/TopBar 渲染时 t(label) 翻译。
export interface NavItem {
  path: string
  label: string
  children?: NavItem[]  // 新增: 侧栏嵌套子菜单
}

// 设置页 4 子项
export const SETTINGS_SUB: NavItem[] = [
  { path: '/settings/general', label: 'nav.sub.general' },
  { path: '/settings/security', label: 'nav.sub.securityConfig' },
  { path: '/settings/scan-rules', label: 'nav.sub.scanRules' },
  { path: '/settings/intercept-rules', label: 'nav.sub.interceptRules' },
]

export const navItems: NavItem[] = [
  { path: '/dashboard', label: 'nav.dashboard' },
  { path: '/assets', label: 'nav.assets' },
  { path: '/findings', label: 'nav.findings' },
  { path: '/history', label: 'nav.history' },
  { path: '/intercept', label: 'nav.intercept' },
  { path: '/settings', label: 'nav.settings', children: SETTINGS_SUB },
]

// route → i18n key(侧栏 label 查找;消费者需 t() 翻译)。
export const navLabels: Record<string, string> = Object.fromEntries(
  [...navItems, ...SETTINGS_SUB].map((i) => [i.path, i.label]),
)
