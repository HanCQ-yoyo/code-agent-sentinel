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

		// 语义缓存:对该 asset 的命令文本跑一次 DispatchCommand(优先级分发),
		// 所有规则复用。nil = 尚未计算或不可计算(无命令文本/permissions 资产);
		// 非 nil = 已缓存(含 Unknown,表示语义层无法判定)。
		// 用 DispatchCommand(优先级分发 git > filesystem > database)而非 Dispatch(domain, ...):
		// 解决 `git commit -m "rm -rf /"` 跨域冲突 —— filesystem 解析器不识别 git
		// 数据区边界会误判 Deny;优先级分发让 git Safe 先返回,正确抑制误报。
		var semResult *semantics.SemanticResult
		if cmdText != "" && a.Type != configengine.AssetPermissions {
			res := semantics.DispatchCommand(cmdText)
			semResult = &res
		}
		// semDenyDomain 记录语义 Deny 来源域(从 RuleID 第一段提取,如 "filesystem"/"git"/"snowflake")。
		// 仅对该域规则跳过正则(语义已判破坏,正则不必再跑)。
		// 注意:snowflake 域 RuleID 是 "snowflake.drop",域段 = "snowflake",
		// 但 sentinel 规则域是 "database" —— 需特殊映射(见下方 semDenyRuleDomain)。
		semDenyDomain := ""
		// semDenyRuleDomain 映射到 sentinel 规则的 domain Metadata:
		//   git → git, filesystem → filesystem, snowflake → database
		semDenyRuleDomain := ""
		if semResult != nil && semResult.Decision == semantics.Deny && semResult.RuleID != "" {
			semDenyDomain = strings.SplitN(semResult.RuleID, ".", 2)[0]
			switch semDenyDomain {
			case "git":
				semDenyRuleDomain = "git"
			case "filesystem":
				semDenyRuleDomain = "filesystem"
			case "snowflake":
				semDenyRuleDomain = "database"
			}
		}
		// semFindingEmitted 标记语义 finding 是否已构造(去重:一个 asset 一条语义 finding)。
		semFindingEmitted := false

		for _, r := range allRules {
			// 规则按 asset_type 路由(Covers=nil → 全资产传入,内部按类型分发)
			if string(r.AssetType) != string(a.Type) {
				continue
			}
			// 项目规则隔离:ProjectPath 非空 → 只对该项目(SourcePath 在项目根下)的资产生效
			if r.ProjectPath != "" && !pathInProject(a.SourcePath, r.ProjectPath) {
				continue
			}

			// 语义两道关卡(仅对有命令文本 + 有语义解析器的域 + 非 permissions 资产触发)。
			// domain 来自规则 Metadata["domain"](destructive.<domain>.<pattern>)。
			domain, _ := r.Metadata["domain"].(string)
			semActive := semResult != nil && semantics.HasParser(domain)

			if semActive {
				// 关卡 1:正则前语义判定。
				//   Deny + 规则域 == 语义来源域(mapped)→ 该域所有规则跳过正则(语义已判破坏);
				//     对首条匹配规则构造一次语义 finding(去重:semFindingEmitted 标记)。
				//   Safe → 跳过该规则(防误报,如 git commit -m 数据区内的 rm 字面量)。
				//     Safe 来自优先级最高的解析器(如 git),对所有有解析器的域规则生效 ——
				//     若 git 判 Safe(commit -m 数据区),filesystem 规则也不应报(避免误报)。
				//   Unknown → 走正则
				switch semResult.Decision {
				case semantics.Deny:
					if domain == semDenyRuleDomain {
						// 对首条匹配规则构造语义 finding(去重)。
						// 优先用 dcg_rule_id == sem.RuleID 的规则(精确匹配,继承正确 severity/remediation);
						// 若无精确匹配(snowflake.drop 是通用语义 RuleID,无对应 dcg_rule_id),
						// 用该域首条规则做载体(severity 取规则自己的,但 RuleID 用 semantic.* 覆盖)。
						if !semFindingEmitted {
							f := makeSemanticFinding(d, r, a, *semResult)
							fp := ruleengine.Fingerprint(r, a.ID)
							applySuppression(&f, fp, d.baseline, d.supprs)
							out = append(out, f)
							semFindingEmitted = true
						}
						continue // 该域所有规则跳过正则(语义已判破坏)
					}
				case semantics.Safe:
					continue // 跳过该规则(语义判安全,防误报)
				}
			}

			res := ruleengine.Eval(r, a)
			if !res.Matched {
				continue
			}

			// 关卡 2:正则后语义 Safe 复核(防误报)。
			// 正则命中,但若语义判 Safe,丢弃 finding(如正则对 git commit -m 数据区
			// 内 rm 字面量误报,语义 Safe 复核抑制)。
			// Deny 在关卡 1 已处理(不会走到这里,semDenyRuleDomain 域规则已 continue);
			// Unknown 不复核(正则命中即报)。
			if semActive && semResult.Decision == semantics.Safe {
				continue // 丢弃正则误报
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

// makeSemanticFinding 构造语义 Deny finding(关卡 1 用)。
//
// 与正则 finding 的区别:
//   - RuleID:用 `semantic.<dcg_rule_id>`(如 semantic.filesystem.rm-rf-root-home),
//     与正则规则 ID(destructive.filesystem.rm-rf-root-home)区分,便于 UI/审计追溯来源。
//   - Evidence:用 sem.Reason(语义解析器的判定理由,如 "rm -rf /(递归强制删根/home)")
//   - Severity/Locations:语义解析器不产位置信息,Locations 留空;Severity 复用规则的 severity
//     (规则是 destructive.filesystem.rm-rf-root-home → critical,语义 finding 继承)
//
// RuleID 映射:sem.RuleID 是 dcg 风格(如 "filesystem.rm-rf-root-home" / "git.reset-hard"),
// 加 "semantic." 前缀避免与正则规则 ID 冲突(正则规则用 "destructive." 前缀)。
// 不查找匹配的 sentinel 规则取其 ID —— 简化实现,且语义 finding 与正则 finding
// 可同时出现(若用户想区分来源)。Fingerprint 仍用规则 r 算(稳定锚定规则意图,
// 不受 Evidence 文本变化影响)。
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
