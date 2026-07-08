import * as monaco from 'monaco-editor'
import { tokens } from '../theme/tokens'

// 从设计令牌派生 Monaco SOC 主题(替代内置 vs/vs-dark)。
// sev 色固定不随主题(硬规则),作字符串/数字 token 标记色。
// 副作用 import:模块求值时 defineTheme,必须在 monaco.editor.create 前执行
// (MonacoViewer 静态 import 本模块,模块求值先于 useEffect create)。
function defineSentinelTheme(mode: 'light' | 'dark') {
  const t = tokens[mode]
  monaco.editor.defineTheme(`sentinel-${mode}`, {
    base: mode === 'dark' ? 'vs-dark' : 'vs',
    inherit: true,
    rules: [
      { token: 'comment', foreground: t.textMuted.slice(1) },
      { token: 'keyword', foreground: t.accent.slice(1) },
      { token: 'string', foreground: t.sevLow.slice(1) },
      { token: 'number', foreground: t.sevMedium.slice(1) },
      { token: 'type', foreground: t.scopePlugin.slice(1) },
    ],
    colors: {
      'editor.background': t.bg + '00',
      'editor.foreground': t.text,
      'editorLineNumber.foreground': t.textDim,
      'editor.lineHighlightBackground': t.surface2,
      'editor.selectionBackground': t.brandSoft,
      'editorCursor.foreground': t.accent,
    },
  })
}

defineSentinelTheme('light')
defineSentinelTheme('dark')
