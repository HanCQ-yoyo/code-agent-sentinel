package security

import (
	"context"
	"time"

	"code-agent-sentinel/internal/configengine"
)

type Orchestrator struct {
	Registry *Registry
}

// Scan 跑匹配的检测器,聚合结果。detectorIDs 为空则跑全部。
func (o *Orchestrator) Scan(ctx context.Context, assets []configengine.Asset, detectorIDs []string) (*ScanResult, error) {
	want := map[string]bool{}
	for _, id := range detectorIDs {
		want[id] = true
	}
	res := &ScanResult{StartedAt: time.Now()}
	for _, d := range o.Registry.Detectors() {
		if len(want) > 0 && !want[d.ID()] {
			continue
		}
		st := DetectorStatus{ID: d.ID(), Available: d.Available(), Reason: d.Reason()}
		if !d.Available() {
			res.Detectors = append(res.Detectors, st)
			continue
		}
		// 只把 Covers() 声明的资产类型传给它
		in := filterByCovers(assets, d.Covers())
		start := time.Now()
		findings, err := d.Scan(ctx, in)
		st.Duration = time.Since(start)
		if err != nil {
			st.Reason = err.Error()
			res.Detectors = append(res.Detectors, st)
			continue
		}
		st.FindingCount = len(findings)
		res.Findings = append(res.Findings, findings...)
		res.Detectors = append(res.Detectors, st)
	}
	res.HealthScore = ComputeHealth(assets, res.Findings)
	res.Duration = time.Since(res.StartedAt)
	return res, nil
}

func filterByCovers(assets []configengine.Asset, covers []configengine.AssetType) []configengine.Asset {
	if len(covers) == 0 {
		return assets
	}
	set := map[configengine.AssetType]bool{}
	for _, c := range covers {
		set[c] = true
	}
	var out []configengine.Asset
	for _, a := range assets {
		if set[a.Type] {
			out = append(out, a)
		}
	}
	return out
}
