// AgentIcon:agent 展示图标(行内)。claude-code 用品牌色 SVG logo(web/src/logo/claudecode-color.svg),
// 未知 agent 回退方块符号。统一替代 agentMeta.icon 的 emoji,与 Anthropic/Claude 品牌橙(#D97757)一致。
//
// 用 ReactNode 而非 string:antd Select/Tabs 的 label 接受 ReactNode,行内渲染也能对齐基线。
// SVG 内联(非 <img>):尺寸随 font-size 缩放(1em),与文字同行高度对齐。
import type { CSSProperties } from 'react'

// Claude Code 品牌 logo path(源自 web/src/logo/claudecode-color.svg,fill 固定 #D97757)。
const CLAUDE_CODE_PATH =
  'M20.998 10.949H24v3.102h-3v3.028h-1.487V20H18v-2.921h-1.487V20H15v-2.921H9V20H7.488v-2.921H6V20H4.487v-2.921H3V14.05H0V10.95h3V5h17.998v5.949zM6 10.949h1.488V8.102H6v2.847zm10.51 0H18V8.102h-1.49v2.847z'

interface Props {
  id: string
  // 与文字同行的尺寸:默认 1em(随 font-size)。可传数字(px)或字符串覆盖。
  size?: number | string
  style?: CSSProperties
}

export function AgentIcon({ id, size = '1em', style }: Props) {
  if (id === 'claude-code') {
    return (
      <svg
        height={size}
        width={size}
        viewBox="0 0 24 24"
        xmlns="http://www.w3.org/2000/svg"
        // display:inline-block 保证与同行文字水平排列(默认 inline 在部分布局下会换行到上一行);
        // verticalAlign:middle 使 logo 与文字中线对齐;marginRight 留出图标与文字的间距。
        style={{ display: 'inline-block', verticalAlign: 'middle', marginRight: '0.2em', lineHeight: 1, ...style }}
        role="img"
        aria-label="Claude Code"
      >
        <title>Claude Code</title>
        <path clipRule="evenodd" d={CLAUDE_CODE_PATH} fill="#D97757" fillRule="evenodd" />
      </svg>
    )
  }
  // 未知 agent:回退方块(与旧 agentMeta '▪' 一致),避免空白。
  return <span style={{ display: 'inline-block', width: size, verticalAlign: 'middle', marginRight: '0.2em', ...style }}>▪</span>
}
