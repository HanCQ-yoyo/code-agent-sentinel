package ruleengine

import (
	"encoding/json"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// makeCommandAsset 构造一条 command 类型资产,供 Eval 等价性验证使用。
func makeCommandAsset(cmd string) configengine.Asset {
	return configengine.Asset{Type: configengine.AssetCommand, Fields: map[string]any{"command": cmd}}
}

// yamlUnmarshalSafe 是测试辅助:避免在测试里直接 import yaml 重复。
func yamlUnmarshalSafe(t *testing.T, data []byte, out *ruleFile) error {
	t.Helper()
	return yamlUnmarshal(data, out)
}

func TestRuleToRowRoundTrip(t *testing.T) {
	// 构造一条含叶子 match + paths + metadata 的规则(经 UnmarshalYAML 路径构造 MatchNode)。
	// ruleFile 顶层需 rules: 段包裹(与 load_test.go 惯例一致)。
	src := []byte(`
rules:
- id: test.roundtrip
  severity: critical
  asset_type: command
  match:
    field: command
    op: contains
    value: "rm -rf /"
  paths:
    include: ["**/*.sh"]
    exclude: ["safe/**"]
  post_exclude: ["echo.*"]
  deobfuscation: ["hex"]
  dotall: true
  remediation: "don't"
  description: "rm root"
  metadata:
    domain: filesystem
    rule_id: filesystem.rm-rf-root
`)
	var rf ruleFile
	if err := yamlUnmarshalSafe(t, src, &rf); err != nil {
		t.Fatalf("parse: %v", err)
	}
	orig := rf.Rules[0]

	row, err := RuleToRow(orig)
	if err != nil {
		t.Fatalf("RuleToRow: %v", err)
	}
	if row.ID != "test.roundtrip" || row.Severity != "critical" {
		t.Fatalf("row = %#v", row)
	}
	// match_json 应是可序列化 map
	var matchMap map[string]any
	if err := json.Unmarshal([]byte(row.MatchJSON), &matchMap); err != nil {
		t.Fatalf("match_json not valid json: %v", err)
	}
	if matchMap["field"] != "command" {
		t.Fatalf("match_json = %#v", matchMap)
	}

	restored, err := RowToRule(row)
	if err != nil {
		t.Fatalf("RowToRule: %v", err)
	}
	if restored.ID != orig.ID || restored.Severity != orig.Severity {
		t.Fatalf("restored id/sev = %q/%q", restored.ID, restored.Severity)
	}
	if restored.AssetType != orig.AssetType {
		t.Fatalf("restored asset_type = %q, want %q", restored.AssetType, orig.AssetType)
	}
	if restored.Dotall != orig.Dotall {
		t.Fatalf("restored dotall = %v, want %v", restored.Dotall, orig.Dotall)
	}
	if restored.Paths == nil || len(restored.Paths.Include) != 1 {
		t.Fatalf("restored paths = %#v", restored.Paths)
	}
	if restored.Metadata["domain"] != "filesystem" {
		t.Fatalf("restored metadata = %#v", restored.Metadata)
	}
	// 关键:往返后 Eval 行为等价(用一条 command 资产验证)
	asset := makeCommandAsset("rm -rf /")
	rOrig, _ := Validate([]Rule{orig})
	rRest, _ := Validate([]Rule{restored})
	if Eval(rOrig[0], asset).Matched != Eval(rRest[0], asset).Matched {
		t.Fatal("Eval behavior differs after roundtrip")
	}
}

func TestComboRowRoundTrip(t *testing.T) {
	cr := ComboRule{
		ID:          "combo.test",
		Severity:    "high",
		Description: "d",
		Remediation: "r",
		Metadata:    map[string]any{"domain": "x"},
		Requires: []ComboCondition{
			{AssetType: "hook", Match: NewMatchNode(map[string]any{"field": "command", "op": "contains", "value": "rm"})},
		},
	}
	row, err := ComboToRow(cr)
	if err != nil {
		t.Fatalf("ComboToRow: %v", err)
	}
	if row.ID != "combo.test" {
		t.Fatalf("row = %#v", row)
	}
	restored, err := RowToCombo(row)
	if err != nil {
		t.Fatalf("RowToCombo: %v", err)
	}
	if restored.ID != cr.ID || len(restored.Requires) != 1 {
		t.Fatalf("restored = %#v", restored)
	}
}
