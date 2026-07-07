import clsx from 'clsx'
import type { CSSProperties, ReactNode } from 'react'

export type BadgeTone =
  | 'neutral' | 'accent'
  | 'scope-global' | 'scope-project' | 'scope-managed' | 'scope-plugin'
  | 'sev-critical' | 'sev-high' | 'sev-medium' | 'sev-low'

// Badge 是通用标签。对比度约定:sev 色只作标记色(填色背景+白字),
// scope 色用低饱和填色+主文字,neutral/accent 用透明底+边框+文字。
export function Badge({ tone, children }: { tone: BadgeTone; children: ReactNode }) {
  const cls: Record<BadgeTone, string> = {
    neutral: 'border border-bg-border text-text-muted bg-transparent',
    accent: 'border border-accent/40 text-accent bg-transparent',
    'scope-global': 'text-text bg-bg-card',
    'scope-project': 'text-text bg-bg-card',
    'scope-managed': 'text-text bg-bg-card',
    'scope-plugin': 'text-text bg-bg-card',
    'sev-critical': 'text-white',
    'sev-high': 'text-white',
    'sev-medium': 'text-white',
    'sev-low': 'text-white',
  }
  // scope 用各自色相作左边框/底色提示;sev 用填色背景
  const style: Record<BadgeTone, CSSProperties> = {} as Record<BadgeTone, CSSProperties>
  const scopeStyle = (v: string): CSSProperties => ({ borderLeft: `3px solid var(${v})` })
  if (tone === 'scope-global') style['scope-global'] = scopeStyle('--scope-global')
  if (tone === 'scope-project') style['scope-project'] = scopeStyle('--scope-project')
  if (tone === 'scope-managed') style['scope-managed'] = scopeStyle('--scope-managed')
  if (tone === 'scope-plugin') style['scope-plugin'] = scopeStyle('--scope-plugin')
  if (tone.startsWith('sev-')) {
    style[tone] = { background: `var(--${tone})` }
  }
  return (
    <span className={clsx('inline-block px-2 py-0.5 rounded text-xs font-mono whitespace-nowrap', cls[tone])} style={style[tone]}>
      {children}
    </span>
  )
}
