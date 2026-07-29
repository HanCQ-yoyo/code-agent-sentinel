package security

import (
	"context"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// rules_detector_span_test.go — Task 9 静态层同治(span 分类 + 片段拆分)测试。
//
// 验证静态层 RulesDetector 升级后与运行时守卫一致(spec §5):
//   - I1 静态闭合:行内 `&& rm -rf /` 经 SplitCommand 拆片段,rm -rf / 独立判语义 → Deny。
//     修前:按行整体跑 DispatchCommand,git commit -m "x" && rm -rf / 整体判 Safe
//     (git commit -m 数据区优先),行内 rm -rf / 漏报(I1)。
//   - span data 区误报消除:echo "rm -rf /" 引号内 rm -rf / 是字面量,ClassifySpans
//     标记为 SpanData,语义 Deny + 正则命中都应丢弃(spec §6.5 行为变更)。

// TestStaticLayerChainSplitFindsRmRf 验证 I1 静态闭合:
// `git commit -m "x" && rm -rf /` 经 SplitCommand 拆成 2 片段,
// 第 2 片段 `rm -rf /` 独立判语义 → Deny,产 semantic.filesystem.* finding(critical)。
//
// 修前(R2):按行整体跑 DispatchCommand,git commit -m 优先返回 Safe,
// 行内 `&& rm -rf /` 被整体 Safe 漏报(I1 链式绕过)。
// 修后(R3):SplitCommand 拆片段,rm -rf / 独立 Deny → 闭合 I1。
//
// 用 AssetScript + Content(走 content 字段,触发 destructive.filesystem.* 正则 +
// filesystem 语义解析器)。R2 下整体 Safe 会抑制该行正则,且无 semantic finding。
func TestStaticLayerChainSplitFindsRmRf(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil, nil)
	assets := []configengine.Asset{{
		ID:      "script-chain",
		Type:    configengine.AssetScript,
		Name:    "script",
		Content: `git commit -m "x" && rm -rf /`,
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// 期望:rm -rf / 片段独立判 Deny → 至少一条 destructive.filesystem.* (正则)
	// 或 semantic.filesystem.* (语义 Deny 兜底)finding,severity critical(rm -rf / 命中根)。
	found := false
	for _, f := range findings {
		if strings.HasPrefix(f.RuleID, "destructive.filesystem.") ||
			strings.HasPrefix(f.RuleID, "semantic.filesystem.") {
			if f.Severity == SeverityCritical {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("I1 静态闭合:`git commit -m \"x\" && rm -rf /` 经片段拆分,rm -rf / 应独立 Deny,"+
			"期望 critical destructive/semantic.filesystem.* finding,但无: %+v", findings)
	}
}

// TestStaticLayerSpanDataNoFinding 验证 span data 区误报消除(spec §6.5 行为变更):
// `echo "rm -rf /"` 引号内的 rm -rf / 是字面量(echo 参数),不执行。
// ClassifySpans 标记 rm -rf / 为 SpanData,语义 Deny + 正则命中都应丢弃。
//
// 修前(R2):filesystem 语义解析器的 rmCmdRe 正则不识别引号边界,把引号内的
// rm -rf / 误判 Deny → 产 semantic.filesystem.* 误报;
// destructive.filesystem.* 正则也会误命中引号内字面量(但被语义 Deny 的 Gate 1 continue 抑制);
// injection.tool-misuse.tm1 正则命中引号内 rm -rf / 产 critical 误报。
// 修后(R3):语义 Deny 路径 + 正则关卡 2 都做 span 复核,Data span 命中丢弃 → 无 finding。
//
// 用 AssetScript + Content(走 content 字段,产生 Locations 供 span 复核)。
func TestStaticLayerSpanDataNoFinding(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil, nil)
	assets := []configengine.Asset{{
		ID:      "script-echo",
		Type:    configengine.AssetScript,
		Name:    "script",
		Content: `echo "rm -rf /"`,
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// 期望:无 destructive.filesystem.* / semantic.filesystem.* / injection.tool-misuse.* finding
	// (引号内 rm -rf / 是字面量,span data 区)。
	for _, f := range findings {
		if strings.HasPrefix(f.RuleID, "destructive.filesystem.") ||
			strings.HasPrefix(f.RuleID, "semantic.filesystem.") ||
			strings.HasPrefix(f.RuleID, "injection.tool-misuse.") {
			t.Errorf("span data 区误报:echo \"rm -rf /\" 引号内 rm -rf 是字面量,不应触发 %s: %+v",
				f.RuleID, f)
		}
	}
}

// TestStaticLayerHookCommandSpanDataNoFinding 验证 hook + command 字段(无 Location)
// 的 span data 区误报消除:hook command `echo "rm -rf /"` 同样应无 finding。
//
// 这关关卡 2 的 command 字段路径:无 Locations,需用 evidence+strings.Index 重新定位
// 字节偏移检查 span(brief Step 4 推荐方法的 command 字段延伸)。
func TestStaticLayerHookCommandSpanDataNoFinding(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil, nil)
	assets := []configengine.Asset{{
		ID:   "hook-echo",
		Type: configengine.AssetHook,
		Name: "hook",
		Fields: map[string]any{
			"command": `echo "rm -rf /"`,
		},
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	for _, f := range findings {
		if strings.HasPrefix(f.RuleID, "destructive.filesystem.") ||
			strings.HasPrefix(f.RuleID, "semantic.filesystem.") ||
			strings.HasPrefix(f.RuleID, "baseline.dangerous-hook") {
			t.Errorf("span data 区误报(command 字段):echo \"rm -rf /\" 引号内字面量,不应触发 %s: %+v",
				f.RuleID, f)
		}
	}
}

// TestStaticLayerDataSpanDenyDoesNotSuppressRealDeny 验证 Deny 聚合策略:
// `echo "rm -rf /" ; rm -rf /` 含 2 片段,第 1 片段 Deny 在 Data span(误报),
// 第 2 片段 Deny 在 Executed span(真报)。computeLineSemantic 应跳过第 1 片段的 Data span Deny,
// 存第 2 片段的 Executed Deny → 关卡 1 emit semantic finding,不漏报。
//
// 修前(若 computeLineSemantic 不过滤 Data span Deny):存第 1 片段 Deny,关卡 1 span 复核
// 丢弃 → 不 emit 语义 finding;但第 2 片段的真 Deny 也被丢弃(因 denyByDomain 已存第 1 片段)
// → 漏报。修后:跳过 Data span Deny,存 Executed Deny → emit。
func TestStaticLayerDataSpanDenyDoesNotSuppressRealDeny(t *testing.T) {
	home := newRulesHome(t)
	d := NewRulesDetector(home, nil, nil)
	assets := []configengine.Asset{{
		ID:      "script-mixed",
		Type:    configengine.AssetScript,
		Name:    "script",
		Content: `echo "rm -rf /" ; rm -rf /`,
	}}
	findings, err := d.Scan(context.Background(), assets)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// 期望:第 2 片段 rm -rf / 是真 Deny → 至少一条 destructive.filesystem.* (正则)
	// 或 semantic.filesystem.* (语义 Deny)finding,severity critical(rm -rf / 命中根)。
	found := false
	for _, f := range findings {
		if (strings.HasPrefix(f.RuleID, "destructive.filesystem.") ||
			strings.HasPrefix(f.RuleID, "semantic.filesystem.")) &&
			f.Severity == SeverityCritical {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Data span Deny 不应抑制同资产内 Executed Deny:"+
			"`echo \"rm -rf /\" ; rm -rf /` 第 2 片段应产 critical finding,但无: %+v", findings)
	}
}
