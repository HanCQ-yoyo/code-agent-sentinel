import type { Severity } from '../types'

// 严重度统一枚举(全平台唯一来源):级别文案 key、排序、配色点。
// 文案标准:严重 / 高危 / 中危 / 低危 / 提示(原「高/中/低/信息」统一为双字,
// 与「严重」对齐成等宽级别序列,标签更整齐)。
export const SEVERITY_ORDER: Severity[] = ['critical', 'high', 'medium', 'low', 'info']

// SEVERITY_LABEL_KEY 存 i18n key(非中文文案),组件用 t(SEVERITY_LABEL_KEY[sev]) 取当前语言文案。
// key 对应 zh.json / en.json 顶层 severity 命名空间的 5 个子键(critical/high/medium/low/info)。
export const SEVERITY_LABEL_KEY: Record<Severity, string> = {
  critical: 'severity.critical',
  high: 'severity.high',
  medium: 'severity.medium',
  low: 'severity.low',
  info: 'severity.info',
}

// 级别色点 / 图表条 / 数字着色用基础色 token(非标签填充);「全部」筛选用 accent。
export const SEVERITY_DOT: Record<Severity, string> = {
  critical: 'var(--sev-critical)',
  high: 'var(--sev-high)',
  medium: 'var(--sev-medium)',
  low: 'var(--sev-low)',
  info: 'var(--sev-info)',
}

// 处置状态 → 实色背景 token(替代 antd 命名色,统一 OKLCH)。
// open=中性、in_progress=cat-1 蓝、resolved=sev-low 绿、false_positive=cat-5 紫、accepted=cat-3 黄。
export const STATUS_COLOR: Record<string, string> = {
  open: 'var(--color-rule-2)',
  in_progress: 'var(--cat-1)',
  resolved: 'var(--sev-low-solid)',
  false_positive: 'var(--cat-5)',
  accepted: 'var(--cat-3)',
}

// 优先级 → 实色背景 token:P0 红 → P3 蓝。
export const PRIORITY_COLOR: Record<string, string> = {
  P0: 'var(--sev-critical-solid)',
  P1: 'var(--sev-high-solid)',
  P2: 'var(--sev-medium-solid)',
  P3: 'var(--cat-1)',
}

// severity → 优先级派生(从 FindingTable/FindingDrawer 提取,消除重复)。
export const severityToPrio = (s: Severity): string =>
  ({ critical: 'P0', high: 'P1', medium: 'P2', low: 'P3', info: 'P3' } as Record<Severity, string>)[s]
