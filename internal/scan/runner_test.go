package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

// writeScanFile 是扫描测试辅助:写文件(自动建父目录)。
func writeScanFile(t *testing.T, p, c string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newTestRunner 构造单 agent Runner(root 指向 fixture)。
func newTestRunner(t *testing.T, home string) *Runner {
	t.Helper()
	agents := configengine.DefaultAgents(home, "")
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(home, nil))
	orch := &security.Orchestrator{Registry: r}
	hist := history.NewStore(filepath.Join(home, "..", "history"))
	return NewRunner(agents, orch, hist)
}

func TestRunnerRunScanWritesHistory(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), []byte(`{"permissions":{"allow":["Bash(*)"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	agents := configengine.DefaultAgents(dir, "")
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(dir, nil))
	orch := &security.Orchestrator{Registry: r}
	hist := history.NewStore(filepath.Join(dir, "history"))
	runner := NewRunner(agents, orch, hist)

	res, err := runner.RunScan(context.Background(), "", ScanScope{Type: "global"}, nil, "")
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if len(res.Findings) == 0 {
		t.Error("应检出通配 Bash")
	}
	if res.HealthScore == nil {
		t.Error("应有健康分")
	}
	// 历史应落盘 1 条(List 返回 ScanSummary 列表,按 StartedAt 倒序)
	recs, err := hist.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("历史应 1 条,got %d", len(recs))
	}
}

func TestRunnerNilHistoryNoPanic(t *testing.T) {
	dir := t.TempDir()
	agents := configengine.DefaultAgents(dir, "")
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(dir, nil))
	orch := &security.Orchestrator{Registry: r}
	runner := NewRunner(agents, orch, nil) // History nil
	res, err := runner.RunScan(context.Background(), "", ScanScope{Type: "global"}, nil, "")
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if res == nil {
		t.Error("res 不应 nil")
	}
}

func TestRunScanSelectsAgentByID(t *testing.T) {
	// 构造两个 agent 指向不同 claudeDir,验证 RunScan(agentID) 扫对应目录。
	home := t.TempDir()
	dirA := filepath.Join(home, "agentA")
	dirB := filepath.Join(home, "agentB")
	os.MkdirAll(filepath.Join(dirA, ".claude"), 0o755)
	os.MkdirAll(filepath.Join(dirB, ".claude"), 0o755)
	// 各放一个不同 settings.json 以区分
	os.WriteFile(filepath.Join(dirA, ".claude", "settings.json"), []byte(`{"model":"A"}`), 0o644)
	os.WriteFile(filepath.Join(dirB, ".claude", "settings.json"), []byte(`{"model":"B"}`), 0o644)

	agents := []configengine.Agent{
		{ID: "a", Name: "A", RootDir: filepath.Join(dirA, ".claude"), ClaudeJSON: "", HomeDir: home},
		{ID: "b", Name: "B", RootDir: filepath.Join(dirB, ".claude"), ClaudeJSON: "", HomeDir: home},
	}
	r := NewRunner(agents, &security.Orchestrator{}, nil)
	eng := r.EngineFor("b")
	if eng == nil || eng.ClaudeDir != filepath.Join(dirB, ".claude") {
		t.Fatalf("EngineFor(b) 应返回 B 的 Engine: %+v", eng)
	}
}

func TestRunScanFallsBackToFirstAgentWhenIDEmpty(t *testing.T) {
	home := t.TempDir()
	agents := []configengine.Agent{
		{ID: "first", RootDir: filepath.Join(home, ".claude1"), HomeDir: home},
		{ID: "second", RootDir: filepath.Join(home, ".claude2"), HomeDir: home},
	}
	r := NewRunner(agents, &security.Orchestrator{}, nil)
	eng := r.EngineFor("")
	if eng == nil || eng.ClaudeDir != filepath.Join(home, ".claude1") {
		t.Fatalf("空 agentID 应回退首 agent: %+v", eng)
	}
}

func TestEngineForCachesByAgentID(t *testing.T) {
	home := t.TempDir()
	agents := []configengine.Agent{{ID: "x", RootDir: filepath.Join(home, ".claude"), HomeDir: home}}
	r := NewRunner(agents, &security.Orchestrator{}, nil)
	e1 := r.EngineFor("x")
	e2 := r.EngineFor("x")
	if e1 != e2 {
		t.Fatal("同 agentID 应返回缓存的同一 Engine 实例")
	}
}

func TestRunScanProjectScopeFiltersAssets(t *testing.T) {
	dir := t.TempDir()
	// 全局 settings 含 Bash(*) 通配(会触发 baseline.wildcard-bash finding);
	// 项目 myproj 的 settings 只有 model(不触发该规则)。
	// 若 scopeAssets 退化为 no-op(返回全量),project scope 会包含全局 settings,
	// 下面所有断言都会失败。
	writeScanFile(t, filepath.Join(dir, ".claude", "settings.json"), `{"permissions":{"allow":["Bash(*)"]}}`)
	proj := filepath.Join(dir, "myproj")
	writeScanFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+proj+`":{}}}`)
	writeScanFile(t, filepath.Join(proj, ".claude", "settings.json"), `{"model":"opus"}`)
	r := newTestRunner(t, dir)

	// global scope:全量(含全局 + 项目资产)
	resGlobal, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "global"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	// project scope:仅 myproj 下资产
	resProj, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "project", Path: proj}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	// 1) project scope 的 finding 数应严格小于 global:
	//    global 有 Bash(*) finding,project scope 排除了全局 settings 故应更少。
	//    no-op 过滤器(project=全量)会让两者相等 → 此断言失败。
	if len(resProj.Findings) >= len(resGlobal.Findings) {
		t.Errorf("project scope findings 应严格少于 global: proj=%d global=%d",
			len(resProj.Findings), len(resGlobal.Findings))
	}

	// 2) 最关键的意图断言:全局 Bash(*) finding 的 key 必须出现在 global 但不出现在 project。
	//    这是「project scope 排除了全局 only 资产」的直接证明。
	//    findingKey = DetectorID \x00 RuleID \x00 AssetID \x00 Evidence(与 handlers_edit.go 一致)。
	globalKeys := make(map[string]bool, len(resGlobal.Findings))
	for _, f := range resGlobal.Findings {
		globalKeys[findingKeyTest(f)] = true
	}
	projKeys := make(map[string]bool, len(resProj.Findings))
	for _, f := range resProj.Findings {
		projKeys[findingKeyTest(f)] = true
	}
	// 找到全局的 Bash(*) 通配 finding(key 含 wildcard-bash 规则)。
	var bashKey string
	bashFound := false
	for _, f := range resGlobal.Findings {
		if f.RuleID == "baseline.wildcard-bash" {
			bashKey = findingKeyTest(f)
			bashFound = true
			break
		}
	}
	if !bashFound {
		t.Fatalf("全局扫描应检出 baseline.wildcard-bash,实际 findings=%+v", resGlobal.Findings)
	}
	if !globalKeys[bashKey] {
		t.Errorf("Bash(*) finding 应在 global scope 中(自检失败,key=%q)", bashKey)
	}
	if projKeys[bashKey] {
		t.Errorf("Bash(*) finding 不应出现在 project scope(说明 project 未排除全局 settings),key=%q", bashKey)
	}
}

