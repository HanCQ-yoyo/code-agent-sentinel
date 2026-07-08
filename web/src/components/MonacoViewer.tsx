import { useEffect, useRef } from 'react'
import * as monaco from 'monaco-editor'
import '../lib/monaco-workers' // 副作用:注册 self.MonacoEnvironment(worker)

// MonacoViewer:monaco-editor 薄包装。P1 只读(readOnly:true),无 minimap。
// 主题由 props 传入(app useTheme() 的 'light'|'dark' → Monaco 'vs'|'vs-dark'),
// theme 变化时 useEffect 重新 updateOptions,无需重建 editor。
// 通过 React.lazy 动态导入(ContentArea 内),markdown 默认预览不触发加载。
//
// height:Monaco 需确定高度(sticky 树右栏 maxHeight 80vh 无固定高,flex 不可靠)。
// 默认 'min(60vh, 560px)'。
export default function MonacoViewer({
  value,
  language,
  theme,
  height = 'min(60vh, 560px)',
}: {
  value: string
  language: string
  theme: 'light' | 'dark'
  height?: string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)

  // 创建 editor(仅一次,依 mount)
  useEffect(() => {
    if (!ref.current) return
    const editor = monaco.editor.create(ref.current, {
      value,
      language,
      theme: theme === 'dark' ? 'vs-dark' : 'vs',
      readOnly: true,
      minimap: { enabled: false },
      automaticLayout: true,
      scrollBeyondLastLine: false,
      fontSize: 12.5,
      fontFamily: 'var(--font-mono, "JetBrains Mono", ui-monospace, monospace)',
      lineNumbers: 'on',
      wordWrap: 'on',
    })
    editorRef.current = editor
    return () => {
      editor.dispose()
      editorRef.current = null
    }
    // 仅 mount 时创建;value/language/theme 变化由下方 effect 处理,不重建
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // value 变化 → setValue(保留光标位置尽量)
  useEffect(() => {
    if (editorRef.current && editorRef.current.getValue() !== value) {
      editorRef.current.setValue(value)
    }
  }, [value])

  // language 变化 → 更新
  useEffect(() => {
    if (editorRef.current) {
      const model = editorRef.current.getModel()
      if (model) monaco.editor.setModelLanguage(model, language)
    }
  }, [language])

  // theme 变化 → 更新(app 切深浅色)
  useEffect(() => {
    if (editorRef.current) {
      editorRef.current.updateOptions({ theme: theme === 'dark' ? 'vs-dark' : 'vs' })
    }
  }, [theme])

  return <div ref={ref} style={{ height, width: '100%' }} />
}
