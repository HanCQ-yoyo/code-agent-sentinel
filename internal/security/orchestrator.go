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
	// 规格要求:资产解析失败(parse_error)必须作为 Finding 暴露,否则健康分不反映
	// 损坏资产、findings 列表也不显示。configengine 在文件损坏时给资产打 ParseError
	// (见 types.go),但检测器只跑 Covers() 匹配的类型,不会主动检查该字段。故由编排器
	// 在算分前统一兜底:把 ParseError 非空的资产转成一条 finding。
	// 字段选择:
	//   DetectorID="orchestrator" —— 由编排器发出,非注册检测器;
	//   RuleID="parse.error";
	//   Severity=Medium —— 解析失败是"无法评估"的盲区,非已确认漏洞(不够 High/Critical),
	//     但确实影响安全可见性(不够 Low),Medium 居中且与需人工复核的风险同级;
	//   Evidence=原始 ParseError 文本;Message/Remediation 给出可读说明。
	for _, a := range assets {
		if a.ParseError == "" {
			continue
		}
		res.Findings = append(res.Findings, Finding{
			DetectorID:  "orchestrator",
			RuleID:      "parse.error",
			Severity:    SeverityMedium,
			AssetID:     a.ID,
			AssetType:   a.Type,
			AssetName:   a.Name,
			Message:     "资产解析失败,无法评估",
			Evidence:    a.ParseError,
			Remediation: "修复或移除该配置文件的语法错误",
		})
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
