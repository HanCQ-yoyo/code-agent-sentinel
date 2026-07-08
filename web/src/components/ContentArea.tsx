import { lazy, Suspense, useState } from 'react'
import { Card, Segmented, Spin, Empty } from 'antd'
import type { Asset } from '../types'
import { MarkdownPreview } from './MarkdownPreview'
import { langByExt } from '../lib/monaco-lang'

// MonacoViewer 懒加载:markdown 默认预览不触发 monaco chunk 加载,
// 只有切源码 / 开 script / 开 structured 才拉 ~5MB chunk(本地 embed,首次见短暂 Spin)。
const MonacoViewer = lazy(() => import('./MonacoViewer'))

// markdown 类资产:content 是 md 正文(frontmatter 已剥离),默认 react-markdown 预览,
// Segmented[预览|源码] 切 Monaco md 高亮。
const MARKDOWN_TYPES = new Set(['memory', 'skill', 'command', 'agent'])

// structured 类资产:无原始 content,展示 fields(settings 有 fields.raw 原始 JSON;
// 其余序列化 JSON.stringify)。Monaco json 高亮。
const STRUCTURED_TYPES = new Set(['settings', 'permissions', 'mcp_server', 'hook', 'keybinding', 'plugin'])

// ContentArea 按 asset.type 分派渲染:
// - markdown → Segmented[预览|源码],默认预览
// - script → Monaco(langByExt(source_path))
// - structured → Monaco json(fields.raw ?? JSON.stringify(fields))
// - 空 content+空 fields → antd Empty
// theme 由 AssetDetailPanel 从 useTheme() 取后透传给 Monaco。
export function ContentArea({ asset, theme }: { asset: Asset; theme: 'light' | 'dark' }) {
  const [view, setView] = useState<'preview' | 'source'>('preview')

  const isMarkdown = MARKDOWN_TYPES.has(asset.type)
  const isScript = asset.type === 'script'
  const isStructured = STRUCTURED_TYPES.has(asset.type)

  // markdown:有 content 才渲染预览/源码
  if (isMarkdown && asset.content) {
    return (
      <Card
        size="small"
        title="内容"
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column' } }}
        extra={
          <Segmented
            size="small"
            value={view}
            onChange={(v) => setView(v as 'preview' | 'source')}
            options={[{ value: 'preview', label: '预览' }, { value: 'source', label: '源码' }]}
          />
        }
      >
        {view === 'preview' ? (
          <div style={{ padding: 12, flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column' }}>
            <MarkdownPreview content={asset.content} />
          </div>
        ) : (
          <div style={{ padding: 12 }}>
            <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
              <MonacoViewer value={asset.content} language="markdown" theme={theme} />
            </Suspense>
          </div>
        )}
      </Card>
    )
  }

  // script:Monaco(按扩展名选语言)
  if (isScript && asset.content) {
    return (
      <Card
        size="small"
        title="内容"
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
      >
        <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
          <MonacoViewer value={asset.content} language={langByExt(asset.source_path)} theme={theme} />
        </Suspense>
      </Card>
    )
  }

  // structured:settings 有 fields.raw(原始 JSON),其余序列化 fields
  if (isStructured) {
    const raw = (asset.fields as Record<string, unknown> | undefined)?.raw
    const value = typeof raw === 'string'
      ? raw
      : JSON.stringify(asset.fields ?? {}, null, 2)
    if (value === '{}' || value === '') {
      return <Card size="small" title="内容"><Empty description="该资产无解析字段" /></Card>
    }
    return (
      <Card
        size="small"
        title="内容"
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
      >
        <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
          <MonacoViewer value={value} language="json" theme={theme} />
        </Suspense>
      </Card>
    )
  }

  // 空 content + 空 fields(或 type 不在已知集合)
  if (!asset.content && (!asset.fields || Object.keys(asset.fields).length === 0)) {
    return <Card size="small" title="内容"><Empty description="该资产无内容" /></Card>
  }

  // 兜底:有 content 但 type 非 markdown/script(罕见),按 plaintext Monaco
  if (asset.content) {
    return (
      <Card
        size="small"
        title="内容"
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
      >
        <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
          <MonacoViewer value={asset.content} language="plaintext" theme={theme} />
        </Suspense>
      </Card>
    )
  }

  // 兜底:有 fields 但 type 非 structured
  const value = JSON.stringify(asset.fields ?? {}, null, 2)
  return (
    <Card
      size="small"
      title="内容"
      style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
      styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
    >
      <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
        <MonacoViewer value={value} language="json" theme={theme} />
      </Suspense>
    </Card>
  )
}
