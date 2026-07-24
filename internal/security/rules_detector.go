package security

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security/ruleengine"
	"code-agent-sentinel/internal/security/ruleengine/semantics"
	"code-agent-sentinel/internal/security/suppression"
)

// RulesDetector 是统一声明式规则引擎检测器,替代旧 BaselineDetector + InjectionDetector。
//
// 它加载内置 + 全局 + 项目规则(经 ruleengine.LoadForScan:合并 + Validate),对每个资产
// 按规则 asset_type 路由求值;命中产 Finding,施加两层抑制(baseline/inline);规则加载/校验
// 错误产独立的 load-error Finding(Severity=Info,不进健康分)。
//
// 设计要点(controller 预飞行决议):
//   - Covers() 返回 nil —— orchestrator 的 filterByCovers 在 covers 为空时传全部资产,
//     RulesDetector 内部按 r.AssetType 路由(各资产只跑匹配类型规则),等价于旧两个检测器
//     各自声明 Covers 的并集,且天然支持未来新 asset_type 的规则(无需改 Covers)。
//   - 加载分层:内置 + 全局 + 抑制(baseline/suppressions)在构造时一次性加载(d.baseRules +
//     d.loadErrs,Meta 与 Scan 共用);项目规则随每次 Scan 动态加载(从 ~/.claude.json 读
//     已知项目,见 Scan.loadProjectRules 注释)。
//   - load-error Finding 用 SeverityInfo(系数 0.0),确保 ComputeHealth 不为其扣分
//     (见 TestRulesDetectorLoadErrorNotInHealth 的数学验证)。
type RulesDetector struct {
	home string
	cfg  *config.DetectorsConfig

	// builtin + global 合并 + Validate 后的规则(Meta 与 Scan 基础层)。
	// Scan 时再叠加项目规则得到 allRules。
	baseRules []ruleengine.Rule
	// 构造时的加载/校验错误(builtin + global + baseline + suppressions)。
	// Scan 时追加项目规则错误,合并成 load-error findings。
	loadErrs []ruleengine.RuleLoadError

	baseline *suppression.BaselineSet  // 已知指纹快照(命中 → Suppression="baseline");nil=无
	supprs   *suppression.Suppressions // 行内豁免(命中 → Suppression="inline");nil=无
}

// NewRulesDetector 构造检测器:加载内置 + 全局规则 + 抑制配置。
//
// 项目规则不在此加载 —— Scan 时按传入资产的已知项目动态加载(见 Scan.loadProjectRules)。
// 原因:Scan 收到的是 []Asset 而非 *Inventory,无法直接拿到 inventory.Projects;
// 项目列表的唯一权威来源是 ~/.claude.json(readProjectList),Scan 内用 configengine
// .NewEngine(home).ListProjects() 读取即可(文件缺失返回 nil,nil,安全)。
//
// 路径 TODO(Finding #5):baseline/suppressions/全局规则目录的路径目前硬编码
// filepath.Join(home, ".claude-sentinel", ...),扫描侧(scan-time)不读 cfg 配置。
// CLI/API 写侧已通过 cfg.ResolveBaselinePath/ResolveSuppressionsPath/ResolveRulesDir
// 支持 config 覆盖(suppress_path/baseline_path/sentinel_rules_dir);若用户在 config 里
// 改了这些路径,写会落到自定义路径,但本检测器仍读默认路径 → 扫描时抑制/baseline/全局规则
// 静默不生效。security 包目前不接收 config(controller 决议待定),故此处不改行为,仅标注。
// 将来把 config 接入本检测器时,改这三处 filepath.Join 为 cfg.Resolve*Path(home)。
func NewRulesDetector(home string, cfg *config.DetectorsConfig) *RulesDetector {
	d := &RulesDetector{home: home, cfg: cfg}

	// 内置 + 全局规则:LoadForScan(home, nil) = builtin + global(无项目),合并 + Validate。
	// 用 LoadForScan 而非 LoadBuiltin+LoadDir 分开调,是为了复用 Merge+Validate 一次成型。
	// 路径 Finding #5:全局规则目录在 LoadForScan 内硬编码,扫描侧不读 cfg(sentinel_rules_dir)。
	rules, errs := ruleengine.LoadForScan(home, nil)
	d.baseRules = rules
	d.loadErrs = errs

	// 抑制配置:文件不存在 → (nil,nil) 静默(用户尚未生成/创建,非错误)。
	// 路径 Finding #5:扫描侧硬编码默认路径,不读 cfg(baseline_path/suppress_path 覆盖在
	// 扫描时不生效,仅写侧生效)。见上方 TODO。
	baselinePath := filepath.Join(home, ".claude-sentinel", "baseline.json")
	if bs, err := suppression.LoadBaseline(baselinePath); err != nil {
		d.loadErrs = append(d.loadErrs, ruleengine.RuleLoadError{
			Source: baselinePath, Reason: fmt.Sprintf("load baseline: %v", err),
		})
	} else {
		d.baseline = bs
	}
	supprPath := filepath.Join(home, ".claude-sentinel", "suppressions.yaml")
	if s, err := suppression.LoadSuppressions(supprPath); err != nil {
		d.loadErrs = append(d.loadErrs, ruleengine.RuleLoadError{
			Source: supprPath, Reason: fmt.Sprintf("load suppressions: %v", err),
		})
	} else {
		d.supprs = s
	}

	return d
}

