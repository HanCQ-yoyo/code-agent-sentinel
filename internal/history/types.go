// internal/history/types.go
package history

import (
	"time"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
)

// ScanRecord 是一次完整扫描的持久化记录(含资产快照)。
type ScanRecord struct {
	ID          string                    `json:"id"` // 时间戳+8hex,与文件名一致
	StartedAt   time.Time                 `json:"started_at"`
	Duration    time.Duration             `json:"duration"`
	Findings    []security.Finding        `json:"findings"`
	Detectors   []security.DetectorStatus `json:"detectors"`
	HealthScore *security.HealthScore     `json:"health_score,omitempty"`
	Inventory   *configengine.Inventory   `json:"inventory"`
	Project     *configengine.Project     `json:"project,omitempty"`
}

// ScanSummary 是列表用的轻量摘要,不含 findings/assets。
type ScanSummary struct {
	ID            string    `json:"id"`
	StartedAt     time.Time `json:"started_at"`
	HealthScore   int       `json:"health_score"`
	Band          string    `json:"band"`
	FindingCount  int       `json:"finding_count"`
	DetectorAvail int       `json:"detector_avail"`
	DetectorTotal int       `json:"detector_total"`
}
