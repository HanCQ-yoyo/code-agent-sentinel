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
	Rules []Rule `yaml:"rules"`
}

// LoadBuiltin 从 go:embed 内置规则 FS 加载规则。每条规则 Source 标 "builtin"。
// 仅解析 YAML,不做 Validate(Validate 在 LoadForScan 统一执行)。
func LoadBuiltin() ([]Rule, []RuleLoadError) {
	var rules []Rule
	var errs []RuleLoadError

	entries, err := builtinRuleFS.ReadDir("rules")
	if err != nil {
		errs = append(errs, RuleLoadError{Source: "builtin", Reason: fmt.Sprintf("read embedded rules dir: %v", err)})
		return nil, errs
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
		parsed, parseErrs := parseRuleYAML(data, "builtin")
		errs = append(errs, parseErrs...)
		rules = append(rules, parsed...)
	}
	return rules, errs
}

// LoadDir 从目录加载 *.yaml 规则文件。目录不存在返回 (nil, nil)。
// source 是来源前缀("global"/"project"),每条规则 Source = source + ":" + 文件路径。
// 仅解析 YAML,不做 Validate。
func LoadDir(dir, source string) ([]Rule, []RuleLoadError) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []RuleLoadError{{Source: source + ":" + dir, Reason: fmt.Sprintf("stat dir: %v", err)}}
	}
	if !info.IsDir() {
		return nil, []RuleLoadError{{Source: source + ":" + dir, Reason: fmt.Sprintf("not a directory: %s", dir)}}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []RuleLoadError{{Source: source + ":" + dir, Reason: fmt.Sprintf("read dir: %v", err)}}
	}

	var rules []Rule
	var errs []RuleLoadError
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
		parsed, parseErrs := parseRuleYAML(data, source+":"+path)
		errs = append(errs, parseErrs...)
		rules = append(rules, parsed...)
	}
	return rules, errs
}

// Merge 按层合并规则:同 (id, projectPath) 后者整条替换前者(保持首次出现位置);
// 新 id 按各层 encounter 顺序追加到末尾。不同 projectPath 的同名规则各自保留
// (builtin/global projectPath 为空,仍按 id 覆盖)。
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
// 项目规则带 projectPath 字段(求值时只对该项目资产生效,Task 11 RulesDetector 检查)。
// projectPath 在 Merge 前设置:Merge 的整条替换会保留带 projectPath 的项目版本,
// 使其覆盖 builtin/global 的同 id 规则时 projectPath 随之生效。
func LoadForScan(home string, inventory *configengine.Inventory) ([]Rule, []RuleLoadError) {
	builtin, errs := LoadBuiltin()

	globalDir := filepath.Join(home, ".claude-sentinel", "rules")
	global, globalErrs := LoadDir(globalDir, "global")
	errs = append(errs, globalErrs...)

	var projectRules []Rule
	if inventory != nil {
		for _, p := range inventory.Projects {
			dir := filepath.Join(p.Path, ".sentinel", "rules")
			prules, perrs := LoadDir(dir, "project")
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
	return valid, errs
}

// parseRuleYAML 解析 YAML 字节流为 Rule 列表,统一设置 Source。
func parseRuleYAML(data []byte, source string) ([]Rule, []RuleLoadError) {
	var rf ruleFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, []RuleLoadError{{Source: source, Reason: fmt.Sprintf("yaml parse: %v", err)}}
	}
	for i := range rf.Rules {
		rf.Rules[i].Source = source
	}
	return rf.Rules, nil
}

// hasYamlExt 判断文件名是否为 .yaml 或 .yml。
func hasYamlExt(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}
