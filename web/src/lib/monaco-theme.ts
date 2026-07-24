import * as monaco from 'monaco-editor'

// 从设计令牌派生 Monaco SOC 主题(替代内置 vs/vs-dark)。
// sev 色固定不随主题(硬规则),作字符串/数字 token 标记色。
// 副作用 import:模块求值时 defineTheme,必须在 monaco.editor.create 前执行
// (MonacoViewer 静态 import 本模块,模块求值先于 useEffect create)。
//
// 注意:Monaco 的 token rules.foreground 只接受 6 位 hex(不带 #),不接受 OKLCH。
// 故此处维护一套 Monaco 专用 hex 语法色(与 design.md 的 OKLCH 主色视觉对齐),
// 不复用 tokens.ts 的 OKLCH 字符串——后者仅供 CSS var 与 antd 消费。
// editor.background/colors 接受 hex/rgba,用 hex 与设计令牌同色温(青绿偏冷)。
function defineSentinelTheme(mode: 'light' | 'dark') {
  // Monaco 专用 hex 色板(与 tokens.ts OKLCH 视觉对齐,浅深各一套)。
  const dark = mode === 'dark'
  const palette = dark
    ? {
        bg: '#14110F',        // paper(原 --color-paper 深色)
        bgTransparent: '#14110F00',
        ink: '#EDE8E0',       // ink
        muted: '#9A938A',     // muted
        dim: '#6B665F',       // dim
        surface: '#221E1A',   // surface(行高亮)
        accent: '#2FB8A3',    // accent(深色)
        accentSoft: 'rgba(47,184,163,0.20)',
        sevLow: '#4FB868',
        sevMedium: '#E8C547',
        scopePlugin: '#B98FE0',
      }
    : {
        bg: '#F6F5F1',
        bgTransparent: '#F6F5F100',
        ink: '#1B1A17',
        muted: '#5C574F',
        dim: '#8A857C',
        surface: '#F2F1EC',
        accent: '#1B6E6A',
        accentSoft: 'rgba(27,110,106,0.18)',
        sevLow: '#2E8B4A',
        sevMedium: '#B8901F',
        scopePlugin: '#6B3A8C',
      }
  monaco.editor.defineTheme(`sentinel-${mode}`, {
    base: dark ? 'vs-dark' : 'vs',
    inherit: true,
    rules: [
      { token: 'comment', foreground: palette.muted.slice(1) },
      { token: 'keyword', foreground: palette.accent.slice(1) },
      { token: 'string', foreground: palette.sevLow.slice(1) },
      { token: 'number', foreground: palette.sevMedium.slice(1) },
      { token: 'type', foreground: palette.scopePlugin.slice(1) },
    ],
    colors: {
      'editor.background': palette.bgTransparent,
      'editor.foreground': palette.ink,
      'editorLineNumber.foreground': palette.dim,
      'editor.lineHighlightBackground': palette.surface,
      'editor.selectionBackground': palette.accentSoft,
      'editorCursor.foreground': palette.accent,
    },
  })
}

defineSentinelTheme('light')
defineSentinelTheme('dark')