func TestRunScanAssetScopeScansSiblings(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, ".claude", "settings.json")
	writeScanFile(t, src, `{"permissions":{"allow":["Bash(*)"]}}`)
	// 第二个资产:项目级 settings,不同 source_path,也含 Bash(*) 通配。
	// 若 scopeAssets 退化为 no-op(返回全量),asset scope 会错误地扫到这个项目资产,
	// 其 finding(不同 AssetID)会出现在 res.Findings 中 → 下面的排除断言失败。
	proj := filepath.Join(dir, "otherproj")
	writeScanFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+proj+`":{}}}`)
	writeScanFile(t, filepath.Join(proj, ".claude", "settings.json"), `{"permissions":{"allow":["Bash(*)"]}}`)
	r := newTestRunner(t, dir)

	// global scope:应检出两个 Bash(*) finding(全局 + 项目,AssetID 不同)
	resGlobal, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "global"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	// asset scope:只扫 source_path == src(全局 settings)
	res, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "asset", Path: src}, []string{"rules"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("res 不应为 nil")
	}
	// 1) 必须检出 finding:Bash(*) 通配应触发 baseline.wildcard-bash。
	//    若 scopeAssets 退化为返回零资产(asset 过滤失效),res.Findings 为空 → 此断言失败。
	if len(res.Findings) == 0 {
		t.Fatal("asset scope 应检出 Bash(*) 通配,实际 findings 为空(scopeAssets 可能返回零资产)")
	}
	// 2) asset scope 的 findings 应是 global scope findings 的子集:
	//    asset scope 只扫同 source_path 的 sibling,其结果必然包含于全量扫描。
	globalKeys := make(map[string]bool, len(resGlobal.Findings))
	for _, f := range resGlobal.Findings {
		globalKeys[findingKeyTest(f)] = true
	}
	for _, f := range res.Findings {
		k := findingKeyTest(f)
		if !globalKeys[k] {
			t.Errorf("asset scope finding 不在 global findings 中(应为其子集): key=%q", k)
		}
	}
	// 3) 排除非目标资产:asset scope 只应扫到 src 对应的资产,不应包含项目级 settings 的 finding。
	//    no-op 过滤器(返回全量)会让项目级 finding 出现 → 此断言失败。
	//    通过比较 AssetID:asset scope 的所有 finding 的 AssetID 必须都在 src 对应资产上。
	//    由于 global 有两个不同 AssetID 的 Bash(*) finding,asset scope 应只含其中一个。
	if len(resGlobal.Findings) < 2 {
		t.Fatalf("global scope 应至少检出 2 个 Bash(*) finding(全局+项目),实际 %d", len(resGlobal.Findings))
	}
	// 收集 asset scope 的 AssetID 集合。
	assetScopeAssetIDs := make(map[string]bool, len(res.Findings))
	for _, f := range res.Findings {
		assetScopeAssetIDs[f.AssetID] = true
	}
	// asset scope 只扫一个物理文件,所有 finding 应共享同一 AssetID。
	if len(assetScopeAssetIDs) != 1 {
		t.Errorf("asset scope 应只含 1 个 AssetID(目标文件),实际 %d: %v",
			len(assetScopeAssetIDs), assetScopeAssetIDs)
	}
	// 该 AssetID 必须是 src 对应的(global 中也存在),且不是项目级 settings 的。
	// 收集 global 中两个 Bash(*) finding 的 AssetID,断言 asset scope 的 AssetID 恰为其中之一,
	// 且 global 中至少有另一个 AssetID 不在 asset scope 中(证明过滤生效)。
	globalBashAssetIDs := make(map[string]bool)
	for _, f := range resGlobal.Findings {
		if f.RuleID == "baseline.wildcard-bash" {
			globalBashAssetIDs[f.AssetID] = true
		}
	}
	for _, f := range res.Findings {
		if !globalBashAssetIDs[f.AssetID] {
			t.Errorf("asset scope finding 的 AssetID 不在 global Bash(*) 集合中: %q", f.AssetID)
		}
	}
	// 关键排除断言:global 中至少有一个 Bash(*) AssetID 不在 asset scope 中。
	excluded := false
	for id := range globalBashAssetIDs {
		if !assetScopeAssetIDs[id] {
			excluded = true
		}
	}
	if !excluded {
		t.Errorf("asset scope 应排除至少一个非目标资产的 Bash(*) finding(实际包含了全部 %d 个)",
			len(globalBashAssetIDs))
	}
}

