export type Severity = 'critical' | 'high' | 'medium' | 'low'

export interface Asset {
  id: string; type: string; scope: string; source_path: string; name: string
  fields?: Record<string, unknown>; content?: string; hash: string; mtime?: string; parse_error?: string
}
export interface Project { path: string; name: string }
export interface Inventory { assets: Asset[]; projects?: Project[]; duplicates?: unknown[] }
export interface Finding {
  id?: string; detector_id: string; rule_id: string; severity: Severity
  asset_id: string; asset_type: string; asset_name: string; message: string; evidence: string; remediation: string
}
export interface HealthScore { score: number; band: string; deductions: { asset_name: string; rule_id: string; severity: Severity; points: number }[] }
export interface DetectorStatus { id: string; available: boolean; reason?: string }
export interface EngineInfo { name: string; kind: string; available: boolean; reason?: string }
export interface RuleInfo { id: string; severity: Severity; description: string }
// rules/covers/engines 可为 null:Go 端子进程检测器(gitleaks/govulncheck)规则在外部工具内、
// 或 Covers() 返回 nil(全部资产),nil 切片序列化为 JSON null(非 [])。前端须防御性判空。
export interface DetectorMeta {
  id: string; name: string; engines: EngineInfo[] | null; rules: RuleInfo[] | null; covers: string[] | null
  available: boolean; reason?: string
}
export interface ScanResult {
  findings: Finding[]
  detectors: { id: string; available: boolean; reason?: string; finding_count: number }[]
  health_score?: HealthScore
}
export interface ScanRecord extends ScanResult {
  id: string
  started_at: string
  duration?: number
  inventory?: Inventory
  projects?: Project[]
}

// agent 抽象(对应后端 configengine.Agent)
export interface Agent { id: string; name: string; root_dir: string; claude_json: string }

// 目录树节点(对应后端 configengine.TreeNode)
export interface TreeNode {
  name: string
  path: string
  kind: 'dir' | 'file' | 'synthetic'
  scope?: string
  asset_ids?: string[]
  children?: TreeNode[]
}

// 目录标签(dir tag)响应:GET /api/dir-tags。defaults 为内置默认,overrides 为用户覆盖。
export interface DirTagsResponse {
  defaults: Record<string, 'config' | 'runtime'>
  overrides: Record<string, 'config' | 'runtime'>
}

// /api/raw 响应:单文件原始内容(无资产文件点开时读取)。
export interface RawFile {
  path: string
  name: string
  size: number
  content: string
  is_text: boolean
}

export interface ScanSummary {
  id: string; started_at: string; health_score: number; band: string
  finding_count: number; detector_avail: number; detector_total: number
}
export interface AgentsResponse { agents: Agent[]; current: string }
