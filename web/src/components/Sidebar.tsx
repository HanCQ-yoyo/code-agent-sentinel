import { NavLink } from 'react-router-dom'
import clsx from 'clsx'

const items = [
  { to: '/dashboard', label: '看板', icon: '◆' },
  { to: '/assets', label: '资产', icon: '▦' },
  { to: '/findings', label: '发现', icon: '⚠' },
  { to: '/history', label: '历史', icon: '◷' },
  { to: '/settings', label: '设置', icon: '⚙' },
]

export function Sidebar() {
  return (
    <nav role="navigation" className="w-56 shrink-0 border-r border-bg-border bg-bg-card p-3 space-y-1">
      <div className="px-3 py-2 mb-2 flex items-center gap-2">
        <span className="text-accent text-lg">◆</span>
        <span className="font-semibold tracking-wide">Sentinel</span>
      </div>
      {items.map(it => (
        <NavLink
          key={it.to}
          to={it.to}
          className={({ isActive }) =>
            clsx(
              'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
              isActive ? 'bg-accent/15 text-accent' : 'text-text-muted hover:text-text hover:bg-bg-border'
            )
          }
        >
          <span className="w-4 text-center">{it.icon}</span>
          {it.label}
        </NavLink>
      ))}
    </nav>
  )
}
