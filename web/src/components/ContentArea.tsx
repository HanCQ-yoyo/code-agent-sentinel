import { lazy, Suspense, useState, type ReactNode } from 'react'
import { Card, Segmented, Spin, Empty, Space, Descriptions, Typography } from 'antd'
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

// structured 类资产:后端已为有原文件的类型(settings/permissions/hook/keybinding/
// .mcp.json)设 content=原文件文本,UI 直接展示 content(Monaco JSON 源码)。
// 无 content 的 structured(MCP server 来自 ~/.claude.json 大文件、plugin 来自目录)
// 展示结构化字段 KV 视图(StructuredFields)。绝不 JSON.stringify(整个 fields)——
// 旧实现把 {model:"",env,raw:{整文件}} 整个 dump,文件被冗余包在 raw 里、空 model 误导。
const STRUCTURED_TYPES = new Set(['settings', 'permissions', 'mcp_server', 'hook', 'keybinding', 'plugin'])

// editableText:返回资产的可编辑文本(与只读态 Monaco 渲染一致)。
// markdown/script/structured → asset.content;兜底 → content ?? JSON.stringify(fields)。
// AssetEditor 用此初始化 draft,确保编辑起点 = 用户在只读态所见文本。
// structured 资产现由后端提供 content=原文件文本(configengine parseSettings 等已设),
// 故直接用 content;content 缺失(MCP 来自 .claude.json 等)回退空串——这类资产
// 不可编辑(AssetEditor.enterEdit 会先 preview 探测,editable=false 时拒绝进入编辑态)。
export function editableText(asset: Asset): string {
  const isMarkdown = MARKDOWN_TYPES.has(asset.type)
  const isScript = asset.type === 'script'
  const isStructured = STRUCTURED_TYPES.has(asset.type)
  if (isMarkdown || isScript || isStructured) return asset.content ?? ''
  return asset.content ?? JSON.stringify(asset.fields ?? {}, null, 2)
}

