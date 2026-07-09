import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useTheme } from '../theme'
import { MonacoBlock } from './MonacoBlock'

// MarkdownPreview:react-markdown + remark-gfm(GFM 表格/任务列表/删除线)渲染 markdown 正文。
// 人语 Inter 优先(prose),代码块用 var(--font-mono)。外层 flex:1 + overflow auto,内容驱动高度。
// 硬规则:机器语 mono / 人语 Inter——正文 Inter,代码 mono。
//
// react-markdown v10 注意:不再传 `inline` prop;围栏代码块走 `pre > code`(`code` 带 `language-xxx` className),
// 行内 code 无 className。因此 `pre` 覆盖负责块级外壳,`code` 覆盖按 className 区分行内/块内内容。
export function MarkdownPreview({ content }: { content: string }) {
  const { theme } = useTheme()
  return (
    <div
      className="markdown-preview"
      style={{
        flex: 1,
        overflow: 'auto',
        fontFamily: 'var(--font-sans)',
        fontSize: 14,
        lineHeight: 1.7,
        color: 'var(--text)',
      }}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          pre({ children }: any) {
            // react-markdown v10:围栏块 pre > code(code 带 language-xxx className)。
            // 提取 code 的 className + 文本,走 MonacoBlock(懒加载 + 进视口创建)。
            const child = Array.isArray(children) ? children[0] : children
            const codeEl: any = child?.props
            const className: string | undefined = codeEl?.className
            const codeText: string = extractText(codeEl?.children)
            if (codeText) {
              return <MonacoBlock code={codeText} className={className} theme={theme} />
            }
            // 兜底:无法提取文本(罕见),回退 styled pre
            return <pre style={{ margin: '8px 0', padding: 12, background: 'var(--surface-2)', borderRadius: 6, overflow: 'auto', fontFamily: 'var(--font-mono)', fontSize: 12.5 }}>{children}</pre>
          },
          // code:react-markdown v10 围栏块走 pre>code(本 override 仅渲染 code 元素本身,
          // 块外壳与 Monaco 由 pre override 接管)。此处 code 总被 pre 包裹 → 渲染纯文本 code。
          // 行内 code(无 className 且 node.position 单行)→ inline 样式。
          // 解构并丢弃 `node`(react-markdown v10 passNode 注入的 hast 节点对象),
          // 否则经 ...props 透传到 <code> DOM 触发 React dev 警告 "React does not recognize the `node` prop"。
          code({ node: _node, className, children, ...props }: any) {
            if (className) {
              return <code className={className} style={{ fontFamily: 'var(--font-mono)' }} {...props}>{children}</code>
            }
            // 无 className:行内 code(围栏无标签块的 code 已被 pre override 经 extractText 提取,
            // 不会走到这里渲染 inline;此处仅 true inline)
            return (
              <code style={{ fontFamily: 'var(--font-mono)', fontSize: '0.9em', background: 'var(--surface-2)', padding: '1px 4px', borderRadius: 3 }} {...props}>
                {children}
              </code>
            )
          },
          // 表格边框走 hairline
          table({ children }: any) {
            return <table style={{ borderCollapse: 'collapse', width: '100%', margin: '8px 0' }}>{children}</table>
          },
          th({ children }: any) {
            return <th style={{ border: '1px solid var(--bg-border)', padding: '6px 10px', textAlign: 'left' }}>{children}</th>
          },
          td({ children }: any) {
            return <td style={{ border: '1px solid var(--bg-border)', padding: '6px 10px' }}>{children}</td>
          },
          a({ children, href, title }: any) {
            return <a href={href} title={title} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--accent)' }}>{children}</a>
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}

// extractText:从 react-markdown code 的 children 提取纯文本(可能是 string 或嵌套节点数组)。
function extractText(children: unknown): string {
  if (children === null || children === undefined) return ''
  if (typeof children === 'string') return children
  if (typeof children === 'number') return String(children)
  if (Array.isArray(children)) return children.map(extractText).join('')
  if (typeof children === 'object' && 'props' in (children as any)) {
    return extractText((children as any).props?.children)
  }
  return ''
}
