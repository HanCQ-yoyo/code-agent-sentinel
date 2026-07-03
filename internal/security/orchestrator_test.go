package security

import (
	"context"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestOrchestratorScan(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeDetector{id: "fake", avail: true})
	r.Register(fakeDetector{id: "off", avail: false})
	o := &Orchestrator{Registry: r}
	assets := []configengine.Asset{{ID: "x", Type: configengine.AssetHook}}
	res, err := o.Scan(context.Background(), assets, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 {
		t.Errorf("findings: %d", len(res.Findings))
	}
	// off 不可用:不出 finding,但 status 记录 unavailable
	offOK := false
	for _, s := range res.Detectors {
		if s.ID == "off" && !s.Available {
			offOK = true
		}
	}
	if !offOK {
		t.Error("off 检测器应标记 unavailable")
	}
}

func TestOrchestratorSelectiveDetectors(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeDetector{id: "a", avail: true})
	r.Register(fakeDetector{id: "b", avail: true})
	o := &Orchestrator{Registry: r}
	res, _ := o.Scan(context.Background(), nil, []string{"a"})
	if len(res.Detectors) != 1 || res.Detectors[0].ID != "a" {
		t.Errorf("应只跑 a: %+v", res.Detectors)
	}
}

// TestOrchestratorExposesParseErrorAsFinding 验证 C-CORR-2:资产带 ParseError 时,
// 编排器应在算分前兜底产一条 parse.error finding(规格:解析失败必须作为 Finding
// 暴露,否则健康分与 findings 列表都不反映损坏资产)。并验证它拉低健康分。
func TestOrchestratorExposesParseErrorAsFinding(t *testing.T) {
	// 空注册表:不跑任何检测器,确保唯一 finding 来自 parse_error 兜底。
	r := NewRegistry()
	o := &Orchestrator{Registry: r}
	bad := configengine.Asset{ID: "bad", Type: configengine.AssetSettings, Name: "settings", ParseError: "invalid JSON at line 1"}
	res, err := o.Scan(context.Background(), []configengine.Asset{bad}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var found *Finding
	for i := range res.Findings {
		if res.Findings[i].RuleID == "parse.error" {
			found = &res.Findings[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("期望 parse.error finding,实际 %+v", res.Findings)
	}
	if found.DetectorID != "orchestrator" {
		t.Errorf("DetectorID = %q, want orchestrator", found.DetectorID)
	}
	if found.Severity != SeverityMedium {
		t.Errorf("parse.error 应 Medium,实际 %s", found.Severity)
	}
	if found.AssetName != "settings" {
		t.Errorf("AssetName = %q", found.AssetName)
	}
	if found.Evidence != "invalid JSON at line 1" {
		t.Errorf("Evidence = %q", found.Evidence)
	}
	// 同一资产无 ParseError 时无 finding、分数 100;有 ParseError 时分数更低。
	good := configengine.Asset{ID: "bad", Type: configengine.AssetSettings, Name: "settings"}
	resGood, _ := o.Scan(context.Background(), []configengine.Asset{good}, nil)
	if res.HealthScore.Score >= resGood.HealthScore.Score {
		t.Errorf("ParseError 应拉低分数:有=%d 无=%d", res.HealthScore.Score, resGood.HealthScore.Score)
	}
}