func (d *RulesDetector) ID() string                       { return "rules" }
func (d *RulesDetector) Covers() []configengine.AssetType { return nil } // 见类型注释
func (d *RulesDetector) Enabled() bool                    { return d.cfg.RulesEnabled() }
func (d *RulesDetector) Available() bool                  { return true }
func (d *RulesDetector) Reason() string                   { return "" }

// Meta 返回检测器能力元数据(UI 展示用,纯静态描述)。
// Rules 摘要来自 baseRules(内置 + 全局);项目规则是动态的、按资产来源项目变化,不入 Meta。
func (d *RulesDetector) Meta() DetectorMeta {
	rules := make([]RuleInfo, 0, len(d.baseRules))
	for _, r := range d.baseRules {
		ri := RuleInfo{
			ID:            r.ID,
			Severity:      r.Severity,
			AssetType:     r.AssetType,
			Description:   r.Description,
			Syntax:        r.Syntax(),
			Remediation:   r.Remediation,
			PostExclude:   r.PostExclude,
			Deobfuscation: r.Deobfuscation,
			Dotall:        r.Dotall,
			Metadata:      r.Metadata,
			SourceFile:    r.Source,
			ProjectPath:   r.ProjectPath,
		}
		if r.Paths != nil {
			ri.Paths = &PathFilterInfo{Include: r.Paths.Include, Exclude: r.Paths.Exclude}
		}
		rules = append(rules, ri)
	}
	return DetectorMeta{
		ID:      d.ID(),
		Name:    "声明式规则引擎",
		Enabled: d.Enabled(),
		Engines: []EngineInfo{{Name: "声明式规则引擎", Kind: "embedded", Enabled: d.Enabled(), Available: true}},
		Rules:   rules,
		Covers:  nil, // 与 Covers() 一致:nil = 全资产类型(内部路由)
	}
}

