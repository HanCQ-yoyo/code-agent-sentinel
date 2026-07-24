package security

import (
	"context"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// final_review_fix_test.go — 最终跨任务 review 修的两个 CRITICAL 回归测试。
//
// 这两个 bug 都是 per-task review 漏掉的(因为 per-task 测试在隔离环境里用 Safe-producing
// 命令,从没把它和真正的破坏性命令放同一 content 里):
//   - C1:语义 Safe 决策按 asset 整体抑制,会漏报同一 content 其它行的真实破坏性命令。
//   - C2:snowflake 语义 Deny 的 RuleID="snowflake.drop"(通用)无对应 dcg_rule_id,
//     pickSemanticCarrier 回退到首条 database 规则(mongodb.stdin-unverified high),
//     severity 被 high 覆盖,且 Gate 1 continue 抑制了正确的 critical 正则规则。

// hasFindingSeverity 返回是否存在以 ruleIDPrefix 开头且 severity == want 的 finding。
func hasFindingSeverity(fs []Finding, ruleIDPrefix string, want Severity) bool {
	for _, f := range fs {
		if strings.HasPrefix(f.RuleID, ruleIDPrefix) && f.Severity == want {
			return true
		}
	}
	return false
}

// ── C1:语义 Safe 必须按行 span-scoping,不能跨行抑制真实破坏性命令 ──

// TestC1_SafeDoesNotSuppressDestructiveOnOtherLine 验证:
// `git commit -m 'rm -rf /'\nrm -rf /` —— 第 1 行 git commit -m 数据区内的 rm -rf 是
// 字面量(Safe),但第 2 行的 rm -rf / 是真实破坏性命令。修前:语义缓存对整个 content
// 跑一次 DispatchCommand,git commit -m 返回 Safe → Gate 1 continue + Gate 2 丢弃,
// 整个 asset 的所有 filesystem 规则被抑制 → rm -rf / 漏报(健康分 60,0 条 filesystem finding)。
//
// 修后:语义按行跑,第 1 行 Safe 仅抑制第 1 行内的正则命中;第 2 行 rm -rf / 的正则命中
// 不在 Safe 行集合内 → 保留,destructive.filesystem.* (critical) finding 仍触发。
func TestC1_SafeDoesNotSuppressDestructiveOnOtherLine(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{{
		ID:      "script-mixed",
		Type:    configengine.AssetScript,
		Name:    "script",
		Content: "git commit -m 'rm -rf /'\nrm -rf /",
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// 第 2 行 rm -rf / 必须仍触发 destructive.filesystem.*(正则)或 semantic.filesystem.*
	// (语义 Deny 兜底)。且 severity 必须是 critical(rm -rf / 是 rm-rf-root-home critical)。
	if !hasFindingSeverity(findings, "destructive.filesystem.", SeverityCritical) &&
		!hasFindingSeverity(findings, "semantic.filesystem.", SeverityCritical) {
		t.Errorf("C1 回归:git commit -m Safe 不应抑制第 2 行的 rm -rf /,但无 critical filesystem finding: %+v", findings)
	}
}

// TestC1_RmInteractiveSafeDoesNotSuppressRmRf 验证 `rm -i tmp\nrm -rf /`:
// 第 1 行 rm -i 是 interactive(Safe),第 2 行 rm -rf / 真实破坏性。修前 Safe 跨行抑制漏报。
func TestC1_RmInteractiveSafeDoesNotSuppressRmRf(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{{
		ID:      "script-rmi-then-rmrf",
		Type:    configengine.AssetScript,
		Name:    "script",
		Content: "rm -i tmp\nrm -rf /",
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if !hasFindingSeverity(findings, "destructive.filesystem.", SeverityCritical) &&
		!hasFindingSeverity(findings, "semantic.filesystem.", SeverityCritical) {
		t.Errorf("C1 回归:rm -i Safe 不应抑制第 2 行 rm -rf /,但无 critical filesystem finding: %+v", findings)
	}
}

// TestC1_GitRestoreStagedSafeDoesNotSuppressRmRf 验证 `git restore --staged file\nrm -rf /`:
// 第 1 行 git restore --staged 是 Safe(仅影响索引),第 2 行 rm -rf / 真实破坏性。
func TestC1_GitRestoreStagedSafeDoesNotSuppressRmRf(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{{
		ID:      "script-restore-then-rmrf",
		Type:    configengine.AssetScript,
		Name:    "script",
		Content: "git restore --staged file\nrm -rf /",
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if !hasFindingSeverity(findings, "destructive.filesystem.", SeverityCritical) &&
		!hasFindingSeverity(findings, "semantic.filesystem.", SeverityCritical) {
		t.Errorf("C1 回归:git restore --staged Safe 不应抑制第 2 行 rm -rf /,但无 critical filesystem finding: %+v", findings)
	}
}

// TestC1_SkillAssetSafeOnOtherLine 验证 skill/memory 资产同样不被跨行 Safe 漏报。
func TestC1_SkillAssetSafeOnOtherLine(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{
		{
			ID:      "skill-mixed",
			Type:    configengine.AssetSkill,
			Name:    "skill",
			Content: "git commit -m 'rm -rf /'\nrm -rf /",
		},
		{
			ID:      "memory-mixed",
			Type:    configengine.AssetMemory,
			Name:    "memory",
			Content: "rm -i tmp\nrm -rf /",
		},
	}
	for _, a := range assets {
		findings, err := d.Scan(context.Background(), []configengine.Asset{a})
		if err != nil {
			t.Fatalf("Scan(%s) error: %v", a.ID, err)
		}
		if !hasFindingSeverity(findings, "destructive.filesystem.", SeverityCritical) &&
			!hasFindingSeverity(findings, "semantic.filesystem.", SeverityCritical) {
			t.Errorf("C1 回归:%s 内 Safe 不应抑制另一行的 rm -rf /,但无 critical filesystem finding: %+v", a.ID, findings)
		}
	}
}

// TestC1_SingleLineSafeStillSuppressedWithinLine 验证修复未过抑制:
// 单行 `git commit -m "rm -rf /"` 的 rm -rf 在 -m 数据区是字面量,语义 Safe 仍应抑制
// 该行内的正则命中(不误报)。这是 TestRulesDetector_SemanticNoFalsePositive 的等价
// 守卫,确保按行 span-scoping 后单行 Safe 仍正常生效。
func TestC1_SingleLineSafeStillSuppressedWithinLine(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{{
		ID:      "script-single-safe",
		Type:    configengine.AssetScript,
		Name:    "script",
		Content: `git commit -m "rm -rf /"`,
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	for _, f := range findings {
		if strings.HasPrefix(f.RuleID, "destructive.filesystem.") {
			t.Errorf("C1 回归:单行 git commit -m 数据区内的 rm -rf 仍应被 Safe 抑制,但 got %s: %+v", f.RuleID, f)
		}
	}
}

// ── C2:snowflake 语义 Deny 必须返回具体 dcg_rule_id,继承正确 severity ──

// TestC2_SnowflakeDropDatabaseCritical 验证 `snow sql --query 'DROP DATABASE d'`:
// 语义 Deny 应返回 RuleID="snowflake.drop-database"(精确匹配 YAML 的 dcg_rule_id),
// pickSemanticCarrier strategy 1 命中 destructive.database.snowflake.drop-database(critical),
// semantic finding severity = critical(不是 high)。
// 修前:返回通用 "snowflake.drop",strategy 1 miss → 回退到 strategy 2 = 首条 database
// 规则 mongodb.stdin-unverified(high),severity 被扭曲为 high,且 Gate 1 continue 抑制
// 了正确的 critical 正则规则。
func TestC2_SnowflakeDropDatabaseCritical(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{{
		ID:   "hook-snow-dropdb",
		Type: configengine.AssetHook,
		Name: "hook",
		Fields: map[string]any{
			"command": "snow sql --query 'DROP DATABASE d'",
		},
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// 语义 finding 应是 semantic.snowflake.drop-database,severity critical。
	found := false
	for _, f := range findings {
		if f.RuleID == "semantic.snowflake.drop-database" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("C2:semantic.snowflake.drop-database severity = %s, want critical (载体规则应按 dcg_rule_id 精确匹配 destructive.database.snowflake.drop-database): %+v", f.Severity, f)
			}
		}
	}
	if !found {
		t.Errorf("C2:期望 semantic.snowflake.drop-database finding,但无: %+v", findings)
	}
}

// TestC2_SnowflakeDropTableCritical 验证 `snow sql --query 'DROP TABLE t'`:
// 语义 RuleID="snowflake.drop-table",severity critical。
func TestC2_SnowflakeDropTableCritical(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{{
		ID:   "hook-snow-droptable",
		Type: configengine.AssetHook,
		Name: "hook",
		Fields: map[string]any{
			"command": "snow sql --query 'DROP TABLE t'",
		},
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "semantic.snowflake.drop-table" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("C2:semantic.snowflake.drop-table severity = %s, want critical: %+v", f.Severity, f)
			}
		}
	}
	if !found {
		t.Errorf("C2:期望 semantic.snowflake.drop-table finding,但无: %+v", findings)
	}
}

// TestC2_SnowflakeTruncateCritical 验证 `snow sql --query 'TRUNCATE TABLE t'`:
// 语义 RuleID="snowflake.truncate-table",severity critical。
func TestC2_SnowflakeTruncateCritical(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{{
		ID:   "hook-snow-truncate",
		Type: configengine.AssetHook,
		Name: "hook",
		Fields: map[string]any{
			"command": "snow sql --query 'TRUNCATE TABLE t'",
		},
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "semantic.snowflake.truncate-table" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("C2:semantic.snowflake.truncate-table severity = %s, want critical: %+v", f.Severity, f)
			}
		}
	}
	if !found {
		t.Errorf("C2:期望 semantic.snowflake.truncate-table finding,但无: %+v", findings)
	}
}

// TestC2_SnowflakeDropSchemaCritical 验证 `snow sql --query 'DROP SCHEMA s'`:
// 语义 RuleID="snowflake.drop-schema",severity critical。
func TestC2_SnowflakeDropSchemaCritical(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)
	assets := []configengine.Asset{{
		ID:   "hook-snow-dropschema",
		Type: configengine.AssetHook,
		Name: "hook",
		Fields: map[string]any{
			"command": "snow sql --query 'DROP SCHEMA s'",
		},
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "semantic.snowflake.drop-schema" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("C2:semantic.snowflake.drop-schema severity = %s, want critical: %+v", f.Severity, f)
			}
		}
	}
	if !found {
		t.Errorf("C2:期望 semantic.snowflake.drop-schema finding,但无: %+v", findings)
	}
}