func TestRunScanBackfillsAgentID(t *testing.T) {
	// Runner fixture: tmp dir history + 双 agent,空 Orchestrator(0 findings)。
	// 验证 Finding.AgentID 回填 + history record AgentID 正确。
	home := t.TempDir()
	agents := []configengine.Agent{
		{ID: "agent-a", Name: "A", RootDir: filepath.Join(home, ".claude"), ClaudeJSON: filepath.Join(home, ".claude.json"), HomeDir: home},
		{ID: "agent-b", Name: "B", RootDir: filepath.Join(home, ".claude"), ClaudeJSON: filepath.Join(home, ".claude.json"), HomeDir: home},
	}
	histDir := t.TempDir()
	hist := history.NewStore(histDir)
	emptyReg := security.NewRegistry()
	orch := &security.Orchestrator{Registry: emptyReg}
	r := NewRunner(agents, orch, hist)

	// 扫 agent-b
	res, err := r.RunScan(context.Background(), "agent-b", ScanScope{}, nil, "")
	if err != nil {
		t.Fatalf("RunScan err: %v", err)
	}
	// 结果中所有 finding(如果有)应带 AgentID
	for _, f := range res.Findings {
		if f.AgentID != "agent-b" {
			t.Errorf("Finding.AgentID = %q, want agent-b", f.AgentID)
		}
	}
	// history record 也应 agentID=agent-b
	latest, err := hist.LatestForAgent("agent-b")
	if err != nil {
		t.Fatalf("LatestForAgent err: %v", err)
	}
	if latest == nil {
		t.Fatal("history LatestForAgent(agent-b) 不应 nil")
	}
	if latest.AgentID != "agent-b" {
		t.Errorf("history AgentID = %q, want agent-b", latest.AgentID)
	}
}

func TestRunScanWritesBatchID(t *testing.T) {
	home := t.TempDir()
	agents := []configengine.Agent{
		{ID: "a1", Name: "A1", RootDir: filepath.Join(home, ".claude"), ClaudeJSON: filepath.Join(home, ".claude.json"), HomeDir: home},
	}
	histDir := t.TempDir()
	hist := history.NewStore(histDir)
	orch := &security.Orchestrator{Registry: security.NewRegistry()}
	r := NewRunner(agents, orch, hist)
	_, err := r.RunScan(context.Background(), "a1", ScanScope{}, nil, "batch-xyz")
	if err != nil {
		t.Fatalf("RunScan err: %v", err)
	}
	rec, _ := hist.LatestForAgent("a1")
	if rec == nil || rec.BatchID != "batch-xyz" {
		t.Errorf("BatchID roundtrip: got %q", rec.BatchID)
	}
}

