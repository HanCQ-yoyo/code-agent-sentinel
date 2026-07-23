package security

import (
	"context"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// destructive_e2e_test.go — Task 12 端到端集成测试。
//
// 与 ruleengine/destructive_test.go(单元层:直接调 ruleengine.Eval)的区别:
// 本测试在 security 包跑完整 RulesDetector.Scan(含语义两道关卡 + post_exclude + 抑制)+
// ComputeHealth(健康分扣分),验证 destructive 规则在真实检测器管道中端到端生效。
//
// 资产直接构造(无需 fixture 文件系统):hook 走 Fields["command"],
// script 走 Content(与 RulesDetector.commandTextFromAsset 路由对齐)。

// TestDestructive_EndToEnd 验证真实破坏性命令经完整检测管道产出 destructive.* finding
// 并拉低健康分(< 100)。
//
// 覆盖:
//   - hook 含 `rm -rf /` → 语义 Deny(filesystem)兜底,产 semantic.filesystem.rm-rf-root-home
//     (critical,载体规则按 dcg_rule_id 精确匹配 rm-rf-root-home)。
//   - hook 含 `git push -f origin main` → 语义 Deny(git)产 semantic.git.push-force-short
//     (critical,载体规则 destructive.git.push-force-short)。
//
// 命中域应覆盖 filesystem + git(两类危险命令各产至少一条 semantic.* finding)。
// 健康分应 < 100(critical finding 系数 4.0,hook 权重 3.0,必扣分)。
//
// 注意:destructive 规则 asset_type=hook(command 字段),故测试用 hook 资产而非 script。
// script 走 content 字段,但 destructive 规则只声明 asset_type=hook,script 资产不匹配
// (RulesDetector 按 r.AssetType 路由)。这是忠实 dcg 源的设计:destructive 规则针对
// 命令行(hook command / mcp_server command),而非脚本正文。
func TestDestructive_EndToEnd(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)

	assets := []configengine.Asset{
		{
			ID:   "hook-rm-root",
			Type: configengine.AssetHook,
			Name: "hook",
			Fields: map[string]any{
				"command": "rm -rf /",
			},
		},
		{
			ID:   "hook-push-force",
			Type: configengine.AssetHook,
			Name: "hook",
			Fields: map[string]any{
				"command": "git push -f origin main",
			},
		},
	}

	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	// 应命中 filesystem 域(rm -rf /) + git 域(git push -f)。
	// 命中来源可能是 destructive.* 正则或 semantic.* 语义兜底(均含域段)。
	hitDomains := map[string]bool{}
	for _, f := range findings {
		rid := f.RuleID
		// destructive.filesystem.* / semantic.filesystem.* / destructive.git.* / semantic.git.*
		if strings.HasPrefix(rid, "destructive.") || strings.HasPrefix(rid, "semantic.") {
			// RuleID 形如 destructive.<domain>.<pattern> 或 semantic.<domain>.<pattern>
			parts := strings.Split(rid, ".")
			if len(parts) >= 3 {
				// parts[0]=destructive|semantic, parts[1]=domain(git|filesystem|database|...),
				// parts[2+]=子域/规则名。
				// 语义 RuleID 是 semantic.<dcgdomain>.<pattern>(如 semantic.filesystem.rm-rf-root-home),
				// dcg domain 段已映射到 sentinel domain(见 rules_detector.go semDenyRuleDomain)。
				hitDomains[parts[1]] = true
			}
		}
	}
	if !hitDomains["filesystem"] {
		t.Errorf("expected filesystem domain finding (rm -rf /), hitDomains=%v findings=%+v", hitDomains, findings)
	}
	if !hitDomains["git"] {
		t.Errorf("expected git domain finding (git push -f), hitDomains=%v findings=%+v", hitDomains, findings)
	}

	// 健康分应因这些 finding 扣分(< 100)。
	health := ComputeHealth(assets, findings)
	if health.Score >= 100 {
		t.Errorf("health score should be < 100 with destructive findings, got %d (deductions=%d)",
			health.Score, len(health.Deductions))
	}
	t.Logf("EndToEnd: %d findings, hitDomains=%v, health=%d", len(findings), hitDomains, health.Score)
}

// TestDestructive_NoFalsePositiveOnSafeAssets 验证安全命令不触发 destructive 规则。
//
// 覆盖:
//   - `git commit -m 'deploy'` → git 语义判 Safe(commit 非破坏子命令),正则不命中。
//   - `rm -i temp` → rm 语义判 Safe(interactive 用户确认),正则 rm-* 不匹配(无 -rf 聚簇)。
//   - `docker ps` → containers 域无破坏标志,正则不命中(无语义解析器,走纯正则)。
//
// 期望:零 destructive.* / semantic.* finding。
func TestDestructive_NoFalsePositiveOnSafeAssets(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil)

	assets := []configengine.Asset{
		{
			ID:   "hook-safe-commit",
			Type: configengine.AssetHook,
			Name: "hook",
			Fields: map[string]any{
				"command": "git commit -m 'deploy'",
			},
		},
		{
			ID:   "hook-safe-rm-i",
			Type: configengine.AssetHook,
			Name: "hook",
			Fields: map[string]any{
				"command": "rm -i temp",
			},
		},
		{
			ID:   "hook-safe-docker-ps",
			Type: configengine.AssetHook,
			Name: "hook",
			Fields: map[string]any{
				"command": "docker ps",
			},
		},
	}

	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	for _, f := range findings {
		if strings.HasPrefix(f.RuleID, "destructive.") || strings.HasPrefix(f.RuleID, "semantic.") {
			t.Errorf("safe asset should not trigger destructive/semantic rule: %s (asset=%s) evidence=%s",
				f.RuleID, f.AssetID, f.Evidence)
		}
	}
	t.Logf("NoFalsePositive: %d findings (all non-destructive)", len(findings))
}