// Scan 对一批资产跑全部匹配规则。
func (d *RulesDetector) Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error) {
	// 构造时已加载 builtin + global 规则(d.baseRules)与抑制配置;Scan 只需叠加项目规则。
	// 项目规则:从 ~/.claude.json 读已知项目(权威源),加载各项目 ~/.sentinel/rules/。
	// Scan 收 []Asset 无法直接拿 inventory,故用 ListProjects() 重建项目集。
	// 项目规则带 ProjectPath 标记(求值时按 SourcePath 隔离,只对该项目资产生效)。
	projectRules, projectErrs := d.loadProjectRules()

	// 合并 builtin + global(已 Validate) + project,再 Validate 一次(项目规则可能未编译正则)。
	// Validate 幂等:对已 Validate 的 builtin/global 重跑只会重填 regexes 缓存,不报错。
	allRules, validateErrs := ruleengine.Validate(ruleengine.Merge(d.baseRules, projectRules))

	// 加载错误:构造时的 d.loadErrs(builtin + global + baseline + suppressions)
	// + 本次项目规则加载错误 + 合并后 Validate 错误。
	loadErrs := append([]ruleengine.RuleLoadError{}, d.loadErrs...) // 复制,不污染构造时快照
	loadErrs = append(loadErrs, projectErrs...)
	loadErrs = append(loadErrs, validateErrs...)

	var out []Finding
	for _, a := range assets {
		// 提取命令文本(语义解析器输入):hook/mcp_server 用 command 字段,
		// script/skill/command/agent/memory 用 content 字段,permissions 无命令文本(跳过语义)。
		cmdText, _ := commandTextFromAsset(a)

		// 按行语义状态(修复最终 review C1:Safe span-scoping)。
		// 修前:对整个命令文本跑一次 DispatchCommand 缓存单个 SemanticResult,任意 Safe 决策
		// 会经关卡 1 `case Safe: continue` + 关卡 2 抑制该 asset 全部规则 —— 包括同一 content
		// 另一行的真实 rm -rf /(漏报:git commit -m 'rm -rf /'\nrm -rf / → health 60 而非 20)。
		// 修后:按行跑 DispatchCommand,Deny 仍 asset-wide-correct(任意行 Deny 即触发语义
		// finding + 该域规则 continue),Safe 只抑制落在 safeLines 内的正则命中(同一行才抑制)。
		// 无命令文本/permissions 资产 → anySem=false,语义关卡整体跳过(纯正则)。
		semState := computeLineSemantic(cmdText)

		// 预选每域语义 finding 载体规则(关卡 1 Deny 用):
		// 按 dcg_rule_id == sem.RuleID 精确匹配(继承正确 severity/remediation);
		// 无精确匹配回退到该域首条 asset_type 匹配规则(snowflake.drop 通用 RuleID 时,
		// 修 C2 后 snowflake 语义返回具体 dcg_rule_id,通常 strategy 1 即命中)。
		// 见 pickSemanticCarrier 注释(修复 review Important #1)。
		semCarriers := map[string]*ruleengine.Rule{}
		for _, dom := range semState.denyOrder {
			res := semState.denyByDomain[dom]
			semCarriers[dom] = pickSemanticCarrier(allRules, a, dom, res.RuleID)
		}
		// emittedDomains:每域只构造一次语义 finding(去重;多 Deny 域各产一条)。
		emittedDomains := map[string]bool{}

		for _, r := range allRules {
			// 规则按 asset_type 路由(Covers=nil → 全资产传入,内部按类型分发)。
			// 修复 review Important #1:原实现严格 `r.AssetType != a.Type` 路由,但 destructive
			// 域规则全部 asset_type=hook + or-tree 覆盖 command/content/allow 字段,严格路由使其
			// 只评估 AssetHook,AssetScript/Skill/Command/Agent/Memory/Permissions 的 rm -rf / 不会被
			// destructive.* 精确规则检测(仅 injection.tm1 粗住 AssetScript,其余无覆盖)。
			// 放宽:destructive 域规则(metadata.source=dcg 且 metadata.domain 属 destructive 五域)
			// 额外评估所有 command-bearing 资产类型(hook/mcp_server/script/skill/command/agent/
			// memory/permissions),or-tree 内部按字段路由自然匹配。
			// injection/baseline/skill 规则仍严格路由(它们 asset_type=script/hook/settings 靶向特定资产)。
			if !ruleAppliesToAsset(r, a.Type) {
				continue
			}
			// 项目规则隔离:ProjectPath 非空 → 只对该项目(SourcePath 在项目根下)的资产生效
			if r.ProjectPath != "" && !pathInProject(a.SourcePath, r.ProjectPath) {
				continue
			}

			// 语义两道关卡(仅对有命令文本 + 有语义解析器的域 + 非 permissions 资产触发)。
			// domain 来自规则 Metadata["domain"](destructive.<domain>.<pattern>)。
			domain, _ := r.Metadata["domain"].(string)
			semActive := semState.anySem && semantics.HasParser(domain)

			if semActive {
				// 关卡 1(正则前语义判定):Deny 按域触发。
				//   若该域有任意行 Deny(denyByDomain[domain] 存在)→ 构造一次语义 finding
				//     (去重:emittedDomains[domain]),并 continue 跳过该域所有正则
				//     (语义已判破坏,正则不必再跑;Deny asset-wide-correct,任意行 Deny 即触发)。
				//   Safe 不再在此无条件 continue(C1 修复):Safe 改由关卡 2 按行 span-scoping 复核,
				//     避免一行 Safe 抑制另一行的真实破坏性命令。
				//   Unknown → 走正则。
				if denyRes, ok := semState.denyByDomain[domain]; ok {
					if !emittedDomains[domain] {
						carrier := r
						if c := semCarriers[domain]; c != nil {
							carrier = *c
						}
						f := makeSemanticFinding(d, carrier, a, denyRes)
						fp := ruleengine.Fingerprint(carrier, a.ID)
						applySuppression(&f, fp, d.baseline, d.supprs)
						out = append(out, f)
						emittedDomains[domain] = true
					}
					continue // 该域所有规则跳过正则(语义已判破坏)
				}
			}

			res := ruleengine.Eval(r, a)
			if !res.Matched {
				continue
			}

			// 关卡 2:正则后语义 Safe 按行复核(防误报,C1 span-scoping)。
			// 正则命中,若该命中落在 Safe 行(同一行语义判 Safe)则丢弃 —— 抑制数据区内
			// 字面量误报(如 git commit -m "rm -rf /" 单行 Safe,该行 rm 字面量不报)。
			// 但落在非 Safe 行(另一行的真实 rm -rf /)→ 不丢弃(修 C1 跨行漏报)。
			// Locations 为空(command 字段命中无行位置)→ 整条命令 wholeSafe 才丢弃
			// (hook/mcp_server 单行命令 Safe 场景)。
			if semActive {
				if semState.findingInSafeLines(res.Locations) {
					continue // 丢弃正则误报(命中在 Safe 行内)
				}
			}

			fp := ruleengine.Fingerprint(r, a.ID)
			f := Finding{
				DetectorID:  d.ID(),
				RuleID:      r.ID,
				Severity:    Severity(r.Severity),
				AssetID:     a.ID,
				AssetType:   a.Type,
				AssetName:   a.Name,
				Message:     r.Description,
				Evidence:    truncate(res.Evidence, 200),
				Remediation: r.Remediation,
				Fingerprint: fp,
				Locations:   res.Locations,
			}
			applySuppression(&f, fp, d.baseline, d.supprs)
			out = append(out, f)
		}
	}

	// load-error Finding:AssetID 用 "rules:" + e.Source(合成 ID,不在任何 inventory)。
	// Severity=Info(系数 0.0)→ ComputeHealth 不为其扣分(见 TestRulesDetectorLoadErrorNotInHealth)。
	// 这是 spec 决策 #12「load-error Finding 不进健康分」的落地:旧 brief 用 SeverityMedium,
	// 但 Medium 系数 1.5 会让合成 AssetID 以 w=1.0 兜底权重扣分,破坏该决策,故改 Info。
	for _, e := range loadErrs {
		out = append(out, Finding{
			DetectorID:  d.ID(),
			RuleID:      "rules.load-error",
			Severity:    SeverityInfo,
			AssetID:     "rules:" + e.Source,
			Message:     "规则加载错误",
			Evidence:    e.Reason,
			Remediation: "修复规则文件语法或配置(详见 evidence)",
		})
	}
	return out, nil
}

