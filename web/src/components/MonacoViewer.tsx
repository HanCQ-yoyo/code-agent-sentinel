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
  highlights,
}: {
  value: string
  language: string
  theme: 'light' | 'dark'
  height?: string
  readOnly?: boolean
  onChange?: (value: string) => void
  // #7 命中位置高亮:RulesDetector finding 带 locations 时,整行背景高亮 + 滚到首个命中。
  // camelCase 与 Monaco Range API 对齐;由 FindingDrawer 在边界从 snake_case 映射而来。
  highlights?: { line: number; startCol: number; endCol: number }[]
}) {
  const ref = useRef<HTMLDivElement>(null)
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)
  // onChange ref:始终持有最新 onChange,避免 mount effect 捕获首渲染闭包(stale closure)。
  // 调用方传内联箭头或 undefined→defined 切换时,编辑回调不会失效(P2 编辑路径不可丢编辑)。
  const onChangeRef = useRef(onChange)
  useEffect(() => { onChangeRef.current = onChange })

  // #7 命中行装饰引用:deltaDecorations 返回新 decorations id 数组,下次运行作为旧 decorations 清除。
  // null/空 highlights 时不加装饰(优雅降级:子进程检测器无 locations 不报错)。
  const decorationsRef = useRef<string[]>([])

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
      // alwaysConsumeMouseWheel:false:Monaco 默认拦截一切 wheel 事件,即使已到滚动边界
      // 也不冒泡。markdown 预览里代码块(MonacoBlock)嵌在外层 .markdown-preview 滚动容器中,
      // 鼠标停在代码块上滚动时 wheel 被 Monaco 吞掉,滚到代码顶/底也无法继续滚预览页。
      // 设 false 后:Monaco 仅在自身确有内容可滚时消费 wheel,到边界(或内容不溢出)即放行
      // 冒泡到外层预览容器,嵌套滚动联动正常。对源码/脚本编辑态无影响(范围内仍滚代码)。
      scrollbar: { alwaysConsumeMouseWheel: false },
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

  // #7 highlights 变化 → deltaDecorations 整行高亮 + revealLineInCenter 滚到首个命中。
  // 依赖 editorRef.current(value effect 已 setValue、editor 必已存在)。
  // 空/undefined highlights:不加装饰,不滚动(优雅降级:子进程检测器 finding 无 locations)。
  // 清旧加新:deltaDecorations 接收旧 id 数组 + 新 decorations,返回新 id 数组存回 ref。
  useEffect(() => {
    const editor = editorRef.current
    if (!editor) return
    if (!highlights || highlights.length === 0) {
      // 清除上一轮装饰(切到无 locations 的 finding 时)
      if (decorationsRef.current.length > 0) {
        decorationsRef.current = editor.deltaDecorations(decorationsRef.current, [])
      }
      return
    }
    const decos = highlights.map((h) => ({
      range: new monaco.Range(h.line, h.startCol, h.line, h.endCol),
      options: {
        isWholeLine: true,
        className: 'hit-line',
        // backgroundColor 用固定 rgba(--warn-bg 变量不存在,见 index.css 注释)。
        backgroundColor: 'rgba(250,173,20,0.18)',
      },
    }))
    editor.revealLineInCenter(highlights[0].line)
    decorationsRef.current = editor.deltaDecorations(decorationsRef.current, decos)
  }, [highlights])

  return <div ref={ref} style={{ height, width: '100%' }} />
}
