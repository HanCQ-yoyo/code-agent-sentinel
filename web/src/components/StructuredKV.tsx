import type { ReactNode } from 'react'
import { Collapse, Tag, Typography, Empty } from 'antd'

// StructuredKV:递归渲染任意值(对象/数组/基本类型)。
// 对象 → Collapse(键作 panel header,值递归);数组 → Tag 列表(对象项递归);
// 基本类型 → Text(string 且像路径/ID/hash → mono,否则正文);空 → secondary "无"。
// data-testid="structured-kv" 钩子(供 e2e)。
export function StructuredKV({ value }: { value: unknown }) {
  return <div data-testid="structured-kv">{renderValue(value)}</div>
}

function renderValue(value: unknown, keyHint?: string): ReactNode {
  if (value === null || value === undefined) {
    return <Typography.Text type="secondary">无</Typography.Text>
  }
  if (Array.isArray(value)) {
    if (value.length === 0) return <Typography.Text type="secondary">无</Typography.Text>
    return (
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
        {value.map((v, i) =>
          typeof v === 'object' && v !== null ? (
            <div key={i} style={{ width: '100%' }}>{renderValue(v)}</div>
          ) : (
            <Tag key={i}>{String(v)}</Tag>
          )
        )}
      </div>
    )
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
    if (entries.length === 0) return <Typography.Text type="secondary">无</Typography.Text>
    return (
      <Collapse size="small" ghost items={entries.map(([k, v]) => ({
        key: k,
        label: <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{k}</span>,
        children: renderValue(v, k),
      }))} />
    )
  }
  // 基本类型:string 判定是否机器语(路径含 /、长 hash、纯 ID)
  const s = String(value)
  const isMachine = keyHint === 'path' || keyHint === 'hash' || keyHint === 'id' || keyHint === 'mtime' || s.includes('/') || /^[0-9a-f]{8,}$/i.test(s)
  return isMachine ? (
    <Typography.Text style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{s}</Typography.Text>
  ) : (
    <Typography.Text>{s}</Typography.Text>
  )
}
