import { Descriptions, Tag, Typography, Empty } from 'antd'
import { useTranslation } from 'react-i18next'
import type { Asset } from '../types'

// 能力面板:按 asset.type 分发,结构化展示"能干什么/能访问什么"。
// 字段来自后端已解析的 asset.fields(skill 的 allowed-tools、hook 的 event/command、mcp 的 command/env 等)。
// memory 走 fields.outline(标题大纲,Task 9);script 无 fields,展示语言/行数。
export function CapabilityPanel({ asset }: { asset: Asset }) {
  const { t } = useTranslation()
  // Asset.fields 类型是 Record<string, unknown>;buildItems 内部按字符串/数组/对象取值,转 any 简化索引访问。
  const f = (asset.fields ?? {}) as Record<string, any>
  const items = buildItems(asset.type, f, asset, t)
  if (items.length === 0) {
    return <Empty description={t('capability.noFields')} image={Empty.PRESENTED_IMAGE_SIMPLE} />
  }
  return (
    <Descriptions column={1} size="small" bordered={false}>
      {items.map((it, idx) => (
        <Descriptions.Item key={idx} label={it.label}>
          {it.render}
        </Descriptions.Item>
      ))}
    </Descriptions>
  )
}

function chipList(values: string[] | undefined): React.ReactNode {
  if (!values || values.length === 0) return <Typography.Text type="secondary">—</Typography.Text>
  return values.map((v) => <Tag key={v} style={{ marginInlineEnd: 4 }}>{v}</Tag>)
}

type Item = { label: React.ReactNode; render: React.ReactNode }

function buildItems(type: string, f: Record<string, any>, asset: Asset, t: any): Item[] {
  switch (type) {
    case 'skill':
    case 'command':
    case 'agent': {
      const tools = typeof f['allowed-tools'] === 'string' ? f['allowed-tools'].split(',').map((s: string) => s.trim()).filter(Boolean) : f['allowed-tools']
      return [
        { label: t('capability.allowedTools'), render: tools && tools.length ? chipList(tools) : <Typography.Text type="warning">{t('capability.allTools')}</Typography.Text> },
        ...(f.description ? [{ label: t('capability.description'), render: <Typography.Text type="secondary">{f.description}</Typography.Text> }] : []),
      ]
    }
    case 'hook':
      return [
        { label: t('capability.hookEvent'), render: <Tag style={{ background: 'var(--cat-1)', color: 'var(--badge-text)', border: 'none' }}>{f.event ?? '—'}</Tag> },
        { label: t('capability.hookMatcher'), render: <Typography.Text code>{f.matcher ?? '—'}</Typography.Text> },
        { label: t('capability.hookCommand'), render: <Typography.Text code>{f.command ?? '—'}</Typography.Text> },
      ]
    case 'mcp_server': {
      const envKeys = f.env ? Object.keys(f.env) : []
      return [
        { label: t('capability.mcpTransport'), render: <Tag>{f.transport ?? 'stdio'}</Tag> },
        ...(f.command ? [{ label: t('capability.mcpCommand'), render: <Typography.Text code>{f.command}</Typography.Text> }] : []),
        ...(f.args ? [{ label: t('capability.mcpArgs'), render: chipList(f.args) }] : []),
        ...(f.url ? [{ label: 'URL', render: <Typography.Text code>{f.url}</Typography.Text> }] : []),
        { label: t('capability.mcpEnv'), render: envKeys.length ? chipList(envKeys) : <Typography.Text type="secondary">—</Typography.Text> },
        { label: t('capability.mcpTools'), render: <Typography.Text type="secondary">{t('capability.mcpToolsUnparsed')}</Typography.Text> },
      ]
    }
    case 'settings': {
      const envKeys = f.env ? Object.keys(f.env) : []
      return [
        ...(envKeys.length ? [{ label: t('capability.envKeys'), render: chipList(envKeys) }] : []),
        ...(f.model ? [{ label: t('capability.model'), render: <Tag>{f.model}</Tag> }] : []),
        ...(f.skip_dangerous ? [{ label: t('capability.skipDangerous'), render: <Tag style={{ background: 'var(--sev-high-solid)', color: 'var(--badge-text)', border: 'none' }}>{t('capability.enabled')}</Tag> }] : []),
      ]
    }
    case 'permissions':
      return [
        { label: t('capability.allow'), render: chipList(f.allow) },
        { label: t('capability.deny'), render: f.deny?.length ? chipList(f.deny) : <Typography.Text type="secondary">—</Typography.Text> },
        { label: t('capability.ask'), render: f.ask?.length ? chipList(f.ask) : <Typography.Text type="secondary">—</Typography.Text> },
      ]
    case 'plugin':
      return [
        { label: t('capability.pluginVersion'), render: <Tag>{f.version ?? '—'}</Tag> },
        ...(f.author ? [{ label: t('capability.pluginAuthor'), render: f.author }] : []),
        ...(f.marketplace ? [{ label: t('capability.pluginMarket'), render: <Typography.Text code>{f.marketplace}</Typography.Text> }] : []),
      ]
    case 'memory': {
      const outline: any[] = f.outline ?? []
      if (outline.length === 0) return []
      return [
        { label: t('capability.memoryOutline'), render: (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {outline.map((o, i) => (
              <span key={i} style={{ paddingInlineStart: (o.level - 1) * 12, fontFamily: 'var(--font-mono)', fontSize: 12 }}>
                {'#'.repeat(o.level)} {o.title} <Typography.Text type="secondary" style={{ fontSize: 10 }}>: {o.line}</Typography.Text>
              </span>
            ))}
          </div>
        ) },
      ]
    }
    case 'script':
      return [
        { label: t('capability.scriptLang'), render: <Tag>{langFromExt(asset.name)}</Tag> },
        { label: t('capability.scriptLines'), render: <Typography.Text code>{asset.content?.split('\n').length ?? 0}</Typography.Text> },
      ]
    case 'credential':
      return [
        { label: t('capability.credKind'), render: <Tag style={{ background: 'var(--sev-critical-solid)', color: 'var(--badge-text)', border: 'none' }}>{f.kind ?? '—'}</Tag> },
        { label: t('capability.credPath'), render: <Typography.Text code>{f.path ?? '—'}</Typography.Text> },
      ]
    case 'keybinding': {
      const pairs = Object.entries(f)
      return pairs.length ? pairs.map(([k, v]) => ({ label: <Typography.Text code>{k}</Typography.Text> as React.ReactNode, render: String(v) })) : []
    }
    default:
      return []
  }
}

function langFromExt(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase()
  return ({ sh: 'shell', bash: 'shell', py: 'python', js: 'javascript', ts: 'typescript', go: 'go', rb: 'ruby' } as Record<string, string>)[ext ?? ''] ?? ext ?? '—'
}
