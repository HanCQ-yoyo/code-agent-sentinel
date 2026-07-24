import type { ReactNode } from 'react'
import { Tag } from 'antd'

export type BadgeTone =
  | 'neutral'
  | 'accent'
  | 'scope-global'
  | 'scope-project'
  | 'scope-managed'
  | 'scope-plugin'
  | 'sev-critical'
  | 'sev-high'
  | 'sev-medium'
  | 'sev-low'
  | 'sev-info'

const sevFill: Record<string, string> = {
  'sev-critical': 'var(--sev-critical-solid)',
  'sev-high': 'var(--sev-high-solid)',
  'sev-medium': 'var(--sev-medium-solid)',
  'sev-low': 'var(--sev-low-solid)',
  'sev-info': 'var(--sev-info-solid)',
}

const scopeColor: Record<string, string> = {
  'scope-global': 'var(--scope-global)',
  'scope-project': 'var(--scope-project)',
  'scope-managed': 'var(--scope-managed)',
  'scope-plugin': 'var(--scope-plugin)',
}

export function Badge({ tone, children }: { tone: BadgeTone; children: ReactNode }) {
  const base: React.CSSProperties = {
    fontFamily: 'var(--font-sans)',
    fontSize: 'var(--fs-sm)',
    marginInlineEnd: 0,
    lineHeight: '20px',
    padding: '0 8px',
    borderRadius: 4,
  }

  if (tone in sevFill) {
    // 级别标签统一:实色填充 + 白字(全级别一致,-solid token 已调深保证白字对比度 ≥ AA)。
    // 文字走 --badge-text token(design.md:取代 #fff 硬编码)。
    return (
      <Tag style={{ ...base, background: sevFill[tone], color: 'var(--badge-text)', border: 'none', fontWeight: 600 }}>
        {children}
      </Tag>
    )
  }

  if (tone in scopeColor) {
    // scope:左色块边框 + 中性面
    return (
      <Tag style={{ ...base, background: 'var(--bg-card)', color: 'var(--text)', borderLeft: `3px solid ${scopeColor[tone]}`, borderWidth: '1px 1px 1px 3px', borderLeftColor: scopeColor[tone], borderColor: 'var(--bg-border)' }}>
        {children}
      </Tag>
    )
  }

  if (tone === 'accent') {
    return <Tag style={{ ...base, color: 'var(--accent)', borderColor: 'var(--accent)', background: 'transparent' }}>{children}</Tag>
  }

  // neutral
  return <Tag style={{ ...base, color: 'var(--text-muted)', borderColor: 'var(--bg-border)', background: 'transparent' }}>{children}</Tag>
}
