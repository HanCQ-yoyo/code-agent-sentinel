package security

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security/findingstate"
	"code-agent-sentinel/internal/security/ruleengine"
	"code-agent-sentinel/internal/security/ruleengine/semantics"
	"code-agent-sentinel/internal/storage"
)

// RulesDetector 是统一声明式规则引擎检测器,替代旧 BaselineDetector + InjectionDetector。
//
// 它从 sqlite db 读内置 + custom 规则(Task 8:每次 Scan 实时读 → 热重载,API 改规则后
// 下次扫描即时生效),叠加项目级 .sentinel/rules 文件规则(经 ruleengine.LoadDetectRules:
// Merge + Validate),对每个资产按规则 asset_type 路由求值;命中产 Finding,施加处置状态
// (applyFindingState);规则加载/校验错误产独立的 load-error Finding(Severity=Info,
// 不进健康分)。
//
// 设计要点(controller 预飞行决议):
//   - Covers() 返回 nil —— orchestrator 的 filterByCovers 在 covers 为空时传全部资产,
//     RulesDetector 内部按 r.AssetType 路由(各资产只跑匹配类型规则),等价于旧两个检测器
//     各自声明 Covers 的并集,且天然支持未来新 asset_type 的规则(无需改 Covers)。
//   - 加载分层:db 规则(builtin + custom)+ 项目规则在每次 Scan 经 LoadDetectRules 实时
//     读 + Merge + Validate(热重载);combos 在构造时从 db 读一次预编译(预编译昂贵且 custom
//     combo 不支持,故不热重载,下次启动才刷新);处置状态(finding_states)构造时加载。
//   - load-error Finding 用 SeverityInfo(系数 0.0),确保 ComputeHealth 不为其扣分
//     (见 TestRulesDetectorLoadErrorNotInHealth 的数学验证)。
//
// Finding #5 闭合(Task 8):扫描侧不再硬编码 ~/.claude-sentinel 全局规则目录,统一读 db。
// 路径覆盖(用户在 config 改 sentinel_rules_dir)由写侧同步进 db(SyncBuiltin),扫描侧
// 读 db 即自动生效,不再静默不生效。
type RulesDetector struct {
	home string
	cfg  *config.DetectorsConfig
	db   *storage.DB // sqlite 句柄,Scan 时实时读(热重载)。nil=临时(Task 10 注入真 db 前),Scan 无规则。

	// combos(builtin + global):构造时从 db 读一次 + ValidateCombo 预编译。
	// 注:Scan 时规则每次重读 db,但 combos 预编译昂贵,构造时读一次。
	// custom combo 不支持,故 combos 只在启动同步后稳定(下次启动才刷新)。
	baseComboRules []ruleengine.ComboRule
	// 构造时的加载/校验错误(combos 加载/校验 + finding_states 加载)。
	// Scan 时追加本次 LoadDetectRules 的错误,合并成 load-error findings。
	loadErrs []ruleengine.RuleLoadError

	// 处置生命周期状态(Task 7 引入,Task 11 删 suppression 后独占处置生命周期)。
	// nil=无 finding_states.yaml(或加载失败),applyFindingState 安全降级为 Status="open"。
	states *findingstate.States
	// negationDropped 计数:被 IsNegatedByContext 丢弃的 content 命中数(不进处置生命周期,
	// 仅观测,可后续暴露到 DetectorStatus 或日志)。drop 不产 Finding。
	negationDropped int
}

// NewRulesDetector 构造检测器:从 db 读 combos + 加载处置状态。
//
// 规则(builtin + custom + 项目)不在构造时加载 —— Scan 每次实时读 db(热重载):
// API 改规则(SetOverride / UpsertRule)后下次扫描即时生效,无需重启或重建检测器。
// combos 在构造时从 db 读一次 + ValidateCombo 预编译(预编译昂贵,且 custom combo 不支持,
// 故不热重载,下次启动才刷新)。
//
// db 由 main.go 注入(WAL,server 独占写,guard 只读)。db==nil 是临时态(Task 10 前
// main.go 传 nil):LoadDetectRules(nil,...) 返回 "db is nil" 错误,Scan 无规则,只产
// load-error finding(Severity=Info 不进健康分)。Task 10 注入真 db 后即正常。
//
// 顺带修 Finding #5:扫描侧不再硬编码 ~/.claude-sentinel 全局规则目录,统一读 db。
func NewRulesDetector(home string, cfg *config.DetectorsConfig, db *storage.DB) *RulesDetector {
	d := &RulesDetector{home: home, cfg: cfg, db: db}

	// combos:构造时从 db 读 + ValidateCombo 预编译。
	// 规则在 Scan 时重读 db,但 combos 预编译昂贵且 custom combo 不支持,故构造时读一次。
	// 项目级 combos 不接(与 LoadForScan 同语义,保持"项目规则只单资产")。传 nil projects
	// → loadRulesForScan 只读 db 的 builtin combos(或 db nil 时回退 LoadForScan 只取 builtin+global
	// combos),不读项目 combo(本就丢弃)。
	_, combos, errs := d.loadRulesForScan(nil)
	validCombos, comboErrs := ruleengine.ValidateCombo(combos)
	d.baseComboRules = validCombos
	d.loadErrs = append(d.loadErrs, errs...)
	d.loadErrs = append(d.loadErrs, comboErrs...)

	// 处置生命周期状态(finding_states.yaml,仍文件,本次不迁)。
	d.loadStates()
	return d
}

