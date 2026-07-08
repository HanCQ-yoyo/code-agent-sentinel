import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { Spin } from 'antd'
import { langByClassName } from '../lib/monaco-lang'

// MonacoViewer 懒加载:md 代码块进视口才拉 monaco chunk。
const MonacoViewer = lazy(() => import('./MonacoViewer'))

// MonacoBlock:md 围栏代码块内嵌 Monaco。IntersectionObserver 进视口才 mount、离开销毁。
// read-only,按行数定高(min 40px/行,max 400px)。无语言标签 → plaintext(仍高亮为纯文本)。
export function MonacoBlock({ code, className, theme }: { code: string; className?: string; theme: 'light' | 'dark' }) {
  const ref = useRef<HTMLDivElement>(null)
  const [visible, setVisible] = useState(false)
  const language = langByClassName(className)
  const lineCount = code.split('\n').length
  const height = `${Math.min(Math.max(lineCount * 18 + 16, 60), 400)}px`

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const io = new IntersectionObserver(
      (entries) => { for (const e of entries) setVisible(e.isIntersecting) },
      { rootMargin: '100px' }
    )
    io.observe(el)
    return () => io.disconnect()
  }, [])

  return (
    <div ref={ref} style={{ height, margin: '8px 0', borderRadius: 6, overflow: 'hidden' }}>
      {visible ? (
        <Suspense fallback={<Spin style={{ display: 'block', margin: '20px auto' }} />}>
          <MonacoViewer value={code} language={language} theme={theme} height={height} />
        </Suspense>
      ) : (
        <pre style={{ margin: 0, height: '100%', padding: 12, background: 'var(--surface-2)', overflow: 'auto', fontFamily: 'var(--font-mono)', fontSize: 12.5 }}>{code}</pre>
      )}
    </div>
  )
}