// loadProjectRules 加载所有已知项目的 ~/.sentinel/rules/ 规则,设置 ProjectPath 标记。
// 项目列表来自 ~/.claude.json(ListProjects);文件缺失/损坏 → 空列表(安全降级)。
func (d *RulesDetector) loadProjectRules() ([]ruleengine.Rule, []ruleengine.RuleLoadError) {
	eng := configengine.NewEngine(d.home, "")
	projects, err := eng.ListProjects()
	if err != nil {
		return nil, nil
	}
	var allRules []ruleengine.Rule
	var errs []ruleengine.RuleLoadError
	for _, p := range projects {
		dir := filepath.Join(p.Path, ".sentinel", "rules")
		prules, perrs := ruleengine.LoadDir(dir, "project")
		errs = append(errs, perrs...)
		for i := range prules {
			prules[i].ProjectPath = p.Path
		}
		allRules = append(allRules, prules...)
	}
	return allRules, errs
}

// pathInProject 判断 assetPath 是否在 projectPath 目录下(项目规则隔离)。
// projectPath 是项目根;assetPath 须落在 projectPath 内(含本身)才视为属该项目。
// 用 filepath.Rel:相对路径不以 ".." 开头即在目录内。
func pathInProject(assetPath, projectPath string) bool {
	if assetPath == "" || projectPath == "" {
		return false
	}
	rel, err := filepath.Rel(projectPath, assetPath)
	if err != nil {
		return false
	}
	return rel != ".." && !startsWithDotDot(rel)
}