// loadStates 从 sqlite db 加载处置状态到 d.states。
// db 为 nil 时 states 为 nil(applyFindingState 见 nil 安全降级为 Status="open")。
// 加载失败(err 非 nil)记 loadErr(不致命,作 load-error finding 暴露)。
func (d *RulesDetector) loadStates() {
	s := findingstate.NewStates(d.db)
	if s == nil {
		s = &findingstate.States{}
	}
	d.states = s
}

// loadRulesForScan 返回 (rules, combos, errs),统一三处规则加载点的 db-nil fail-open 回退。
//
//   - d.db 非 nil(生产 + Task 8 db-path 测试):读 sqlite 实时规则(热重载,API 改规则后下次
//     扫描即时生效)—— LoadDetectRules(db, projects) 读 builtin+custom(SQL 层过滤
//     enabled=false)+ 叠加项目级 .sentinel/rules 文件规则 + Merge + Validate。
//   - d.db == nil(临时态:main.go 未注入真 db / 旧测试传 nil):回退 legacy 文件路径
//     LoadForScan(home, inv),保持迁移前既有行为(builtin embed + ~/.claude-sentinel/rules
//     全局文件规则 + 各项目 .sentinel/rules 项目规则)。这样 storage 未就绪时不静默关闭
//     检测,既有测试不改即通过。
//
// 此回退镜像 guard 的 fail-open 模式(Task 9):storage 不可用不得静默禁用检测;
// 对检测器而言,nil db 即"storage 不可用"的情形。
//
// projects 语义:传 nil = 只读 builtin(+global 文件)规则(无项目规则,与 nil-db 下
// LoadForScan(home, nil) 一致);传非 nil = 同时加载这些项目的 .sentinel/rules。
func (d *RulesDetector) loadRulesForScan(projects []configengine.Project) (rules []ruleengine.Rule, combos []ruleengine.ComboRule, errs []ruleengine.RuleLoadError) {
	if d.db == nil {
		// 临时 nil db(main.go 未注入 / 旧测试):回退文件路径,保持既有行为。
		// LoadForScan(home, inv) = builtin embed + ~/.claude-sentinel/rules 全局文件规则;
		// inv.Projects 非空时还加载各项目 .sentinel/rules(与 db 路径项目规则加载语义一致)。
		inv := &configengine.Inventory{}
		if projects != nil {
			inv.Projects = projects
		}
		return ruleengine.LoadForScan(d.home, inv)
	}
	return ruleengine.LoadDetectRules(d.db, projects)
}

func (d *RulesDetector) ID() string                       { return "rules" }
func (d *RulesDetector) Covers() []configengine.AssetType { return nil } // 见类型注释
func (d *RulesDetector) Enabled() bool                    { return d.cfg.RulesEnabled() }
func (d *RulesDetector) Available() bool                  { return true }
func (d *RulesDetector) Reason() string                   { return "" }

// Meta 返回检测器能力元数据(UI 展示用,纯静态描述)。
// Rules 摘要实时从 db 读(builtin + custom,无项目规则——项目规则按资产来源项目动态变化,
// 不入 Meta)。db==nil 时回退 LoadForScan 读 builtin+global 文件规则(临时态,Task 10 注入真 db 后正常)。
func (d *RulesDetector) Meta() DetectorMeta {
	rules, _, _ := d.loadRulesForScan(nil)
	ri := make([]RuleInfo, 0, len(rules))
	for _, r := range rules {
		info := RuleInfo{
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
			info.Paths = &PathFilterInfo{Include: r.Paths.Include, Exclude: r.Paths.Exclude}
		}
		ri = append(ri, info)
	}
	return DetectorMeta{
		ID:      d.ID(),
		Name:    "声明式规则引擎",
		Enabled: d.Enabled(),
		Engines: []EngineInfo{{Name: "声明式规则引擎", Kind: "embedded", Enabled: d.Enabled(), Available: true}},
		Rules:   ri,
		Covers:  nil, // 与 Covers() 一致:nil = 全资产类型(内部路由)
	}
}

