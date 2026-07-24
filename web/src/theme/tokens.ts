export type Mode = 'light' | 'dark'

// Hallmark · genre: modern-minimal · macrostructure family: Workbench · design-system: design.md · designed-as-app
// 主题:custom OKLCH,锚色青绿 oklch(50% 0.09 195)(原品牌色 #1B6E6A)。中性色往青绿偏冷调。
// 浅深共享 hue,只动 lightness/chroma。无纯黑纯白 —— paper/card 都带微量青绿 chroma。
// 收敛自原 6 套配色:accent + severity + category 三套语义色,加品牌身份色 agent-claude。
// tokens.ts 是 OKLCH 值的 TS 单一来源;index.css 的 :root 与之同名同值(一处定义,两处消费)。

export interface TokenSet {
  // 表面与文字(paper/ink/中性)
  paper: string
  paper2: string
  surface: string
  rule: string
  rule2: string
  muted: string
  dim: string
  ink: string
  accent: string
  accentSoft: string
  focus: string
  onAccent: string
  // severity 5 级(-solid 为标签实色填充,保证白字 ≥ AA)
  sevCritical: string
  sevHigh: string
  sevMedium: string
  sevLow: string
  sevInfo: string
  sevCriticalSolid: string
  sevHighSolid: string
  sevMediumSolid: string
  sevLowSolid: string
  sevInfoSolid: string
  badgeText: string
  // category 6 色循环(统一 scope / pinned-project / 趋势线,替代原 3 处各写一套 hex)
  cat1: string
  cat2: string
  cat3: string
  cat4: string
  cat5: string
  cat6: string
  // 品牌身份色(Claude 橙,仅 agent 图标,与 accent 平级,不进 category)
  agentClaude: string
  fontSans: string
  fontMono: string
}

export const fontSans =
  "'Inter Tight', 'Noto Sans SC', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', sans-serif"
export const fontMono =
  "'JetBrains Mono', 'Noto Sans SC', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"

export const tokens: Record<Mode, TokenSet> = {
  light: {
    paper: 'oklch(97% 0.008 175)',
    paper2: 'oklch(99% 0.004 175)',
    surface: 'oklch(95% 0.010 175)',
    rule: 'oklch(90% 0.008 175)',
    rule2: 'oklch(84% 0.010 175)',
    muted: 'oklch(46% 0.012 175)',
    dim: 'oklch(52% 0.010 175)',
    ink: 'oklch(24% 0.012 175)',
    accent: 'oklch(50% 0.090 195)',
    accentSoft: 'oklch(50% 0.090 195 / 0.10)',
    focus: 'oklch(50% 0.090 195)',
    onAccent: 'oklch(99% 0.004 175)',
    sevCritical: 'oklch(58% 0.190 27)',
    sevHigh: 'oklch(63% 0.150 55)',
    sevMedium: 'oklch(70% 0.130 85)',
    sevLow: 'oklch(62% 0.150 145)',
    sevInfo: 'oklch(62% 0.012 250)',
    sevCriticalSolid: 'oklch(52% 0.200 27)',
    sevHighSolid: 'oklch(56% 0.160 55)',
    sevMediumSolid: 'oklch(50% 0.150 85)',
    sevLowSolid: 'oklch(55% 0.150 145)',
    sevInfoSolid: 'oklch(52% 0.014 250)',
    badgeText: 'oklch(99% 0.004 175)',
    cat1: 'oklch(62% 0.130 250)',
    cat2: 'oklch(60% 0.150 145)',
    cat3: 'oklch(72% 0.150 85)',
    cat4: 'oklch(58% 0.190 27)',
    cat5: 'oklch(55% 0.160 300)',
    cat6: 'oklch(68% 0.110 195)',
    agentClaude: 'oklch(68% 0.130 41)',
    fontSans,
    fontMono,
  },
  dark: {
    paper: 'oklch(16% 0.008 175)',
    paper2: 'oklch(20% 0.010 175)',
    surface: 'oklch(24% 0.010 175)',
    rule: 'oklch(28% 0.008 175)',
    rule2: 'oklch(34% 0.010 175)',
    muted: 'oklch(66% 0.010 175)',
    dim: 'oklch(62% 0.008 175)',
    ink: 'oklch(92% 0.006 175)',
    accent: 'oklch(72% 0.110 195)',
    accentSoft: 'oklch(72% 0.110 195 / 0.14)',
    focus: 'oklch(72% 0.110 195)',
    onAccent: 'oklch(16% 0.008 175)',
    sevCritical: 'oklch(64% 0.180 27)',
    sevHigh: 'oklch(70% 0.140 55)',
    sevMedium: 'oklch(75% 0.130 85)',
    sevLow: 'oklch(68% 0.150 145)',
    sevInfo: 'oklch(70% 0.012 250)',
    sevCriticalSolid: 'oklch(52% 0.200 27)',
    sevHighSolid: 'oklch(56% 0.160 55)',
    sevMediumSolid: 'oklch(50% 0.150 85)',
    sevLowSolid: 'oklch(55% 0.150 145)',
    sevInfoSolid: 'oklch(52% 0.014 250)',
    badgeText: 'oklch(99% 0.004 175)',
    cat1: 'oklch(68% 0.130 250)',
    cat2: 'oklch(66% 0.150 145)',
    cat3: 'oklch(78% 0.140 85)',
    cat4: 'oklch(64% 0.180 27)',
    cat5: 'oklch(62% 0.150 300)',
    cat6: 'oklch(74% 0.110 195)',
    agentClaude: 'oklch(72% 0.130 41)',
    fontSans,
    fontMono,
  },
}
