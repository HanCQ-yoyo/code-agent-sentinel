package ruleengine

import "testing"

func TestQuickRejectEmptyKeywords(t *testing.T) {
	// 空关键词列表保守不 reject(返回 false=进入精检,防漏放行)
	if QuickReject("rm -rf /", nil) {
		t.Fatal("空 keywords 应返回 false(进入精检),防漏放行")
	}
}

func TestQuickRejectHitKeywordEnterPrecise(t *testing.T) {
	kw := []string{"git", "rm"}
	// 命中关键词 = 进入精检 = false
	if QuickReject("git reset --hard", kw) {
		t.Fatal("命中 git 应进入精检(false),不该 reject")
	}
	if QuickReject("rm -rf /", kw) {
		t.Fatal("命中 rm 应进入精检(false)")
	}
}

func TestQuickRejectNoKeywordPassThrough(t *testing.T) {
	kw := []string{"git", "rm"}
	// 未命中关键词 = 放行 = true
	if !QuickReject("ls -la", kw) {
		t.Fatal("未命中关键词应放行(true)")
	}
	if !QuickReject("echo hello", kw) {
		t.Fatal("echo 未命中应放行")
	}
}

func TestQuickRejectWordBoundary(t *testing.T) {
	kw := []string{"git"}
	// cat .gitignore 含 "git" 子串但不是词,应放行(词边界)
	if !QuickReject("cat .gitignore", kw) {
		t.Fatal("cat .gitignore 不应因 git 子串进入精检(词边界)")
	}
}

func TestQuickRejectObfuscationFallback(t *testing.T) {
	kw := []string{"git"}
	// g\it reset --hard 含混淆字符 \,无子串命中 → 不 reject(回退 normalize 重判)
	if QuickReject("g\\it reset --hard", kw) {
		t.Fatal("含混淆字符应回退 normalize(不 reject),防 g\\it 漏报")
	}
}

func TestCollectKeywordsFromRules(t *testing.T) {
	rules := []Rule{
		{ID: "a", Metadata: map[string]any{"keywords": []any{"git", "rm"}}},
		{ID: "b", Metadata: map[string]any{"keywords": []any{"rm", "docker"}}},
		{ID: "c", Metadata: map[string]any{"domain": "x"}}, // 无 keywords 跳过
	}
	kw := CollectKeywords(rules)
	got := map[string]bool{}
	for _, k := range kw {
		got[k] = true
	}
	if !got["git"] || !got["rm"] || !got["docker"] || len(kw) != 3 {
		t.Fatalf("CollectKeywords 去重不对: %v", kw)
	}
}

func TestCollectKeywordsFromBuiltin(t *testing.T) {
	rules, _, _ := LoadBuiltin()
	kw := CollectKeywords(rules)
	if len(kw) == 0 {
		t.Fatal("LoadBuiltin 规则应有 keywords")
	}
	// 核心关键词必须在(覆盖 git/filesystem/containers 三域)
	have := map[string]bool{}
	for _, k := range kw {
		have[k] = true
	}
	for _, want := range []string{"git", "rm", "docker"} {
		if !have[want] {
			t.Errorf("缺少核心关键词 %q", want)
		}
	}
}
