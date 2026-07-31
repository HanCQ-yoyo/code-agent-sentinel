package ruleengine

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"code-agent-sentinel/internal/configengine"
	"gopkg.in/yaml.v3"
)

//go:embed rules/*.yaml
var builtinRuleFS embed.FS

// ruleFile 是规则 YAML 文件的顶层结构。
type ruleFile struct {
	Rules      []Rule      `yaml:"rules"`
	ComboRules []ComboRule `yaml:"combo_rules"`
}

// LoadBuiltin 从 go:embed 内置规则 FS 加载规则。每条规则 Source 标 "builtin"。
// 仅解析 YAML,不做 Validate(Validate 在 LoadForScan 统一执行)。
// 返回 (rules, combos, errs):combos 是 combo_rules 段(Task 8 加,Task 9 求值)。
func LoadBuiltin() (rules []Rule, combos []ComboRule, errs []RuleLoadError) {
	entries, err := builtinRuleFS.ReadDir("rules")
	if err != nil {
		errs = append(errs, RuleLoadError{Source: "builtin", Reason: fmt.Sprintf("read embedded rules dir: %v", err)})
		return nil, nil, errs
	}
	for _, entry := range entries {
		if entry.IsDir() || !hasYamlExt(entry.Name()) {
			continue
		}
		path := "rules/" + entry.Name()
		data, err := builtinRuleFS.ReadFile(path)
		if err != nil {
			errs = append(errs, RuleLoadError{Source: "builtin:" + path, Reason: fmt.Sprintf("read: %v", err)})
			continue
		}
		parsed, parsedCombos, parseErrs := parseRuleYAML(data, "builtin")
		errs = append(errs, parseErrs...)
		rules = append(rules, parsed...)
		combos = append(combos, parsedCombos...)
	}
	return rules, combos, errs
}

// LoadInterceptBuiltin 只加载 destructive_commands.yaml(运行时拦截 builtin 规则来源)。
//
// 拦截域只该用破坏性命令规则(destructive.git.* / destructive.filesystem.* 等),
// baseline/injection/skill 等检测规则不进拦截表。syncBuiltinRules 用本函数给 intercept
// 域同步 builtin 行,确保拦截规则表只含拦截规则。
//
// 只读 embed 内的 rules/destructive_commands.yaml 单文件(Source 标 "builtin"),
// 不做 Validate(Validate 在 LoadInterceptRules 统一执行)。combos 恒为 nil
// (guard 不消费 combo 规则,见 db_init.go syncBuiltinRules 注释)。
func LoadInterceptBuiltin() (rules []Rule, combos []ComboRule, errs []RuleLoadError) {
	const path = "rules/destructive_commands.yaml"
	data, err := builtinRuleFS.ReadFile(path)
	if err != nil {
		return nil, nil, []RuleLoadError{{Source: "builtin:" + path, Reason: fmt.Sprintf("read: %v", err)}}
	}
	parsed, _, parseErrs := parseRuleYAML(data, "builtin")
	return parsed, nil, parseErrs
}

// LoadDir 从目录加载 *.yaml 规则文件。目录不存在返回 (nil, nil, nil)。
// source 是来源前缀("global"/"project"),每条规则 Source = source + ":" + 文件路径。
// 仅解析 YAML,不做 Validate。
// 返回 (rules, combos, errs):combos 是 combo_rules 段。
func LoadDir(dir, source string) (rules []Rule, combos []ComboRule, errs []RuleLoadError) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, []RuleLoadError{{Source: source + ":" + dir, Reason: fmt.Sprintf("stat dir: %v", err)}}
	}
	if !info.IsDir() {
		return nil, nil, []RuleLoadError{{Source: source + ":" + dir, Reason: fmt.Sprintf("not a directory: %s", dir)}}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, []RuleLoadError{{Source: source + ":" + dir, Reason: fmt.Sprintf("read dir: %v", err)}}
	}

	for _, entry := range entries {
		if entry.IsDir() || !hasYamlExt(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, RuleLoadError{Source: source + ":" + path, Reason: fmt.Sprintf("read: %v", err)})
			continue
		}
		parsed, parsedCombos, parseErrs := parseRuleYAML(data, source+":"+path)
		errs = append(errs, parseErrs...)
		rules = append(rules, parsed...)
		combos = append(combos, parsedCombos...)
	}
	return rules, combos, errs
}

