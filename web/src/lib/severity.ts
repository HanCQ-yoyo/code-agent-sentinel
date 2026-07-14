import type { Severity } from '../types'

// 严重度统一枚举(全平台唯一来源):级别文案、排序、配色点。
// 文案标准:严重 / 高危 / 中危 / 低危 / 提示(原「高/中/低/信息」统一为双字,
// 与「严重」对齐成等宽级别序列,标签更整齐)。
export const SEVERITY_ORDER: Severity[] = ['critical', 'high', 'medium', 'low', 'info']

export const SEVERITY_LABEL: Record<Severity, string> = {
  critical: '严重',
  high: '高危',
  medium: '中危',
  low: '低危',
  info: '提示',
}

// 级别色点 / 图表条 / 数字着色用基础色 token(非标签填充);「全部」筛选用 accent。
export const SEVERITY_DOT: Record<Severity, string> = {
  critical: 'var(--sev-critical)',
  high: 'var(--sev-high)',
  medium: 'var(--sev-medium)',
  low: 'var(--sev-low)',
  info: 'var(--sev-info)',
}
