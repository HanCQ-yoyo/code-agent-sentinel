import { lazy, Suspense, useState } from 'react'
import { Card, Segmented, Spin, Empty } from 'antd'
import { useTranslation } from 'react-i18next'
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

// editableText:返回资产的可编辑文本(与只读态 Monaco 渲染一致)。
// markdown/script → asset.content;structured → fields.raw ?? JSON.stringify(fields);兜底 → content ?? JSON.stringify(fields)。
// AssetEditor 用此初始化 draft,确保编辑起点 = 用户在只读态所见文本。
// 关键:structured 资产无 asset.content(configengine 仅给 memory/script 设 content),
// 必须从 fields.raw 取原始文本,否则 draft 为空 → 编辑 silently 无效。
export function editableText(asset: Asset): string {
  const isMarkdown = MARKDOWN_TYPES.has(asset.type)
  const isScript = asset.type === 'script'
  const isStructured = STRUCTURED_TYPES.has(asset.type)
  if (isMarkdown || isScript) return asset.content ?? ''
  if (isStructured) {
    const raw = (asset.fields as Record<string, unknown> | undefined)?.raw
    return typeof raw === 'string' ? raw : JSON.stringify(asset.fields ?? {}, null, 2)
  }
  return asset.content ?? JSON.stringify(asset.fields ?? {}, null, 2)
}

// ContentArea 按 asset.type 分派渲染:
// - markdown → Segmented[预览|源码],默认预览(编辑态默认源码)
// - script → Monaco(langByExt(source_path))
// - structured → Monaco json(fields.raw ?? JSON.stringify(fields))
// - 空 content+空 fields → antd Empty
// theme 由调用方(AssetEditor)从 useTheme() 取后透传给 Monaco。
// readOnly/onChange:P2 编辑模式透传给 MonacoViewer。onChange 存在 = 编辑态。
export function ContentArea({
  asset,
  theme,
  readOnly,
  onChange,
  highlights,
  fill,
}: {
  asset: Asset
  theme: 'light' | 'dark'
  readOnly?: boolean
  onChange?: (v: string) => void
  // #7 命中位置高亮:透传到 MonacoViewer;有非空 highlights 时 markdown 默认切源码视图
  //(否则高亮被预览挡住,看不见)。camelCase 与 MonacoViewer.highlights 对齐。
  highlights?: { line: number; startCol: number; endCol: number }[]
  // #6 全屏填充:fill=true 时 markdown 源码容器 + MonacoViewer 撑满父容器高度(100%),
  // 与预览视图在 fullscreen Modal 里对齐(否则源码视图停留在默认 min(60vh,560px),被截断)。
  // 仅 AssetEditor 全屏 Modal 传 fill=true;内联只读态 / 编辑态不传 → 行为不变。
  fill?: boolean
}) {
  const { t } = useTranslation()
  // 编辑态默认源码视图(让用户进入编辑即可直接修改,无需手动切「源码」)。
  // 有命中位置(#7)时也默认源码:让 hit-line 高亮可见,不被 markdown 预览挡住。
  const [view, setView] = useState<'preview' | 'source'>(highlights && highlights.length > 0 ? 'source' : (onChange ? 'source' : 'preview'))

  const isMarkdown = MARKDOWN_TYPES.has(asset.type)
  const isScript = asset.type === 'script'
  const isStructured = STRUCTURED_TYPES.has(asset.type)

  // markdown:有 content 才渲染预览/源码;编辑态(content 可为空)也进入此分支
  if (isMarkdown && (onChange || asset.content)) {
    return (
      <Card
        size="small"
        title={t('content.title')}
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column' } }}
        extra={
          <Segmented
            size="small"
            value={view}
            onChange={(v) => setView(v as 'preview' | 'source')}
            options={[{ value: 'preview', label: t('content.preview') }, { value: 'source', label: t('content.source') }]}
          />
        }
      >
        {view === 'preview' ? (
          <div style={{ padding: 12, flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column' }}>
            <MarkdownPreview content={asset.content ?? ''} />
          </div>
        ) : (
          <div style={{ padding: 12, ...(fill ? { flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' } : {}) }}>
            <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
              <MonacoViewer value={asset.content ?? ''} language="markdown" theme={theme} readOnly={readOnly} onChange={onChange} highlights={highlights} height={fill ? '100%' : undefined} />
            </Suspense>
          </div>
        )}
      </Card>
    )
  }

  // script:Monaco(按扩展名选语言);编辑态(content 可为空)也进入此分支
  if (isScript && (onChange || asset.content)) {
    return (
      <Card
        size="small"
        title={t('content.title')}
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
      >
        <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
          <MonacoViewer value={asset.content ?? ''} language={langByExt(asset.source_path)} theme={theme} readOnly={readOnly} onChange={onChange} highlights={highlights} height={fill ? '100%' : undefined} />
        </Suspense>
      </Card>
    )
  }

  // structured:JSON 类资产不做结构化预览,直接 Monaco JSON 源码
  //(settings/permissions/mcp_server/hook/keybinding/plugin)。fields.raw 为原始文本,
  // 否则 JSON.stringify(fields)。空对象 → Empty(只读态)。
  //
  // 编辑态(onChange 提供):用 asset.content 作为 Monaco 值。AssetEditor 传 {...asset, content: draft}
  // 覆盖 content,使编辑态 Monaco 显示 draft 而非 fields.raw。
  // 原因:structured 资产无 asset.content,若编辑态仍从 fields.raw 取值,则 onChange 更新 draft 后
  // fields.raw 不变,Monaco 始终显示旧值 → 编辑 silently revert。改用 asset.content(= draft)解决。
  if (isStructured) {
    const raw = (asset.fields as Record<string, unknown> | undefined)?.raw
    const readOnlyValue = typeof raw === 'string'
      ? raw
      : JSON.stringify(asset.fields ?? {}, null, 2)
    const value = onChange ? (asset.content ?? '') : readOnlyValue
    if (!onChange && (value === '{}' || value === '')) {
      return <Card size="small" title={t('content.title')}><Empty description={t('content.emptyNoFields')} /></Card>
    }
    return (
      <Card
        size="small"
        title={t('content.title')}
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
      >
        <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
          <MonacoViewer value={value} language="json" theme={theme} readOnly={readOnly} onChange={onChange} highlights={highlights} height={fill ? '100%' : undefined} />
        </Suspense>
      </Card>
    )
  }

  // 空 content + 空 fields(或 type 不在已知集合)
  if (!asset.content && (!asset.fields || Object.keys(asset.fields).length === 0)) {
    return <Card size="small" title={t('content.title')}><Empty description={t('content.emptyNoContent')} /></Card>
  }

  // 兜底:有 content 但 type 非 markdown/script(罕见),按 plaintext Monaco
  if (asset.content) {
    return (
      <Card
        size="small"
        title={t('content.title')}
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
      >
        <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
          <MonacoViewer value={asset.content} language="plaintext" theme={theme} readOnly={readOnly} onChange={onChange} highlights={highlights} />
        </Suspense>
      </Card>
    )
  }

  // 兜底:有 fields 但 type 非 structured
  const value = JSON.stringify(asset.fields ?? {}, null, 2)
  return (
    <Card
      size="small"
      title={t('content.title')}
      style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
      styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
    >
      <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
        <MonacoViewer value={value} language="json" theme={theme} readOnly={readOnly} onChange={onChange} highlights={highlights} />
      </Suspense>
    </Card>
  )
}
