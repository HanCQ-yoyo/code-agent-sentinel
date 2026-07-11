package ruleengine

import (
	"errors"
	"fmt"
	"regexp"

	"code-agent-sentinel/internal/configengine"
)

// validSeverity 判断 severity 是否在合法枚举内。
func validSeverity(s string) bool {
	switch s {
	case "info", "low", "medium", "high", "critical":
		return true
	}
	return false
}

// validAssetType 判断 asset_type 字符串是否匹配 configengine 的 11 个常量之一。
func validAssetType(s string) bool {
	switch configengine.AssetType(s) {
	case configengine.AssetSettings, configengine.AssetPermissions, configengine.AssetHook,
		configengine.AssetMCPServer, configengine.AssetSkill, configengine.AssetCommand,
		configengine.AssetAgent, configengine.AssetPlugin, configengine.AssetMemory,
		configengine.AssetKeybinding, configengine.AssetScript:
		return true
	}
	return false
}

// compileRegexPattern 按 全局约束注入前缀后编译正则。
// 返回编译好的 Regexp 或编译错误。
func compileRegexPattern(pattern string, dotall bool) (*regexp.Regexp, error) {
	full := "(?i)(?m)"
	if dotall {
		full += "(?s)"
	}
	full += pattern
	return regexp.Compile(full)
}

// Validate 对一批规则做 schema 校验 + 正则编译 + match 树递归校验。
// 合法规则(含禁用规则)进 valid,非法规则不进 valid 其错误进 errs。
func Validate(rules []Rule) (valid []Rule, errs []RuleLoadError) {
	for i := range rules {
		r := &rules[i]
		if err := validateRule(r); err != nil {
			errs = append(errs, RuleLoadError{
				Source: r.Source,
				RuleID: r.ID,
				Reason: err.Error(),
			})
		} else {
			valid = append(valid, *r)
		}
	}
	return valid, errs
}

// validateRule 校验单条规则,校验通过时填充编译态字段(assetType, regexes)。
func validateRule(r *Rule) error {
	// 1. 必填字段
	if r.ID == "" {
		return errors.New("missing required field: id")
	}
	if r.Severity == "" {
		return errors.New("missing required field: severity")
	}
	if !validSeverity(r.Severity) {
		return fmt.Errorf("invalid severity %q (want info|low|medium|high|critical)", r.Severity)
	}
	if r.AssetType == "" {
		return errors.New("missing required field: asset_type")
	}
	if !validAssetType(r.AssetType) {
		return fmt.Errorf("invalid asset_type %q", r.AssetType)
	}

	// 填充编译态 assetType
	r.assetType = configengine.AssetType(r.AssetType)

	// 2. post_exclude 正则编译验证(全量校验:即使规则禁用也验证,不缓存——Task 5 求值器重建)
	for _, pat := range r.PostExclude {
		if _, err := compileRegexPattern(pat, r.Dotall); err != nil {
			return fmt.Errorf("post_exclude regex %q compile failed: %v", pat, err)
		}
	}

	// 3. match 可空(禁用,合法)
	if len(r.Match.raw) == 0 {
		return nil
	}

	// 4. 递归校验 match 树(同时编译正则存入 r.regexes)
	if err := validateMatchRaw(r.Match.raw, r); err != nil {
		return err
	}

	return nil
}

// validateMatchRaw 递归校验 match 节点的 raw map。
func validateMatchRaw(raw map[string]any, r *Rule) error {
	if len(raw) == 0 {
		return errors.New("empty match node")
	}

	// 统计布尔操作符键(and/or/not)的个数
	boolKeys := 0
	for _, k := range []string{"and", "or", "not"} {
		if _, ok := raw[k]; ok {
			boolKeys++
		}
	}

	switch {
	case boolKeys > 1:
		return errors.New("match node has multiple boolean operators (and/or/not)")

	case boolKeys == 1:
		// 布尔节点不允许混入叶子字段
		if _, hasField := raw["field"]; hasField {
			return errors.New("cannot mix boolean operator with 'field'")
		}
		if _, hasOp := raw["op"]; hasOp {
			return errors.New("cannot mix boolean operator with 'op'")
		}
		if v, ok := raw["and"]; ok {
			return validateAndOr(v, "and", r)
		}
		if v, ok := raw["or"]; ok {
			return validateAndOr(v, "or", r)
		}
		if v, ok := raw["not"]; ok {
			return validateNot(v, r)
		}

	default:
		// boolKeys == 0:叶子节点
		return validateLeaf(raw, r)
	}
	// unreachable: boolKeys==1 guarantees one of and/or/not fires
	return nil
}

