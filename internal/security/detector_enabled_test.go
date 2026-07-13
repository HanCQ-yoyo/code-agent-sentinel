package security

import (
	"context"
	"testing"

	"code-agent-sentinel/internal/config"
)

// TestOrchestratorSkipsDisabledDetector 禁用的检测器:不跑、不出 finding、status 标 Disabled。
func TestOrchestratorSkipsDisabledDetector(t *testing.T) {
	cfg := &config.DetectorsConfig{Rules: config.DetectorToggle{Enabled: false}}
	r := NewRegistry()
	r.Register(fakeDetector{id: "fake", avail: true}) // fake 始终 enabled
	// 用 rules 检测器作为可禁用对象(其 Enabled 读 cfg.Rules.Enabled)。
	rd := NewRulesDetector(t.TempDir(), cfg)
	r.Register(rd)
	o := &Orchestrator{Registry: r}

	res, err := o.Scan(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// rules 被 disabled:不出 finding;fake 出 1 条。
	if len(res.Findings) != 1 {
		t.Errorf("findings = %d, want 1(rules 被禁用,只 fake 出 1)", len(res.Findings))
	}
	var rulesStat *DetectorStatus
	for i := range res.Detectors {
		if res.Detectors[i].ID == "rules" {
			rulesStat = &res.Detectors[i]
		}
	}
	if rulesStat == nil {
		t.Fatal("缺 rules status")
	}
	if !rulesStat.Disabled {
		t.Error("rules 应标记 Disabled")
	}
}

// TestSecretDetectorConfig 禁用 → Enabled false;自定义二进制反映在 Available/Meta。
func TestSecretDetectorConfig(t *testing.T) {
	// 禁用
	disabled := &config.DetectorsConfig{Secret: config.BinaryDetectorConfig{Enabled: false}}
	d := NewSecretDetector(disabled)
	if d.Enabled() {
		t.Error("禁用时 Enabled 应 false")
	}
	if d.Available() {
		// 禁用时 Available 无意义,但实现不应 panic;允许 true/false,只要 Enabled=false 即跳过。
		t.Log("禁用时 Available=", d.Available())
	}
	// 自定义二进制(不存在)→ Enabled true、Available false
	custom := &config.DetectorsConfig{Secret: config.BinaryDetectorConfig{Enabled: true, Binary: "/nonexistent/gitleaks"}}
	d2 := NewSecretDetector(custom)
	if !d2.Enabled() {
		t.Error("启用时 Enabled 应 true")
	}
	if d2.Available() {
		t.Error("不存在的二进制 Available 应 false")
	}
	m := d2.Meta()
	if m.Engines[0].Available != d2.Available() {
		t.Errorf("Meta engine available 应反映 Available(): got %v, want %v", m.Engines[0].Available, d2.Available())
	}
}

// TestDepDetectorEngineLevelDisable dep 检测器级启用,但 npm 引擎禁用:只跑启用的引擎。
func TestDepDetectorEngineLevelDisable(t *testing.T) {
	cfg := &config.DetectorsConfig{Dep: config.DepDetectorConfig{
		Enabled: true,
		Engines: map[string]config.BinaryDetectorConfig{"npm": {Enabled: false}},
	}}
	d := NewDependencyDetector(cfg)
	if !d.Enabled() {
		t.Error("dep 检测器级应启用")
	}
	if d.DepEngineEnabled("npm") {
		t.Error("npm 引擎应禁用")
	}
	if !d.DepEngineEnabled("govulncheck") {
		t.Error("govulncheck 引擎应启用(未配置=默认启用)")
	}
}