// startsWithDotDot 判断 rel 是否以 ".." 后跟分隔符开头(即 "../xxx",在目录外)。
// rel==".." 已在调用处排除。
func startsWithDotDot(rel string) bool {
	return len(rel) >= 3 && rel[0] == '.' && rel[1] == '.' && (rel[2] == filepath.Separator || rel[2] == '/')
}

// ruleAppliesToAsset 判断规则是否对指定 asset 类型生效(路由)。
//
// 修复 review Important #1(destructive 域 asset_type=hook 覆盖缺口):
//   - 严格路由:所有非 destructive 域规则(injection/baseline/skill 等)按 r.AssetType == a.Type
//     精确匹配(它们 asset_type=script/hook/settings 靶向特定资产)。
//   - 放宽路由:destructive 域规则(metadata.source=dcg 且 metadata.domain 属五域之一
//     git/filesystem/database/containers/package_managers)虽声明 asset_type=hook,
//     其 or-tree 覆盖 command/content/allow 三字段 → 额外评估所有 command-bearing 资产类型
//     (hook/mcp_server/script/skill/command/agent/memory/permissions)。
//     使 AssetScript/Skill/Command/Agent/Memory/Permissions 内的 rm -rf / 能被 destructive.*
//     精确规则检测(原严格路由使其只评估 AssetHook,or-tree content/allow 分支成死代码)。
//   - 注:放宽后 destructive 与 injection.tm1(asset_type=script)可能在同一 AssetScript 上
//     双触发(不同 severity/remediation),属 defense-in-depth,可接受。
func ruleAppliesToAsset(r ruleengine.Rule, assetType configengine.AssetType) bool {
	// 精确匹配:所有规则的原有行为(asset_type 相同)。
	if string(r.AssetType) == string(assetType) {
		return true
	}
	// 放宽:destructive 域规则额外评估 command-bearing 资产。
	src, _ := r.Metadata["source"].(string)
	domain, _ := r.Metadata["domain"].(string)
	if src != "dcg" {
		return false
	}
	switch domain {
	case "git", "filesystem", "database", "containers", "package_managers":
	default:
		return false
	}
	// 仅当规则声明 asset_type=hook(destructive 规则的统一声明)且目标类型属 command-bearing。
	if r.AssetType != string(configengine.AssetHook) {
		return false
	}
	switch assetType {
	case configengine.AssetHook, configengine.AssetMCPServer,
		configengine.AssetScript, configengine.AssetSkill, configengine.AssetCommand,
		configengine.AssetAgent, configengine.AssetMemory, configengine.AssetPermissions:
		return true
	}
	return false
}

// rulesForTest 仅供测试:暴露 baseRules 供测试算 fingerprint(规则结构稳定)。
// 非导出方法,不进公开 API。
func (d *RulesDetector) rulesForTest() []ruleengine.Rule {
	return d.baseRules
}

// commandTextFromAsset 从资产提取命令文本 + 字段名(语义解析器输入)。
//   - hook/mcp_server:取 Fields["command"](命令行文本),field="command"
//   - script/skill/command/agent/memory:取 Content(脚本/markdown 正文),field="content"
//   - permissions(allow 字段)及其他:返回 ("","")(权限声明非命令,语义层跳过)
//
// 字段缺失(如 hook 无 command)返回 ("",""),语义关卡短路。
func commandTextFromAsset(a configengine.Asset) (text string, field string) {
	switch a.Type {
	case configengine.AssetHook, configengine.AssetMCPServer:
		if v, ok := a.Fields["command"].(string); ok {
			return v, "command"
		}
		return "", ""
	case configengine.AssetScript, configengine.AssetSkill, configengine.AssetCommand,
		configengine.AssetAgent, configengine.AssetMemory:
		return a.Content, "content"
	}
	return "", ""
}

