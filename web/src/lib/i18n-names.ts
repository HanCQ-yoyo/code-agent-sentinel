import i18n from '../i18n'
import type { DetectorMeta } from '../types'

// 规则引擎名 / 规则名称的双语支持。
//
// 背景:后端 detector.name 与 rule.description 都是中文硬编码(见
// internal/security/*_detector.go 的 Name 字段、ruleengine/rules/*.yaml 的 description 字段)。
// 为支持中英双语而不改动所有后端规则文件,在前端维护以 detector_id / rule_id 为键的双语字典:
//   - 有翻译:用 i18n 翻译(随用户语言切换);
//   - 无翻译:回退到调用方传入的原始字符串(后端 message/description/name)。
//
// 规则用 rule_id 做键(稳定唯一),而非 description 文本(文本是中文,无法做 en key)。
// 新增规则时只需在 zh.json/en.json 的 rules 命名空间补一条 {{rule_id}} 即可,未补的规则自动回退。

// 检测器名:detector_id → i18n 翻译(detectors.<id>),回退 fallback(通常传 detector.name)。
export function detectorName(det: { id: string; name: string }): string {
  return detectorNameById([det], det.id)
}

export function detectorNameById(detectors: { id: string; name: string }[], id: string): string {
  const key = `detectors.${id}`
  if (i18n.exists(key)) return i18n.t(key)
  const d = detectors.find((x) => x.id === id)
  return d?.name ?? id
}

// 规则名:rule_id → i18n 翻译(rules.<rule_id>),回退 fallback(传 message/description)。
// 同时服务 Finding(rule_id + message)与 RuleInfo/FlatRule(id + description)。
export function ruleNameById(ruleId: string, fallback: string): string {
  const key = `rules.${ruleId}`
  return i18n.exists(key) ? i18n.t(key) : fallback
}

// 便捷重载:规则对象带 id + description(RuleInfo / FlatRule)。
export function ruleName(rule: { id: string; description: string }): string {
  return ruleNameById(rule.id, rule.description)
}

export type { DetectorMeta }
