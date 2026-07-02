package security

import (
	"time"

	"code-agent-sentinel/internal/configengine"
)

// Severity 表示检测结果的严重等级。
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
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
}

// DetectorStatus 是一个检测器的运行状态。
type DetectorStatus struct {
	ID           string        `json:"id"`
	Available    bool          `json:"available"`
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