// formatFieldValue:把 structured KV 视图的字段值格式化为可读字符串。
// string → 原样;array → 逗号分隔(mcp args / permissions allow 等更直观);
// object → JSON(mcp env 等);其余 → String(v)。空数组/空串显示「(空)」。
function formatFieldValue(v: unknown): string {
  if (v === null || v === undefined) return '(空)'
  if (typeof v === 'string') return v === '' ? '(空)' : v
  if (Array.isArray(v)) return v.length === 0 ? '(空)' : v.join(', ')
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

// ContentArea 按 asset.type 分派渲染:
// - markdown → Segmented[预览|源码],默认预览(编辑态默认源码)
// - script → Monaco(langByExt(source_path))
// - structured → 有 content:Monaco json(原文件文本);无 content:结构化 KV 视图
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
  headerActions,
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
  // 内容框标题区操作按钮(编辑/全屏 等),由调用方 AssetEditor 注入。渲染在 Card extra 最左侧,
  // markdown 视图里排在「预览/源码」Segmented 左侧;script/structured 等无 Segmented 的视图里
  // 独占 extra。调用方不传则不渲染(如全屏 Modal 内复用时不需要再嵌编辑/全屏按钮)。
  headerActions?: ReactNode
}) {
  const { t } = useTranslation()
  // 编辑态默认源码视图(让用户进入编辑即可直接修改,无需手动切「源码」)。
  // 有命中位置(#7)时也默认源码:让 hit-line 高亮可见,不被 markdown 预览挡住。
  const [view, setView] = useState<'preview' | 'source'>(highlights && highlights.length > 0 ? 'source' : (onChange ? 'source' : 'preview'))

  const isMarkdown = MARKDOWN_TYPES.has(asset.type)
  const isScript = asset.type === 'script'
  const isStructured = STRUCTURED_TYPES.has(asset.type)

  // 卡片标题区 extra 组合:headerActions(编辑/全屏,左侧)+ 预览/源码 Segmented(markdown 视图,右侧)。
  // 两块用 antd Space 隔开;无 Segmented 的视图(script/structured/兜底)只放 headerActions。
  const previewSourceSeg = (
    <Segmented
      size="small"
      value={view}
      onChange={(v) => setView(v as 'preview' | 'source')}
      options={[{ value: 'preview', label: t('content.preview') }, { value: 'source', label: t('content.source') }]}
    />
  )
  const cardExtra = headerActions ? (
    <Space size="small" align="center">
      {headerActions}
      {isMarkdown ? previewSourceSeg : null}
    </Space>
  ) : isMarkdown ? previewSourceSeg : undefined

  // markdown:有 content 才渲染预览/源码;编辑态(content 可为空)也进入此分支
  if (isMarkdown && (onChange || asset.content)) {
    return (
      <Card
        size="small"
        title={t('content.title')}
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column' } }}
        extra={cardExtra}
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
        extra={cardExtra}
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
      >
        <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
          <MonacoViewer value={asset.content ?? ''} language={langByExt(asset.source_path)} theme={theme} readOnly={readOnly} onChange={onChange} highlights={highlights} height={fill ? '100%' : undefined} />
        </Suspense>
      </Card>
    )
  }

  // structured:有 content(settings/permissions/hook/keybinding/.mcp.json)→ Monaco JSON
  // 源码(原文件文本);无 content(MCP 来自 ~/.claude.json、plugin 来自目录)→ 结构化字段
  // KV 视图(StructuredFields),绝不 JSON.stringify(整个 fields)。
  //
  // 编辑态(onChange 提供):仅对有 content 的资产可达(无 content 的 MCP/plugin 不可编辑,
  // AssetEditor.enterEdit 的 preview 探测会返回 editable=false 拒绝进入编辑态)。AssetEditor
  // 传 {...asset, content: draft} 覆盖 content,故编辑态 Monaco 显示 draft。
  if (isStructured) {
    const value = onChange ? (asset.content ?? '') : (asset.content ?? '')
    // 有原文件文本:Monaco JSON 源码。
    if (value !== '') {
      return (
        <Card
          size="small"
          title={t('content.title')}
          extra={cardExtra}
          style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
          styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
        >
          <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
            <MonacoViewer value={value} language="json" theme={theme} readOnly={readOnly} onChange={onChange} highlights={highlights} height={fill ? '100%' : undefined} />
          </Suspense>
        </Card>
      )
    }
    // 无原文件文本:结构化字段 KV 视图(MCP server 来自 .claude.json / plugin)。
    // 跳过内部字段 raw(检测器全文本载体,非用户可见语义)。
    const fields = asset.fields ?? {}
    const entries = Object.entries(fields).filter(([k]) => k !== 'raw')
    if (entries.length === 0) {
      return <Card size="small" title={t('content.title')} extra={cardExtra}><Empty description={t('content.emptyNoFields')} /></Card>
    }
    return (
      <Card
        size="small"
        title={t('content.title')}
        extra={cardExtra}
        style={{ flex: 1, minHeight: 240, overflow: 'auto' }}
        styles={{ body: { padding: 12 } }}
      >
        <div data-testid="structured-kv">
          <Descriptions size="small" column={1} bordered>
            {entries.map(([k, v]) => (
              <Descriptions.Item key={k} label={<Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{k}</Typography.Text>}>
                <Typography.Text style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{formatFieldValue(v)}</Typography.Text>
              </Descriptions.Item>
            ))}
          </Descriptions>
        </div>
      </Card>
    )
  }

  // 空 content + 空 fields(或 type 不在已知集合)
  if (!asset.content && (!asset.fields || Object.keys(asset.fields).length === 0)) {
    return <Card size="small" title={t('content.title')} extra={cardExtra}><Empty description={t('content.emptyNoContent')} /></Card>
  }

  // 兜底:有 content 但 type 非 markdown/script(罕见),按 plaintext Monaco
  if (asset.content) {
    return (
      <Card
        size="small"
        title={t('content.title')}
        extra={cardExtra}
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
      extra={cardExtra}
      style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
      styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
    >
      <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
        <MonacoViewer value={value} language="json" theme={theme} readOnly={readOnly} onChange={onChange} highlights={highlights} />
      </Suspense>
    </Card>
  )
}
