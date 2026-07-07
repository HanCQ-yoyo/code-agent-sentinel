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

const sevFill: Record<string, string> = {
  'sev-critical': 'var(--sev-critical)',
  'sev-high': 'var(--sev-high)',
  'sev-medium': 'var(--sev-medium)',
  'sev-low': 'var(--sev-low)',
}

const scopeColor: Record<string, string> = {
  'scope-global': 'var(--scope-global)',
  'scope-project': 'var(--scope-project)',
  'scope-managed': 'var(--scope-managed)',
  'scope-plugin': 'var(--scope-plugin)',
}

export function Badge({ tone, children }: { tone: BadgeTone; children: ReactNode }) {
  const base: React.CSSProperties = {
    fontFamily: 'var(--font-mono, "JetBrains Mono", ui-monospace, monospace)',
    fontSize: 12,
    marginInlineEnd: 0,
  }

  if (tone in sevFill) {
    // sev 填充:critical 白字,其余固定深墨 #1a1a1a(对比度硬规则)
    const ink = tone === 'sev-critical' ? '#fff' : '#1a1a1a'
    return (
      <Tag style={{ ...base, background: sevFill[tone], color: ink, border: 'none' }}>
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
