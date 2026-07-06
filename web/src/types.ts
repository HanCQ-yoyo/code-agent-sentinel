export type Severity = 'critical' | 'high' | 'medium' | 'low'
export interface Asset { id: string; type: string; scope: string; source_path: string; name: string; fields?: Record<string, unknown>; content?: string; hash: string; mtime?: string; parse_error?: string }
export interface Inventory { assets: Asset[]; project?: { path: string; name: string }; duplicates?: unknown[] }
export interface Finding { id?: string; detector_id: string; rule_id: string; severity: Severity; asset_id: string; asset_type: string; asset_name: string; message: string; evidence: string; remediation: string }
export interface HealthScore { score: number; band: string; deductions: { asset_name: string; rule_id: string; severity: Severity; points: number }[] }
export interface ScanResult { findings: Finding[]; detectors: { id: string; available: boolean; reason?: string; finding_count: number }[]; health_score?: HealthScore }
export interface DetectorStatus { id: string; available: boolean; reason?: string }
