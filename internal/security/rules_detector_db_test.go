package security

import (
	"context"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security/ruleengine"
	"code-agent-sentinel/internal/storage"
)

// rules_detector_db_test.go — Task 8 RulesDetector 持 db 实时读测试。
//
// 验证两条:
//  1. TestRulesDetectorScanFromDB:RulesDetector 持 db 句柄,Scan 从 db 读 builtin 规则,
//     对 rm -rf / 产 finding(规则来自 db 而非文件 LoadForScan)。
//  2. TestRulesDetectorHotReloadAfterDBChange:API 改规则(SetOverride 禁用某规则)后,
//     下次 Scan 实时反映(热重载)——不重启、不重新构造 RulesDetector,finding 减少。

// newDBWithBuiltinForDetector 开一个临时 db,跑迁移,并把真实 embed builtin 规则经
// 适配层同步进 detect 域(rules + combos)。复用 ruleengine 包测试的 newDBWithBuiltin 思路,
// 但本测试在 package security(非 ruleengine),不能调 ruleengine 的未导出 rulesToStored,
// 故用导出的 RuleToStoredRule / ComboToStoredCombo 循环转换。
func newDBWithBuiltinForDetector(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "det.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	builtin, builtinCombos, _ := ruleengine.LoadBuiltin()
	stored, err := ruleengineRuleToStored(builtin, "v1")
	if err != nil {
		t.Fatal(err)
	}
	storedCombos, err := ruleengineCombosToStored(builtinCombos, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.SyncBuiltin(db, storage.DomainDetect, stored, storedCombos, "v1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ruleengineRuleToStored 把 []ruleengine.Rule 转成 []storage.StoredRule(SyncBuiltin 输入)。
// ruleengine 包内的未导出 rulesToStored 不可跨包用,这里用导出的 RuleToStoredRule 循环。
func ruleengineRuleToStored(rules []ruleengine.Rule, v string) ([]storage.StoredRule, error) {
	out := make([]storage.StoredRule, 0, len(rules))
	for _, r := range rules {
		s, err := ruleengine.RuleToStoredRule(r, "builtin", v)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// ruleengineCombosToStored 把 []ruleengine.ComboRule 转成 []storage.StoredCombo(SyncBuiltin 输入)。
func ruleengineCombosToStored(combos []ruleengine.ComboRule, v string) ([]storage.StoredCombo, error) {
	out := make([]storage.StoredCombo, 0, len(combos))
	for _, c := range combos {
		s, err := ruleengine.ComboToStoredCombo(c, "builtin", v)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// TestRulesDetectorScanFromDB 验证 RulesDetector 持 db 后 Scan 从 db 读规则。
// 资产是 rm -rf /(AssetCommand,command 字段),destructive.filesystem.* 规则经
// ruleAppliesToAsset 放宽路由对 command-bearing 类型生效,应产 finding。
func TestRulesDetectorScanFromDB(t *testing.T) {
	db := newDBWithBuiltinForDetector(t)
	d := NewRulesDetector("", config.DefaultConfig().Detectors, db)
	assets := []configengine.Asset{
		{ID: "cmd1", Type: configengine.AssetCommand, Fields: map[string]any{"command": "rm -rf /"}},
	}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected findings for rm -rf /")
	}
}

// TestRulesDetectorHotReloadAfterDBChange 验证热重载:同一 RulesDetector 实例,
// API 改规则(SetOverride 禁用命中的 builtin 规则)后,下次 Scan 实时反映(finding 减少)。
// 不重新构造检测器,证明 Scan 每次实时读 db。
func TestRulesDetectorHotReloadAfterDBChange(t *testing.T) {
	db := newDBWithBuiltinForDetector(t)
	d := NewRulesDetector("", config.DefaultConfig().Detectors, db)
	assets := []configengine.Asset{
		{ID: "cmd1", Type: configengine.AssetCommand, Fields: map[string]any{"command": "rm -rf /"}},
	}
	// 第一次扫描:有 finding
	before, _ := d.Scan(context.Background(), assets)
	beforeCount := len(before)

	// 禁用命中的 builtin 规则(取首条 finding 的 rule_id,设 override=false)
	if beforeCount > 0 {
		rid := before[0].RuleID
		// RuleID 可能是 semantic.* 前缀,取真实规则 id
		if len(rid) > 9 && rid[:9] == "semantic." {
			rid = rid[9:]
		}
		if err := storage.SetOverride(db, storage.DomainDetect, rid, false); err != nil {
			t.Fatal(err)
		}
	}
	// 第二次扫描:热重载,该规则不再命中 → findings 减少
	after, _ := d.Scan(context.Background(), assets)
	if len(after) >= beforeCount {
		t.Fatalf("hot reload failed: before=%d after=%d (expected after < before)", beforeCount, len(after))
	}
}