// Scan 对一批资产跑全部匹配规则。
func (d *RulesDetector) Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error) {
	// 每次扫描从 db 实时读规则(热重载:API 改规则后下次扫描即时生效)。
	// LoadDetectRules 内部:读 db(builtin+custom,SQL 层过滤 enabled=false)→ 叠加项目级
	// .sentinel/rules 文件规则(带 ProjectPath)→ Merge + Validate。返回已合并校验的 rules。
	//
	// 项目规则加载语义与原 LoadForScan 完全一致:从 ~/.claude.json 读已知项目(权威源),
	// 各项目 LoadDir(.sentinel/rules)、设 ProjectPath、Merge 作第二层。Scan 收 []Asset 无法
	// 直接拿 inventory,故用 ListProjects() 重建项目集传给 LoadDetectRules。
	projects, _ := d.loadProjects()
	allRules, _, loadErrs := d.loadRulesForScan(projects)

	// 加载错误:构造时的 d.loadErrs(combos + finding_states)
	// + 本次 LoadDetectRules 的错误(db 读 + 项目规则 + Validate)。
	combinedErrs := append([]ruleengine.RuleLoadError{}, d.loadErrs...) // 复制,不污染构造时快照
	combinedErrs = append(combinedErrs, loadErrs...)

	var out []Finding
	for _, a := range assets {
		// 提取命令文本(语义解析器输入):hook/mcp_server 用 command 字段,
		// script/skill/command/agent/memory 用 content 字段,permissions 无命令文本(跳过语义)。
		cmdText, _ := commandTextFromAsset(a)

		// 按行 + 行内片段语义状态(R3 升级:SplitCommand 拆片段闭合 I1 + span 复核丢 Data 区误报)。
		// 详见 computeLineSemantic / findingInSafeSpans 注释。无命令文本/permissions 资产 →
		// anySem=false,语义关卡整体跳过(纯正则)。
		semState := computeLineSemantic(cmdText)

		// 预选每域语义 finding 载体规则(关卡 1 Deny 用):
		// 按 rule_id == sem.RuleID 精确匹配(继承正确 severity/remediation);
		// 无精确匹配回退到该域首条 asset_type 匹配规则(snowflake.drop 通用 RuleID 时,
		// 修 C2 后 snowflake 语义返回具体 rule_id,通常 strategy 1 即命中)。
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
			// 放宽:destructive 域规则(metadata.source=builtin 且 metadata.domain 属 destructive 五域)
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
				//   若该域有任意片段 Executed Deny(denyByDomain[domain] 存在 —— computeLineSemantic
				//     已在聚合时跳过 Data span 的误报 Deny)→ 构造一次语义 finding(去重:
				//     emittedDomains[domain]),并 continue 跳过该域所有正则(语义已判破坏,
				//     正则不必再跑;Deny asset-wide-correct,任意片段 Deny 即触发)。
				//   Safe 不再在此无条件 continue(C1 修复):Safe 改由关卡 2 按 span 复核,
				//     避免一行/一片段 Safe 抑制另一片段的真实破坏性命令。
				//   Unknown → 走正则。
				if denyRes, ok := semState.denyByDomain[domain]; ok {
					if !emittedDomains[domain] {
						carrier := r
						if c := semCarriers[domain]; c != nil {
							carrier = *c
						}
						f := makeSemanticFinding(d, carrier, a, denyRes)
						fp := ruleengine.Fingerprint(carrier, a.ID)
						applyFindingState(&f, fp, d.states) // 统一处置生命周期
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

			// 关卡 2:正则后 span 复核(R3 升级,防误报 + span data 区丢弃)。
			// 正则命中,若该命中落在 Data/Comment span(引号内字面量/注释)则丢弃 ——
			// 抑制数据区内字面量误报(如 echo "rm -rf /" 的 rm -rf / 在引号内是字面量)。
			// 落在 Executed span(真实破坏命令)→ 保留。
			// content 字段命中:用 Location.Line/StartCol 转字节偏移查 span。
			// command 字段命中(Locations 空):用 evidence+strings.Index 重新定位字节偏移。
			// evidence 也为空 → 回退 wholeSafe(保留 R2 行为)。
			//
			// R3 扩大范围:R2 仅对有语义解析器的域(semActive)做关卡 2;R3 改为对任意有
			// 命令文本的资产(anySem)做 span 复核 —— 即便规则域无语义解析器(如 injection),
			// 其正则命中若落在 Data span(引号内字面量)也应丢弃(spec §6.5 行为变更)。
			// 无命令文本/permissions 资产 anySem=false,关卡 2 整体跳过(纯正则,permissions
			// 字段无引号边界概念)。
			if semState.anySem {
				if semState.findingInSafeSpans(res.Locations, cmdText, res.Evidence) {
					continue // 丢弃正则误报(命中在 Data/Comment span)
				}
			}

			// 关卡 2b:否定词上下文抑制(content-only,Task 7)。
			// content 字段命中若行首/匹配点前 lookBehindChars 字符内有"禁止/不允许/do-not"等
			// 否定词 → drop pre-emit(不进处置生命周期,只计计数,不产 Finding)。
			// command 字段命中(locs 空)不抑制:避免假阴性 —— `禁止: rm -rf` 注释里的
			// 否定词不能让真实破坏命令变安全(与 command 字段不进 Safe 关卡同理)。
			// semantic emit 点(上方 Deny 分支)也不加 negation:语义 Deny 是高置信破坏判定,
			// 否定词不能让真实破坏命令变安全。
			if ruleengine.IsNegatedByContext(a.Content, res.Locations) {
				d.negationDropped++
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
				Evidence:    truncate(res.Evidence, 200),
				Remediation: r.Remediation,
				Fingerprint: fp,
				Locations:   res.Locations,
			}
			applyFindingState(&f, fp, d.states) // 统一处置生命周期
			out = append(out, f)
		}
	}

	// 资产内去重(Task 7):同 asset_id + 同 Location.Line(content 同行)或同 asset_id +
	// 同 command 字段串(command 字段命中)的多条 finding 合并为一条 finding-group。
	// group 带 PrimaryRuleID(=RuleID)、ContributingRuleIDs(其他贡献规则)、Locations(并集)、
	// Severity(取最大)。仅同资产内合并,跨资产不合并(那是聚合视图的事)。
	//
	// 去重时机:在 combo 第二遍 + load-error 之前。原因:combo finding 的 Locations 为空,
	// AssetType=primary.Type(可能是 AssetHook/AssetMCPServer),若去重放最后会与真实 hook/mcp
	// finding 按相同 AssetName+Evidence 误合并。load-error finding 的 AssetID="rules:..." 合成 ID,
	// AssetType 为空(走 else 唯一负 key 分支,不误并),但为保持一致也放去重之后。combo 和
	// load-error 本身不参与去重(它们各自语义独立:combo 是跨资产 AND,load-error 是元信息)。
	out = dedupIntraAsset(out)

	// 跨资产组合规则第二遍(Task 9):同 agent 资产集内,所有 requires 同时命中(AND)
	// → 1 条 Finding 挂到 primary(首个 require 命中的资产)。
	//
	// allComboRules 来源:d.baseComboRules(builtin combos,NewRulesDetector 构造时
	// 已 ValidateCombo 预编译)。项目 combo 暂不接(LoadDetectRules 丢弃项目 combo,保持
	// "项目规则只单资产"语义,后续任务扩展)。
	//
	// 求值见 comboMatches:每个 require 用 CompiledRule()(ValidateCombo 预编译的 Rule,
	// 含 regexes 缓存)跑 ruleengine.Eval;CompiledRule() 为 nil(理论不发生)安全降级不命中。
	for _, cr := range d.baseComboRules {
		if primary, evidence, ok := comboMatches(cr, assets); ok {
			out = append(out, makeComboFinding(d, cr, primary, evidence))
		}
	}

	// load-error Finding:AssetID 用 "rules:" + e.Source(合成 ID,不在任何 inventory)。
	// Severity=Info(系数 0.0)→ ComputeHealth 不为其扣分(见 TestRulesDetectorLoadErrorNotInHealth)。
	// 这是 spec 决策 #12「load-error Finding 不进健康分」的落地:旧 brief 用 SeverityMedium,
	// 但 Medium 系数 1.5 会让合成 AssetID 以 w=1.0 兜底权重扣分,破坏该决策,故改 Info。
	for _, e := range combinedErrs {
		f := Finding{
			DetectorID:  d.ID(),
			RuleID:      "rules.load-error",
			Severity:    SeverityInfo,
			AssetID:     "rules:" + e.Source,
			Message:     "规则加载错误",
			Evidence:    e.Reason,
			Remediation: "修复规则文件语法或配置(详见 evidence)",
		}
		// load-error 无 Fingerprint(空串),applyFindingState 只设 Status="open"(无匹配状态)。
		applyFindingState(&f, "", d.states)
		out = append(out, f)
	}
	return out, nil
}

// loadProjects 返回已知项目列表(来自 ~/.claude.json ListProjects),供 LoadDetectRules
// 叠加各项目 .sentinel/rules 文件规则。文件缺失/损坏 → 空列表(安全降级,不报错)。
// LoadDetectRules 内部对每个项目 LoadDir + 设 ProjectPath + Merge,语义与原 loadProjectRules
// 一致(只是把 LoadDir/Merge 下沉进 LoadDetectRules,Scan 不再自己 Merge)。
func (d *RulesDetector) loadProjects() ([]configengine.Project, error) {
	eng := configengine.NewEngine(d.home, "")
	return eng.ListProjects()
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
//   - 放宽路由:destructive 域规则(metadata.source=builtin 且 metadata.domain 属五域之一
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
	if src != "builtin" {
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

// lineSemanticState 是按行 + 行内片段跑语义解析后的聚合状态(Task 9 R3 升级)。
//
// R1/R2:对整个命令文本跑一次 DispatchCommand,缓存单个 SemanticResult。
// 问题 1(C1 跨行漏报):任何 Safe 决策(git commit -m / rm -i / git restore --staged /
// git checkout -b)会经关卡 1 `case Safe: continue` + 关卡 2 抑制该 asset 全部规则 ——
// 包括同一 content 另一行的真实 rm -rf /(漏报:health 60 而非 20)。修(C1):按行切分。
// 问题 2(I1 链式绕过):按行整体跑 DispatchCommand,`git commit -m "x" && rm -rf /` 整体
// 判 Safe(git commit -m 优先),行内 `&& rm -rf /` 漏报。修(R3):行内 SplitCommand 拆片段,
// 每片段独立判语义。
// 问题 3(span data 区误报):filesystem 语义解析器的 rmCmdRe 正则不识别引号边界,把
// `echo "rm -rf /"` 引号内的 rm -rf / 误判 Deny;destructive.filesystem.* 正则也误命中
// 引号内字面量。修(R3):语义 Deny + 正则命中都做 span 复核,Data/Comment span 命中丢弃。
//
// R3 升级:按行切分,行内 SplitCommand 拆片段,逐片段跑 DispatchCommand,聚合:
//   - denyByDomain:每域首条 Executed-span Deny 结果(用于关卡 1 语义 finding + 该域规则 continue)。
//     Data span 内的 Deny(引号内字面量误报)在聚合时跳过,不进 denyByDomain。
//     Deny 仍 asset-wide-correct(任意片段 Executed Deny 即触发),保持破坏性兜底不漏。
//   - wholeSafe:所有片段都 Safe(单行命令或全部片段 Safe);用于无 Location 的 command
//     字段匹配(hook/mcp_server command 无行位置,按整条命令 Safe 判定丢弃)。
//   - spans/lineStarts:cmdText 的 ClassifySpans 结果 + 每行起始字节偏移,在聚合时一次算好
//     缓存,关卡 2 的 findingInSafeSpans 按 Location 转字节偏移查 span 成员时复用(避免
//     每正则命中 × 每 Location 重算 ClassifySpans,hot path 优化)。
//
// 简化:按 \n 切行,行内按 SplitCommand 拆片段;跨行命令(如 rm -rf \\\n/)可能被拆散。
// 这类情况任意片段若 Unknown → wholeSafe=false → 不抑制,正则照常命中(不漏报);仅当跨行
// 命令恰好被判 Safe 时可能误抑制,属 R1 可接受简化(review C1 明确允许 line-split 简化)。
type lineSemanticState struct {
	denyByDomain map[string]semantics.SemanticResult // 域 → 首条 Executed Deny(关卡 1 用)
	denyOrder    []string                            // Deny 域出现顺序(稳定 emit 顺序)
	wholeSafe    bool                                // 所有片段均 Safe(无 Location 命中丢弃用)
	anySem       bool                                // 是否跑了语义(有命令文本 + 非 permissions)
	spans        []ruleengine.Span                   // cmdText 的 ClassifySpans 缓存(关卡 2 复用)
	lineStarts   []int                               // cmdText 每行起始字节偏移(关卡 2 复用)
}

// computeLineSemantic 按行 + 行内片段(SplitCommand)跑 DispatchCommand,聚合 lineSemanticState。
// 无命令文本 / permissions 资产 → 返回 anySem=false(语义关卡整体跳过)。
//
// R3 升级:原按行整体判 wholeSafe 会漏行内 `&& rm -rf /`(I1);现按 SplitCommand 拆片段
// 独立判语义。`git commit -m "x" && rm -rf /` 拆成 [git commit -m "x", rm -rf /],后者
// 独立 Deny → 闭合 I1。
//
// Deny 聚合策略(关键):若首条 Deny 落在 Data span(引号内字面量,如 echo "rm -rf /" 的
// rm -rf /),跳过它继续找下一个落在 Executed span 的 Deny。原因:同一资产可能含多个片段,
// `echo "rm -rf /" ; rm -rf /` 的第 1 片段 Deny 是误报(Data span),第 2 片段 Deny 是真报
// (Executed span)。若误存第 1 片段,关卡 1 会丢弃 Deny,漏报第 2 片段的真实破坏命令。
//
// 缓存:spans = ClassifySpans(cmdText)、lineStarts = 每行起始字节偏移,在此一次算好,
// 关卡 2 的 findingInSafeSpans 按 Location 转字节偏移查 span 成员时复用(避免每正则命中
// × 每 Location 重算 ClassifySpans,hot path 优化)。
func computeLineSemantic(cmdText string) lineSemanticState {
	st := lineSemanticState{
		denyByDomain: map[string]semantics.SemanticResult{},
	}
	if cmdText == "" {
		return st
	}
	st.anySem = true
	lines := strings.Split(cmdText, "\n")
	allSafe := true
	for _, line := range lines {
		// 行内拆片段(I1 闭合):SplitCommand 按 &&/;/||/| 拆分(引号/$() 感知),
		// 每片段独立跑 DispatchCommand。单片段(无分隔符)返回 []string{line}。
		segs := ruleengine.SplitCommand(line)
		for _, seg := range segs {
			res := semantics.DispatchCommand(seg)
			switch res.Decision {
			case semantics.Deny:
				dom := mapSemDomain(res.RuleID)
				if dom != "" {
					// 跳过 Data span 内的 Deny(引号内字面量误报):echo "rm -rf /" 的 rm
					// 在 SpanData 区,语义解析器误判 Deny。不存入 denyByDomain,继续找
					// 真 Deny(Executed span)。若整资产全是 Data span Deny,该域无 Deny 记录,
					// 关卡 1 不 emit 语义 finding,正则层兜底(关卡 2 span 复核会丢弃)。
					if _, ok := st.denyByDomain[dom]; !ok && !semanticDenyInDataSpan(seg, res) {
						st.denyByDomain[dom] = res
						st.denyOrder = append(st.denyOrder, dom)
					}
				}
				allSafe = false
			case semantics.Safe:
				// Safe 片段:wholeSafe 聚合用(R2 safeLines 行级 map 已删,R3 关卡 2 用 span 复核)
			case semantics.Unknown:
				allSafe = false
			}
		}
	}
	st.wholeSafe = allSafe
	// 缓存 cmdText 的 spans + lineStarts(关卡 2 hot path 复用,避免每正则命中重算)
	st.spans = ruleengine.ClassifySpans(cmdText)
	st.lineStarts = lineStartOffsetsLocal(cmdText)
	return st
}

// mapSemDomain 把语义 RuleID 的首段映射到 sentinel 规则 domain Metadata。
//
//	git → git, filesystem → filesystem, snowflake.* → database
//
// snowflake 语义 RuleID 现返回具体 rule_id(如 snowflake.drop-database,修复 C2),
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

// findingInSafeSpans 判断正则命中是否落在 Data/Comment span(关卡 2 用,R3 升级)。
//
// R2 findingInSafeLines 按 Location.Line 是否在 safeLines 判定(行级 Safe 抑制)。
// R3 升级为 span 级:用 ClassifySpans 判定命中字节偏移落在 Executed/Data/Comment span。
//   - Data/Comment span 命中 → 丢弃(引号内字面量 / 注释,如 echo "rm -rf /" 的 rm -rf /)
//   - Executed span 命中 → 保留(真实破坏命令,交 wholeSafe 复核)
//
// 坐标系对齐(关键设计,与 Task 7 运行时层一致):
//   - ruleengine.Location 是 {Line, StartCol, EndCol} 行列模型(schema.go:110);
//     ruleengine.Span 是 {Start, End} 字节偏移模型(span.go:15)。坐标系不同,不能直接比较。
//   - content 字段命中:Locations 非空,用 st.lineStarts 把 (Line, StartCol) 转回字节偏移,
//     再查 st.spans 的 span 成员。
//   - command 字段命中:Locations 为空(eval.go:7:仅 content 字段叶子产 Location),
//     用 evidence 在 cmdText 中 strings.Index 重新定位字节偏移(方案 #3 的静态层延伸)。
//
// 无 Location + 无 Evidence → 回退 wholeSafe(保留 R2 行为:整条命令 Safe 才丢弃)。
// 命中字节偏移落在 Data/Comment span → 丢弃(true);否则保留(false)。
//
// 性能:st.spans / st.lineStarts 在 computeLineSemantic 一次算好缓存,本函数每正则命中
// 只做 O(S) span 查找(S=span 数,通常小),不重算 ClassifySpans(hot path 优化)。
func (st lineSemanticState) findingInSafeSpans(locs []ruleengine.Location, cmdText string, evidence string) bool {
	if len(locs) == 0 {
		// command 字段命中无 Location:用 evidence 重新定位字节偏移。
		// evidence 为空 → 回退 wholeSafe(保留 R2 行为)。
		if evidence == "" {
			return st.wholeSafe
		}
		offset := strings.Index(cmdText, evidence)
		if offset < 0 {
			return st.wholeSafe // evidence 在 cmdText 中找不到,回退 wholeSafe(保守)
		}
		return st.offsetInDataOrCommentSpan(offset, offset+len(evidence))
	}
	// content 字段命中:逐 Location 转 byte offset 查 span。
	// 命中多行只要有一行非 Data/Comment span 就不丢弃(保守:避免漏报跨行命中)。
	for _, loc := range locs {
		if loc.Line < 1 || loc.Line > len(st.lineStarts) {
			continue // 越界,跳过(保守:不据此判定丢弃)
		}
		lineStart := st.lineStarts[loc.Line-1]
		offset := lineStart + (loc.StartCol - 1) // 1-based col → 0-based byte offset
		end := lineStart + (loc.EndCol - 1)      // EndCol 半开,转字节偏移
		if !st.offsetInDataOrCommentSpan(offset, end) {
			return false // 此 Location 不在 Data/Comment span → 不丢弃
		}
	}
	return true // 所有 Location 都在 Data/Comment span → 丢弃
}

// offsetInDataOrCommentSpan 判断 [offset, end) 区间是否完全落在 Data/Comment span 内。
// 用 st.spans(computeLineSemantic 缓存的 ClassifySpans 结果)判定。命中起点不在任何 span
// (理论上不会发生,ClassifySpans 覆盖全文本)→ 返回 false(保守不丢弃)。
// 命中起点在 Executed span → false(真实破坏命令,保留)。
// 命中起点在 Data/Comment span → true(字面量误报,丢弃;跨边界也丢弃,与运行时 hitInDataSpan
// 保守一致:引号内字面量误报优先丢弃)。
func (st lineSemanticState) offsetInDataOrCommentSpan(offset, end int) bool {
	for _, s := range st.spans {
		if offset >= s.Start && offset < s.End {
			if s.Kind == ruleengine.SpanExecuted {
				return false // Executed 区:真实命中,不丢弃
			}
			// Data/Comment 区:命中起点在数据/注释 span → 丢弃
			return true
		}
	}
	return false // 不在任何 span(理论不发生),保守不丢弃
}

// lineStartOffsetsLocal 返回每行起始字节偏移(starts[i] = 第 i+1 行起点)。第 1 行起点 0。
// 与 ruleengine.lineStartOffsets 同实现(包级私有,不能跨包调用,在此重声明)。
// TODO(altitude): ruleengine.lineStartOffsets 导出后改为复用,消除重复(受 Task 9 范围
// 约束「不改 ruleengine 包」暂保留)。
func lineStartOffsetsLocal(text string) []int {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// semanticDenyInDataSpan 判断语义 Deny 的危险关键字是否落在 Data/Comment span(关卡 1 用,R3)。
// 语义解析器(gitCmdRe/rmCmdRe)的正则不识别引号边界,会把 echo "rm -rf /" 引号内的 rm 误判 Deny。
// 复核策略(方案 #3 的语义层延伸,与运行时 guard semanticDenyInDataSpan 一致):
// 按域提取危险关键字,检查它是否出现在任意 SpanExecuted 文本中。若关键字仅出现在
// Data/Comment span(引号内字面量)→ true(丢弃 Deny,降级 Unknown 走正则层)。
//
// 关键字提取(按已实现的语义解析器):
//   - git.*       → 子命令关键字(reset/branch/clean/push/stash/checkout 等,从 RuleID 取)
//   - filesystem.* → "rm"(rm 语义解析器只对 rm 关键字判 Deny)
//   - snowflake.* → "snow sql"(snowflake 语义解析器只对 snow sql 判 Deny)
//   - 其余 → 保守 false(未知域不剔除,走 Deny,安全不变量:宁可误拦)
//
// TODO(altitude): 与 cmd/sentinel/guard_cmd.go 的 semanticDenyInDataSpan 近似重复,
// 提取到 ruleengine 共享包可消除分歧(受 Task 9 范围约束「不改 ruleengine 包」暂保留)。
func semanticDenyInDataSpan(segText string, sem semantics.SemanticResult) bool {
	if sem.RuleID == "" || segText == "" {
		return false
	}
	keyword := semanticDenyKeyword(sem.RuleID)
	if keyword == "" {
		return false
	}
	spans := ruleengine.ClassifySpans(segText)
	// 关键字必须出现在至少一个 SpanExecuted 文本中,否则视为纯数据区字面量
	for _, s := range spans {
		if s.Kind == ruleengine.SpanExecuted && strings.Contains(s.Text, keyword) {
			return false // 关键字在 executed 区,是真命中
		}
	}
	// 关键字不在任何 executed span → 纯 Data/Comment 区字面量 → 丢弃
	return true
}

// semanticDenyKeyword 按 RuleID 返回用于 span 复核的危险关键字(小写)。
// git.reset-hard → "reset";git.branch-force-delete → "branch";filesystem.* → "rm";
// snowflake.* → "snow sql"。
// RuleID 用连字符分隔子修饰(如 git.push-force-short),关键字取 git. 后到首个 - 的部分。
//
// TODO(altitude): 与 cmd/sentinel/guard_cmd.go 的 semanticDenyKeyword 字符相同,
// 提取到 ruleengine 共享包可消除分歧(受 Task 9 范围约束「不改 ruleengine 包」暂保留)。
func semanticDenyKeyword(ruleID string) string {
	if strings.HasPrefix(ruleID, "git.") {
		rest := strings.TrimPrefix(ruleID, "git.")
		if i := strings.Index(rest, "-"); i > 0 {
			return rest[:i]
		}
		return rest
	}
	if strings.HasPrefix(ruleID, "filesystem.") {
		return "rm"
	}
	if strings.HasPrefix(ruleID, "snowflake.") {
		return "snow sql"
	}
	return ""
}

// pickSemanticCarrier 为语义 Deny finding 预选载体规则(修复 review Important #1)。
//
// 选择策略(按优先级):
//  1. 精确匹配:规则 Metadata["rule_id"] == semRuleID(如 semRuleID="filesystem.rm-rf-root-home"
//     匹配 destructive.filesystem.rm-rf-root-home 的 rule_id)。继承正确 severity/remediation。
//  2. 回退:该域首条 asset_type 匹配规则(snowflake.drop 是通用语义 RuleID,无对应 rule_id,
//     回退到首条 database 规则做载体)。
//  3. 若该域无任何匹配规则(理论不发生,Deny 必有对应域规则),返回 nil。
//
// semRuleID 是语义解析器返回的 RuleID(如 "filesystem.rm-rf-root-home" / "git.reset-hard" /
// "snowflake.drop")。规则的 rule_id 元数据与语义 RuleID 对齐。
//
// 修复前:用循环内首条域匹配规则做载体,若该规则 severity 与语义判定不匹配(如 rm -rf / →
// 首条 sed-exec-unverified high,而非 rm-rf-root-home critical),finding severity 被扭曲,
// 健康分扣分失真(underweighted 37.5%)。
func pickSemanticCarrier(rules []ruleengine.Rule, a configengine.Asset, semDenyRuleDomain, semRuleID string) *ruleengine.Rule {
	// 策略 1:精确匹配 rule_id == semRuleID。
	// 遍历全部规则(不止该域),因 semRuleID 含域段(如 "filesystem.rm-rf-root-home"),
	// rule_id 也含域段,跨域不会误匹配。
	if semRuleID != "" {
		for i := range rules {
			r := &rules[i]
			if !ruleAppliesToAsset(*r, a.Type) {
				continue
			}
			if r.ProjectPath != "" && !pathInProject(a.SourcePath, r.ProjectPath) {
				continue
			}
			if ruleID, _ := r.Metadata["rule_id"].(string); ruleID == semRuleID {
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
//   - RuleID:用 `semantic.<rule_id>`(如 semantic.filesystem.rm-rf-root-home),
//     与正则规则 ID(destructive.filesystem.rm-rf-root-home)区分,便于 UI/审计追溯来源。
//   - Evidence:用 sem.Reason(语义解析器的判定理由,如 "rm -rf /(递归强制删根/home)")
//   - Severity/Locations:语义解析器不产位置信息,Locations 留空;Severity 复用载体规则的 severity
//     (载体规则按 rule_id == sem.RuleID 精确匹配,如 rm -rf / → rm-rf-root-home critical,
//     语义 finding 继承正确 severity,健康分扣分不失真)
//
// RuleID 映射:sem.RuleID 是语义解析器返回的 RuleID(如 "filesystem.rm-rf-root-home" / "git.reset-hard"),
// 加 "semantic." 前缀避免与正则规则 ID 冲突(正则规则用 "destructive." 前缀)。
// 载体规则 r 由 pickSemanticCarrier 预选(按 rule_id 精确匹配,fallback 到域首条规则)。
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

// comboMatches 检查组合规则的所有 requires 是否在同 agent 资产集内各至少命中一个资产(AND)。
// 返回 (primaryAsset, evidence, matched)。primary = 第一个 require 命中的资产;
// evidence 列出各 require 命中的资产 ID(格式 "<asset_type>[<asset_id>]")。
//
// 求值:每个 require 用 CompiledRule()(ValidateCombo 预编译的 Rule,含 regexes 缓存)
// 跑 ruleengine.Eval。CompiledRule() 为 nil(未预编译,理论不发生)→ 安全降级不命中。
// AssetType 过滤:非空时只评估该类型资产,空=任意类型。
// 每个 require 命中一个即可(break),全部 requires 都命中 → 组合成立。
func comboMatches(cr ruleengine.ComboRule, assets []configengine.Asset) (primary configengine.Asset, evidence string, ok bool) {
	var ev []string
	first := true
	for _, req := range cr.Requires {
		hit := false
		for _, a := range assets {
			if req.AssetType != "" && a.Type != configengine.AssetType(req.AssetType) {
				continue
			}
			compiled := req.CompiledRule()
			if compiled == nil {
				// 未预编译(理论不发生,ValidateCombo 已编译);安全降级:不命中。
				continue
			}
			if res := ruleengine.Eval(*compiled, a); res.Matched {
				if first {
					primary = a
					first = false
				}
				ev = append(ev, fmt.Sprintf("%s[%s]", req.AssetType, a.ID))
				hit = true
				break // 每个 require 命中一个即可
			}
		}
		if !hit {
			return primary, "", false // 某 require 未命中 → 组合不成立
		}
	}
	return primary, strings.Join(ev, ", "), true
}

// makeComboFinding 构造组合规则的 Finding,挂到 primary 资产。
// 镜像现有正则 finding(rules_detector.go:254-268)的构造:
//   - Severity 用 Severity(cr.Severity) 强转(已 ValidateCombo 校验为合法枚举,无需 SeverityFromString)
//   - Fingerprint 用 combo rule ID 算(构造临时 Rule{ID: cr.ID} 传 Fingerprint)
//   - applyFindingState 施加处置生命周期(Task 11 删 suppression 后统一)
//   - Locations 留空(组合规则无单点命中位置)
func makeComboFinding(d *RulesDetector, cr ruleengine.ComboRule, primary configengine.Asset, evidence string) Finding {
	fp := ruleengine.Fingerprint(ruleengine.Rule{ID: cr.ID}, primary.ID)
	f := Finding{
		DetectorID:  d.ID(),
		RuleID:      cr.ID,
		Severity:    Severity(cr.Severity),
		AssetID:     primary.ID,
		AssetType:   primary.Type,
		AssetName:   primary.Name,
		Message:     cr.Description,
		Evidence:    "组合命中: " + evidence,
		Remediation: cr.Remediation,
		Fingerprint: fp,
		// Locations 留空(组合规则无单点命中位置)
	}
	applyFindingState(&f, fp, d.states) // 统一处置生命周期
	return f
}
