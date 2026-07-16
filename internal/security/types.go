package security

import (
	"time"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security/ruleengine"
)

// Severity 表示检测结果的严重等级。
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info" // 低置信度 finding,系数 0.0 不影响健康分
)

// Finding 是一条检测结果。
type Finding struct {
	ID          string                 `json:"id"`
	DetectorID  string                 `json:"detector_id"`
	RuleID      string                 `json:"rule_id"`
	Severity    Severity               `json:"severity"`
	AssetID     string                 `json:"asset_id"`
	AssetType   configengine.AssetType `json:"asset_type"`
	AssetName   string                 `json:"asset_name"`
	Message     string                 `json:"message"`
	Evidence    string                 `json:"evidence"`
	Remediation string                 `json:"remediation"`
	Fingerprint string                 `json:"fingerprint,omitempty"` // 规则指纹(baseline/inline 抑制用);仅 RulesDetector 填充
	// Locations 是 content 字段命中的文件位置(仅 RulesDetector 填充;子进程检测器无)。
	// 供 UI(Task 18)在 Monaco 高亮命中行;不参与健康分,不进 Fingerprint。
	Locations   []ruleengine.Location `json:"locations,omitempty"`
	Suppressed  bool                  `json:"suppressed,omitempty"`
	Suppression string                `json:"suppression,omitempty"` // "baseline" / "inline"
	Reason      string                `json:"reason,omitempty"`
}

// DetectorStatus 是一个检测器的运行状态。
type DetectorStatus struct {
	ID           string        `json:"id"`
	Enabled      bool          `json:"enabled"`
	Available    bool          `json:"available"`
	Disabled     bool          `json:"disabled,omitempty"` // 用户禁用(Enabled=false)
	Reason       string        `json:"reason,omitempty"`
	FindingCount int           `json:"finding_count"`
	Duration     time.Duration `json:"duration"`
}

// ScanResult 是一次扫描的聚合结果。
type ScanResult struct {
	Findings    []Finding        `json:"findings"`
	Detectors   []DetectorStatus `json:"detectors"`
	StartedAt   time.Time        `json:"started_at"`
	Duration    time.Duration    `json:"duration"`
	HealthScore *HealthScore     `json:"health_score,omitempty"`
}

// HealthScore 是健康分结果。
type HealthScore struct {
	Score      int         `json:"score"`
	Band       string      `json:"band"`
	Rmax       float64     `json:"rmax"`
	Deductions []Deduction `json:"deductions"`
}

// Deduction 是一条可解释扣分。
type Deduction struct {
	AssetID   string   `json:"asset_id"`
	AssetType string   `json:"asset_type"`
	AssetName string   `json:"asset_name"`
	RuleID    string   `json:"rule_id"`
	Severity  Severity `json:"severity"`
	Points    float64  `json:"points"`
}
