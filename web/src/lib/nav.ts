// 导航标签单一来源:侧栏 Menu label 与内容区顶栏面包屑分段均引用,防漂移。
// route key 不变(仅 label 文案变化)。
export interface NavItem {
  path: string
  label: string
}

export const navItems: NavItem[] = [
  { path: '/dashboard', label: 'Dashboard' },
  { path: '/assets', label: '资产发现' },
  { path: '/findings', label: '风险管理' },
  { path: '/history', label: '历史扫描' },
  { path: '/settings', label: '系统设置' },
]

// route → label(侧栏 label 查找)。
export const navLabels: Record<string, string> = Object.fromEntries(
  navItems.map((i) => [i.path, i.label]),
)
