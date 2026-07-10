import { useEffect, useRef } from 'react'
import * as monaco from 'monaco-editor'
import '../lib/monaco-workers' // 副作用:注册 self.MonacoEnvironment(worker)
import '../lib/monaco-theme' // 副作用:注册 sentinel-light/dark 主题(从 tokens 派生)

// MonacoViewer:monaco-editor 薄包装。readOnly 默认 true(保留 P1 只读行为),
// 可传 readOnly={false} + onChange 解锁编辑(P2)。无 minimap。
// 主题由 props 传入(app useTheme() 的 'light'|'dark' → Monaco 'sentinel-light'|'sentinel-dark'),
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
  readOnly = true,
  onChange,
}: {
  value: string
  language: string
  theme: 'light' | 'dark'
  height?: string
  readOnly?: boolean
  onChange?: (value: string) => void
}) {
  const ref = useRef<HTMLDivElement>(null)
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)
  // onChange ref:始终持有最新 onChange,避免 mount effect 捕获首渲染闭包(stale closure)。
  // 调用方传内联箭头或 undefined→defined 切换时,编辑回调不会失效(P2 编辑路径不可丢编辑)。
  const onChangeRef = useRef(onChange)
  useEffect(() => { onChangeRef.current = onChange })

  // 创建 editor(仅一次,依 mount)
  useEffect(() => {
    if (!ref.current) return
    const editor = monaco.editor.create(ref.current, {
      value,
      language,
      theme: theme === 'dark' ? 'sentinel-dark' : 'sentinel-light',
      readOnly,
      minimap: { enabled: false },
      automaticLayout: true,
      scrollBeyondLastLine: false,
      fontSize: 12.5,
      fontFamily: 'var(--font-mono, "JetBrains Mono", ui-monospace, monospace)',
      lineNumbers: 'on',
      wordWrap: 'on',
    })
    // 编辑回调:通过 ref 调用最新 onChange,无 stale closure 风险。
    // 无需 if(onChange) 守卫——ref 为 undefined 时 ?. 安全跳过。
    editor.onDidChangeModelContent(() => onChangeRef.current?.(editor.getValue()))
    editorRef.current = editor
    return () => {
      editor.dispose()
      editorRef.current = null
    }
    // 仅 mount 时创建;value/language/theme 变化由下方 effect 处理,不重建
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // readOnly 变化 → 更新
  useEffect(() => {
    if (editorRef.current) {
      editorRef.current.updateOptions({ readOnly })
    }
  }, [readOnly])

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
      editorRef.current.updateOptions({ theme: theme === 'dark' ? 'sentinel-dark' : 'sentinel-light' })
    }
  }, [theme])

  return <div ref={ref} style={{ height, width: '100%' }} />
}