// TestRunScanUserScopeScansGlobalAndPlugin:user scope 只扫 ScopeGlobal+ScopePlugin 资产,
// 排除 ScopeProject。若 scopeAssets user 分支缺失,RunScan 未知 type 退化为全量 → 扫到项目资产。
func TestRunScanUserScopeScansGlobalAndPlugin(t *testing.T) {
	dir := t.TempDir()
	// 全局 settings(含 Bash(*) 通配)→ user scope 应扫到。
	src := filepath.Join(dir, ".claude", "settings.json")
	writeScanFile(t, src, `{"permissions":{"allow":["Bash(*)"]}}`)
	// 项目级 settings(也含 Bash(*) 通配,不同 source_path)→ user scope 应排除。
	proj := filepath.Join(dir, "otherproj")
	writeScanFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+proj+`":{}}}`)
	writeScanFile(t, filepath.Join(proj, ".claude", "settings.json"), `{"permissions":{"allow":["Bash(*)"]}}`)
	r := newTestRunner(t, dir)

	// global scope findings 数量(全局 + 项目)。
	resGlobal, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "global"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	// user scope:只扫全局资产。
	res, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "user"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("res 不应为 nil")
	}
	if len(res.Findings) == 0 {
		t.Fatal("user scope 应检出全局 Bash(*) 通配,findings 为空(scopeAssets user 分支可能缺失)")
	}
	// user scope 排除项目资产 → findings 严格少于 global(后者含项目)。
	if len(res.Findings) >= len(resGlobal.Findings) {
		t.Fatalf("user scope 应排除项目资产,findings %d 不应 >= global %d", len(res.Findings), len(resGlobal.Findings))
	}
}

// TestRunScanAssetIDScopeScansSingleAsset:asset-id scope 按 Asset.ID 精确扫单条记录,
// 排除同 source_path 的 sibling 与其他资产。若分支缺失,退化为全量 → 扫到非目标资产。
func TestRunScanAssetIDScopeScansSingleAsset(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, ".claude", "settings.json")
	writeScanFile(t, src, `{"permissions":{"allow":["Bash(*)"]}}`)
	// 项目级资产(不同 source_path,也含 Bash(*))→ asset-id 不应扫到。
	proj := filepath.Join(dir, "otherproj")
	writeScanFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+proj+`":{}}}`)
	writeScanFile(t, filepath.Join(proj, ".claude", "settings.json"), `{"permissions":{"allow":["Bash(*)"]}}`)
	r := newTestRunner(t, dir)

	// 先 global 扫一次拿到全局 settings 资产的 AssetID。
	resGlobal, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "global"}, []string{"rules"}, "")
	if err != nil {
		t.Fatal(err)
	}
	// 找到全局 settings 的 finding,取其 AssetID(就是目标资产 ID)。
	var targetID string
	for _, f := range resGlobal.Findings {
		if f.AssetID != "" {
			targetID = f.AssetID
			break
		}
	}
	if targetID == "" {
		t.Fatal("global scope 未产出 finding,无法取 AssetID")
	}

	// asset-id scope:只扫 ID == targetID 这一条。
	res, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "asset-id", Path: targetID}, []string{"rules"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("res 不应为 nil")
	}
	if len(res.Findings) == 0 {
		t.Fatal("asset-id scope 应检出目标资产的 Bash(*) 通配,findings 为空")
	}
	// 所有 finding 必须归属 targetID(不混入项目资产)。
	for _, f := range res.Findings {
		if f.AssetID != targetID {
			t.Fatalf("asset-id scope 混入非目标资产 finding: got %s want %s", f.AssetID, targetID)
		}
	}
	// 未知 ID → 空结果(不报错)。
	resEmpty, err := r.RunScan(context.Background(), "claude-code", ScanScope{Type: "asset-id", Path: "nonexistent-id"}, []string{"rules"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if resEmpty == nil || len(resEmpty.Findings) != 0 {
		t.Fatalf("asset-id 未知 ID 应返回空,got %v", resEmpty)
	}
}

// findingKeyTest 生成 finding 去重键,与 handlers_edit.go 的 findingKey 一致:
// (DetectorID, RuleID, AssetID, Evidence)。供测试比较 scope 过滤结果用。
func findingKeyTest(f security.Finding) string {
	return f.DetectorID + "\x00" + f.RuleID + "\x00" + f.AssetID + "\x00" + f.Evidence
}