// validateLeaf 校验叶子节点:field + op + value(按 op 契约)。
func validateLeaf(raw map[string]any, r *Rule) error {
	fieldVal, hasField := raw["field"]
	if !hasField {
		return errors.New("leaf node missing 'field'")
	}
	fieldStr, ok := fieldVal.(string)
	if !ok {
		return errors.New("'field' must be a string")
	}

	opVal, hasOp := raw["op"]
	if !hasOp {
		return errors.New("leaf node missing 'op'")
	}
	opStr, ok := opVal.(string)
	if !ok {
		return errors.New("'op' must be a string")
	}
	if !validOp(opStr) {
		return fmt.Errorf("invalid op %q", opStr)
	}

	value, hasValue := raw["value"]

	// value 按 op 契约校验
	switch opStr {
	case OpExists, OpNotExists:
		// value 可缺(忽略)

	case OpWithin, OpNotWithin:
		if !hasValue {
			return fmt.Errorf("op %q requires 'value' as non-empty array", opStr)
		}
		arr, ok := value.([]any)
		if !ok {
			return fmt.Errorf("op %q requires 'value' as array, got %T", opStr, value)
		}
		if len(arr) == 0 {
			return fmt.Errorf("op %q requires non-empty array", opStr)
		}

	default:
		// 其余 op:value 必须是标量字符串
		if !hasValue {
			return fmt.Errorf("op %q requires 'value' as string", opStr)
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("op %q requires 'value' as string, got %T", opStr, value)
		}
	}

	// 正则编译(regex_match / not_regex_match / key_matches)
	if opStr == OpRegexMatch || opStr == OpNotRegexMatch || opStr == OpKeyMatches {
		valStr, ok := value.(string)
		if !ok {
			// 类型错误应已在上面 default 分支报出,这里防御
			return fmt.Errorf("op %q requires 'value' as string for regex", opStr)
		}
		compiled, err := compileRegexPattern(valStr, r.Dotall)
		if err != nil {
			return fmt.Errorf("regex compile failed (op=%s field=%s): %v", opStr, fieldStr, err)
		}
		if r.regexes == nil {
			r.regexes = make(map[string]*regexp.Regexp)
		}
		// key = op:field:value,Task 4 求值器按同样规则构造 key 读取。
		// 含 value 避免同 op+field 不同 pattern 的叶子互相覆盖。
		r.regexes[opStr+":"+fieldStr+":"+valStr] = compiled
	}

	return nil
}

// validateAndOr 校验 and/or 节点:值是 ≥1 元素 list,递归校验每个元素。
func validateAndOr(v any, op string, r *Rule) error {
	list, ok := v.([]any)
	if !ok {
		return fmt.Errorf("op %q requires a list, got %T", op, v)
	}
	if len(list) == 0 {
		return fmt.Errorf("op %q requires at least one element", op)
	}
	for i, elem := range list {
		childRaw, ok := elem.(map[string]any)
		if !ok {
			return fmt.Errorf("op %q element[%d] must be a map, got %T", op, i, elem)
		}
		if err := validateMatchRaw(childRaw, r); err != nil {
			return fmt.Errorf("op %q element[%d]: %v", op, i, err)
		}
	}
	return nil
}

// validateNot 校验 not 节点:值是单个 map(恰一子节点),递归校验。
func validateNot(v any, r *Rule) error {
	childRaw, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("op 'not' requires a single map child, got %T", v)
	}
	return validateMatchRaw(childRaw, r)
}