// lineSemanticState 是按行跑语义解析后的聚合状态(修复最终 review C1)。
//
// 修前:对整个命令文本跑一次 DispatchCommand,缓存单个 SemanticResult,所有规则复用。
// 问题:任何 Safe 决策(git commit -m / rm -i / git restore --staged / git checkout -b)
// 都会经关卡 1 `case Safe: continue` + 关卡 2 抑制该 asset 的所有规则 —— 包括同一
// content 里另一行的真实 rm -rf /(漏报:health 60 而非 20)。
//
// 修后(C1 span-scoping):对 content 资产按行切分,逐行跑 DispatchCommand,聚合:
//   - denyByDomain:每域首条 Deny 结果(用于关卡 1 语义 finding + 该域规则 continue)。
//     Deny 仍 asset-wide-correct(任意行 Deny 即触发),保持破坏性兜底不漏。
//   - safeLines:被判 Safe 的行号集合(1-based);关卡 2 只丢弃 Location.Line 落在
//     safeLines 内的正则命中(同一行 Safe 才抑制,不跨行)。
//   - wholeSafe:所有行都 Safe(单行命令或全部行 Safe);用于无 Location 的 command
//     字段匹配(hook/mcp_server command 无行位置,按整条命令 Safe 判定丢弃)。
//
// 简化:按 \n 切行;跨行命令(如 rm -rf \\\n/)可能被拆散。这类情况任意行若 Unknown
// → 不进 safeLines → 不抑制,正则照常命中(不漏报);仅当跨行命令恰好被判 Safe 时可能
// 误抑制,属 R1 可接受简化(review C1 明确允许 line-split 简化)。
type lineSemanticState struct {
	denyByDomain map[string]semantics.SemanticResult // 域 → 首条 Deny(关卡 1 用)
	denyOrder    []string                            // Deny 域出现顺序(稳定 emit 顺序)
	safeLines    map[int]bool                        // Safe 行号(1-based,关卡 2 用)
	wholeSafe    bool                                // 所有行均 Safe(无 Location 命中丢弃用)
	anySem       bool                                // 是否跑了语义(有命令文本 + 非 permissions)
}

// computeLineSemantic 按行跑 DispatchCommand,聚合 lineSemanticState。
// 无命令文本 / permissions 资产 → 返回 anySem=false(语义关卡整体跳过)。
func computeLineSemantic(cmdText string) lineSemanticState {
	st := lineSemanticState{
		denyByDomain: map[string]semantics.SemanticResult{},
		safeLines:    map[int]bool{},
	}
	if cmdText == "" {
		return st
	}
	st.anySem = true
	lines := strings.Split(cmdText, "\n")
	allSafe := true
	for i, line := range lines {
		res := semantics.DispatchCommand(line)
		switch res.Decision {
		case semantics.Deny:
			dom := mapSemDomain(res.RuleID)
			if dom != "" {
				if _, ok := st.denyByDomain[dom]; !ok {
					st.denyByDomain[dom] = res
					st.denyOrder = append(st.denyOrder, dom)
				}
			}
			allSafe = false
		case semantics.Safe:
			st.safeLines[i+1] = true
		case semantics.Unknown:
			allSafe = false
		}
	}
	st.wholeSafe = allSafe
	return st
}

// mapSemDomain 把语义 RuleID 的首段映射到 sentinel 规则 domain Metadata。
//   git → git, filesystem → filesystem, snowflake.* → database
// snowflake 语义 RuleID 现返回具体 dcg_rule_id(如 snowflake.drop-database,修复 C2),
// 首段仍是 "snowflake",映射到 sentinel 的 "database" 域。
func mapSemDomain(ruleID string) string {
	switch strings.SplitN(ruleID, ".", 2)[0] {
	case "git":
		return "git"
	case "filesystem":
		return "filesystem"
	case "snowflake":
		return "database"
	}
	return ""
}

// findingInSafeLines 判断正则命中是否落在 Safe 行(关卡 2 用)。
// Locations 为空(command 字段命中,无行位置)时,若整条命令 wholeSafe 才丢弃;
// 否则按 Location.Line 是否在 safeLines 判定。命中多行只要有一行非 Safe 就不丢弃
// (保守:避免漏报跨行命中)。
func (st lineSemanticState) findingInSafeLines(locs []ruleengine.Location) bool {
	if len(locs) == 0 {
		return st.wholeSafe // command 字段命中无行位置:整条命令 Safe 才丢弃
	}
	for _, loc := range locs {
		if !st.safeLines[loc.Line] {
			return false // 有命中行非 Safe → 不丢弃
		}
	}
	return true
}


