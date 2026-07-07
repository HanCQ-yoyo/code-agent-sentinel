export type Severity = 'critical' | 'high' | 'medium' | 'low'

export interface Asset {
  id: string; type: string; scope: string; source_path: string; name: string
  fields?: Record<string, unknown>; content?: string; hash: string; mtime?: string; parse_error?: string
}
export interface Inventory { assets: Asset[]; project?: { path: string; name: string }; duplicates?: unknown[] }
export interface Finding {
  id?: string; detector_id: string; rule_id: string; severity: Severity
  asset_id: string; asset_type: string; asset_name: string; message: string; evidence: string; remediation: string
}
export interface HealthScore { score: number; band: string; deductions: { asset_name: string; rule_id: string; severity: Severity; points: number }[] }
export interface DetectorStatus { id: string; available: boolean; reason?: string }

// 检测器能力元数据(对应后端 security.DetectorMeta)
export interface EngineInfo { name: string; kind: string; available: boolean; reason?: string }
export interface RuleInfo { id: string; severity: Severity; description: string }
export interface DetectorMeta {
  id: string; name: string; engines: EngineInfo[]; rules: RuleInfo[]; covers: string[]
  // 向后兼容:后端 DetectorMeta 未直接含 available/reason,但 UI 仍需整体可用状态
  available: boolean; reason?: string
}

export interface ScanResult {
  findings: Finding[]
  detectors: { id: string; available: boolean; reason?: string; finding_count: number }[]
  health_score?: HealthScore
}

// 历史记录
export interface ScanRecord extends ScanResult {
  id: string
  started_at: string
  duration?: number
  inventory?: Inventory
  project?: { path: string; name: string }
}
export interface ScanSummary {
  id: string; started_at: string; health_score: number; band: string
  finding_count: number; detector_avail: number; detector_total: number
}
