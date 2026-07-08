import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

// MarkdownPreview:react-markdown + remark-gfm(GFM 表格/任务列表/删除线)渲染 markdown 正文。
// 人语 Inter 优先(prose),代码块用 var(--font-mono)。外层 flex:1 + overflow auto,内容驱动高度。
// 硬规则:机器语 mono / 人语 Inter——正文 Inter,代码 mono。
//
// react-markdown v10 注意:不再传 `inline` prop;围栏代码块走 `pre > code`(`code` 带 `language-xxx` className),
// 行内 code 无 className。因此 `pre` 覆盖负责块级外壳,`code` 覆盖按 className 区分行内/块内内容。
export function MarkdownPreview({ content }: { content: string }) {
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
          // 块级代码外壳:react-markdown v10 对围栏代码渲染 `pre > code`,
          // 覆盖 `pre` 注入 mono 字体 + surface 背景,避免与 `code` 覆盖产生嵌套 <pre>。
          pre({ children }: any) {
            return (
              <pre
                style={{
                  margin: '8px 0',
                  padding: 12,
                  background: 'var(--surface-2)',
                  borderRadius: 6,
                  overflow: 'auto',
                  fontFamily: 'var(--font-mono)',
                  fontSize: 12.5,
                  lineHeight: 1.5,
                }}
              >
                {children}
              </pre>
            )
          },
          // code:有 className(language-xxx)→ 块内 code(已被 pre 包裹);无 className → 行内 code。
          code({ className, children, ...props }: any) {
            if (className) {
              return (
                <code className={className} style={{ fontFamily: 'var(--font-mono)' }} {...props}>
                  {children}
                </code>
              )
            }
            return (
              <code
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: '0.9em',
                  background: 'var(--surface-2)',
                  padding: '1px 4px',
                  borderRadius: 3,
                }}
                {...props}
              >
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
          a({ children, href }: any) {
            return <a href={href} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--accent)' }}>{children}</a>
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