// Merge 按层合并规则:同 (id, projectPath) 后者整条替换前者(保持首次出现位置);
// 新 id 按各层 encounter 顺序追加到末尾。不同 projectPath 的同名规则各自保留
// (builtin/global projectPath 为空,仍按 id 覆盖)。
//
// TODO(spec §2 分歧,2026-07-13 final review):spec 要求「同 id 覆盖(项目>全局>内置)」,
// 即项目规则应能覆盖/禁用同 id 内置规则(只对本项目)。当前复合键设计使项目规则与内置规则
// 「共存」——用户写一条项目规则(match 缺失)想只对本项目禁用某内置规则时,内置规则仍会对该项目
// 资产生效。全局覆盖内置(常见禁用场景)工作正常,缺口仅在「只对单个项目禁用内置」这一窄场景。
// 实现项目级覆盖需在 RulesDetector.Scan 求值时按 (asset 所在项目, rule id) 抑制同 id 内置/全局
// 规则,而非改 Merge(复合键为多项目同 id 隔离所必需)。经用户决策:先合并,后续任务补。
func Merge(layers ...[]Rule) []Rule {
	var merged []Rule
	index := make(map[string]int) // id+"|"+projectPath → 在 merged 中的位置

	for _, layer := range layers {
		for _, r := range layer {
			key := r.ID + "|" + r.ProjectPath
			if pos, ok := index[key]; ok {
				merged[pos] = r // 整条替换
			} else {
				index[key] = len(merged)
				merged = append(merged, r)
			}
		}
	}
	return merged
}

// LoadForScan 加载全部规则(builtin → global → 各 project),合并后跑 Validate。
// 项目规则带 ProjectPath 字段(求值时只对该项目资产生效,Task 11 RulesDetector 检查)。
// ProjectPath 在 Merge 前设置:Merge 用 (id, ProjectPath) 复合键去重,因此不同项目的
// 同 id 规则各自共存(不互相覆盖),而 builtin/global(ProjectPath 为空)仍按 id 覆盖;
// 项目规则的 ProjectPath 被保留,供 Task 11 RulesDetector 按项目对规则作用域收窄。
//
// 返回 (rules, combos, errs):
//   - rules:单资产 Rule,经 Merge + Validate。
//   - combos:builtin + global 的 ComboRule(项目 combo 暂不接,保持"项目规则只单资产"语义)。
//     combo 不经 Merge(无 id 覆盖语义,直接 concat),也不经 Validate——返回 raw,
//     由 RulesDetector 构造时调 ValidateCombo 预编译(controller addendum 决议 4)。
//
// Deprecated:DB 模式下主加载器是 LoadDetectRules(从 sqlite 读 detect_rules + overrides,
// enabled 过滤、热重载)。本函数仅作为 RulesDetector 的 nil-db fail-open 回退保留
// (internal/security/rules_detector.go:131,db 不可用时维持检测能力,与 guard fail-open
// 同语义),以及 load_test.go 的旧文件路径等价性测试。新代码应调 LoadDetectRules(db, projects)。
// 不得删除:删除会让 RulesDetector 在 db 故障时丢失检测能力,违反"存储故障不致检测失效"铁律。
func LoadForScan(home string, inventory *configengine.Inventory) (rules []Rule, combos []ComboRule, errs []RuleLoadError) {
	builtin, builtinCombos, errs := LoadBuiltin()
	combos = append(combos, builtinCombos...)

	globalDir := filepath.Join(home, ".claude-sentinel", "rules")
	global, globalCombos, globalErrs := LoadDir(globalDir, "global")
	errs = append(errs, globalErrs...)
	combos = append(combos, globalCombos...)

	var projectRules []Rule
	if inventory != nil {
		for _, p := range inventory.Projects {
			dir := filepath.Join(p.Path, ".sentinel", "rules")
			// 项目 combo 暂不接(_ 丢弃):保持"项目规则只单资产"语义,Task 8 范畴不扩。
			prules, _, perrs := LoadDir(dir, "project")
			errs = append(errs, perrs...)
			for i := range prules {
				prules[i].ProjectPath = p.Path
			}
			projectRules = append(projectRules, prules...)
		}
	}

	merged := Merge(builtin, global, projectRules)
	valid, validateErrs := Validate(merged)
	errs = append(errs, validateErrs...)
	return valid, combos, errs
}

// parseRuleYAML 解析 YAML 字节流为 Rule 列表 + ComboRule 列表,统一设置 Source。
// 返回 (rules, combos, errs):rules 来自 rules 段,combos 来自 combo_rules 段。
func parseRuleYAML(data []byte, source string) (rules []Rule, combos []ComboRule, errs []RuleLoadError) {
	var rf ruleFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, nil, []RuleLoadError{{Source: source, Reason: fmt.Sprintf("yaml parse: %v", err)}}
	}
	for i := range rf.Rules {
		rf.Rules[i].Source = source
	}
	for i := range rf.ComboRules {
		rf.ComboRules[i].Source = source
	}
	return rf.Rules, rf.ComboRules, nil
}

// hasYamlExt 判断文件名是否为 .yaml 或 .yml。
func hasYamlExt(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}
