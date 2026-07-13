package security

import (
	"context"
	"fmt"
	"path/filepath"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security/ruleengine"
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

	baseline *suppression.BaselineSet   // 已知指纹快照(命中 → Suppression="baseline");nil=无
	supprs   *suppression.Suppressions  // 行内豁免(命中 → Suppression="inline");nil=无
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

func (d *RulesDetector) ID() string                     { return "rules" }
func (d *RulesDetector) Covers() []configengine.AssetType { return nil } // 见类型注释
func (d *RulesDetector) Enabled() bool                    { return d.cfg.RulesEnabled() }
func (d *RulesDetector) Available() bool                 { return true }
func (d *RulesDetector) Reason() string                  { return "" }

// Meta 返回检测器能力元数据(UI 展示用,纯静态描述)。
// Rules 摘要来自 baseRules(内置 + 全局);项目规则是动态的、按资产来源项目变化,不入 Meta。
func (d *RulesDetector) Meta() DetectorMeta {
	rules := make([]RuleInfo, 0, len(d.baseRules))
	for _, r := range d.baseRules {
		rules = append(rules, RuleInfo{
			ID:          r.ID,
			Severity:    r.Severity,
			Description: r.Description,
			Syntax:      r.Syntax(),
		})
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
		for _, r := range allRules {
			// 规则按 asset_type 路由(Covers=nil → 全资产传入,内部按类型分发)
			if string(r.AssetType) != string(a.Type) {
				continue
			}
			// 项目规则隔离:ProjectPath 非空 → 只对该项目(SourcePath 在项目根下)的资产生效
			if r.ProjectPath != "" && !pathInProject(a.SourcePath, r.ProjectPath) {
				continue
			}
			matched, evidence := ruleengine.Eval(r, a)
			if !matched {
				continue
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
				Evidence:    truncate(evidence, 200),
				Remediation: r.Remediation,
				Fingerprint: fp,
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
			DetectorID: d.ID(),
			RuleID:     "rules.load-error",
			Severity:   SeverityInfo,
			AssetID:    "rules:" + e.Source,
			Message:    "规则加载错误",
			Evidence:   e.Reason,
			Remediation: "修复规则文件语法或配置(详见 evidence)",
		})
	}
	return out, nil
}

// loadProjectRules 加载所有已知项目的 ~/.sentinel/rules/ 规则,设置 ProjectPath 标记。
// 项目列表来自 ~/.claude.json(ListProjects);文件缺失/损坏 → 空列表(安全降级)。
func (d *RulesDetector) loadProjectRules() ([]ruleengine.Rule, []ruleengine.RuleLoadError) {
	eng := configengine.NewEngine(d.home)
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