// pickSemanticCarrier 为语义 Deny finding 预选载体规则(修复 review Important #1)。
//
// 选择策略(按优先级):
//  1. 精确匹配:规则 Metadata["dcg_rule_id"] == semRuleID(如 semRuleID="filesystem.rm-rf-root-home"
//     匹配 destructive.filesystem.rm-rf-root-home 的 dcg_rule_id)。继承正确 severity/remediation。
//  2. 回退:该域首条 asset_type 匹配规则(snowflake.drop 是通用语义 RuleID,无对应 dcg_rule_id,
//     回退到首条 database 规则做载体)。
//  3. 若该域无任何匹配规则(理论不发生,Deny 必有对应域规则),返回 nil。
//
// semRuleID 是语义解析器返回的 dcg 风格 RuleID(如 "filesystem.rm-rf-root-home" / "git.reset-hard" /
// "snowflake.drop")。规则的 dcg_rule_id 在 Tasks 4-7 转写时设置(与 dcg 源 rule_id 对齐)。
//
// 修复前:用循环内首条域匹配规则做载体,若该规则 severity 与语义判定不匹配(如 rm -rf / →
// 首条 sed-exec-unverified high,而非 rm-rf-root-home critical),finding severity 被扭曲,
// 健康分扣分失真(underweighted 37.5%)。
func pickSemanticCarrier(rules []ruleengine.Rule, a configengine.Asset, semDenyRuleDomain, semRuleID string) *ruleengine.Rule {
	// 策略 1:精确匹配 dcg_rule_id == semRuleID。
	// 遍历全部规则(不止该域),因 semRuleID 含域段(如 "filesystem.rm-rf-root-home"),
	// dcg_rule_id 也含域段,跨域不会误匹配。
	if semRuleID != "" {
		for i := range rules {
			r := &rules[i]
			if !ruleAppliesToAsset(*r, a.Type) {
				continue
			}
			if r.ProjectPath != "" && !pathInProject(a.SourcePath, r.ProjectPath) {
				continue
			}
			if dcgID, _ := r.Metadata["dcg_rule_id"].(string); dcgID == semRuleID {
				return r
			}
		}
	}
	// 策略 2:回退到该域首条 asset_type 匹配规则。
	for i := range rules {
		r := &rules[i]
		if !ruleAppliesToAsset(*r, a.Type) {
			continue
		}
		if r.ProjectPath != "" && !pathInProject(a.SourcePath, r.ProjectPath) {
			continue
		}
		domain, _ := r.Metadata["domain"].(string)
		if domain == semDenyRuleDomain {
			return r
		}
	}
	// 策略 3:无匹配(理论不发生)。
	return nil
}

// makeSemanticFinding 构造语义 Deny finding(关卡 1 用)。
//
// 与正则 finding 的区别:
//   - RuleID:用 `semantic.<dcg_rule_id>`(如 semantic.filesystem.rm-rf-root-home),
//     与正则规则 ID(destructive.filesystem.rm-rf-root-home)区分,便于 UI/审计追溯来源。
//   - Evidence:用 sem.Reason(语义解析器的判定理由,如 "rm -rf /(递归强制删根/home)")
//   - Severity/Locations:语义解析器不产位置信息,Locations 留空;Severity 复用载体规则的 severity
//     (载体规则按 dcg_rule_id == sem.RuleID 精确匹配,如 rm -rf / → rm-rf-root-home critical,
//     语义 finding 继承正确 severity,健康分扣分不失真)
//
// RuleID 映射:sem.RuleID 是 dcg 风格(如 "filesystem.rm-rf-root-home" / "git.reset-hard"),
// 加 "semantic." 前缀避免与正则规则 ID 冲突(正则规则用 "destructive." 前缀)。
// 载体规则 r 由 pickSemanticCarrier 预选(按 dcg_rule_id 精确匹配,fallback 到域首条规则)。
// Fingerprint 仍用载体规则 r 算(稳定锚定规则意图,不受 Evidence 文本变化影响)。
func makeSemanticFinding(d *RulesDetector, r ruleengine.Rule, a configengine.Asset,
	sem semantics.SemanticResult) Finding {
	ruleID := "semantic." + sem.RuleID
	if sem.RuleID == "" {
		// 语义解析器未填 RuleID(理论上不会发生,Deny 必填 RuleID),兜底用规则 ID
		ruleID = r.ID
	}
	return Finding{
		DetectorID:  d.ID(),
		RuleID:      ruleID,
		Severity:    Severity(r.Severity),
		AssetID:     a.ID,
		AssetType:   a.Type,
		AssetName:   a.Name,
		Message:     r.Description,
		Evidence:    truncate(sem.Reason, 200),
		Remediation: r.Remediation,
		Fingerprint: ruleengine.Fingerprint(r, a.ID),
	}
}
