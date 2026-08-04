package main

import (
	"fmt"
	"os"
	"path/filepath"

	"code-agent-sentinel/internal/security/ruleengine"
	"code-agent-sentinel/internal/storage"
)

// ruleEngineVersion 是 builtin 规则同步进 db 时打标的引擎版本。
// 升级内置规则后改此版本号,SyncBuiltin 会刷新 db 里的 builtin 行(覆盖同版本 / 删旧版本残留)。
// 当前为 "v1"(Task 10 首次接入 sqlite 持久化)。
const ruleEngineVersion = "v1"

// syncBuiltinRules 把 embed 内置规则同步进 db 的两个域(detect + intercept)。
//
// 每次启动都跑(SyncBuiltin 幂等):引擎版本升级后新增/修改的 builtin 规则会被刷新进 db,
// 被删的 builtin 行会被清掉(custom 行与 overrides 不动)。这是 RulesDetector 从 db 读
// builtin 规则的前提 —— 不同步则 db 的 builtin 行为空,检测器只能靠 nil-db 文件回退。
//
// detect 域带 combos(跨资产组合规则,检测器构造时读一次预编译);
// intercept 域 combos 传 nil(运行时拦截当前不消费 combo 规则,guard 只用单资产规则)。
//
// 同步失败不阻断启动(打 stderr,检测器/guard 已 fail-open)。
func syncBuiltinRules(db *storage.DB) {
	if db == nil {
		return
	}
	builtin, builtinCombos, loadErrs := ruleengine.LoadBuiltin()
	for _, e := range loadErrs {
		fmt.Fprintf(os.Stderr, "加载 builtin 规则错误: %s: %s\n", e.Source, e.Reason)
	}

	builtinStored, err := rulesToStoredRules(builtin, ruleEngineVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "转换 builtin 规则失败,跳过同步: %v\n", err)
		return
	}
	builtinComboStored, err := combosToStoredRules(builtinCombos, ruleEngineVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "转换 builtin combos 失败,跳过 combos 同步: %v\n", err)
		builtinComboStored = nil // combos 失败不阻断 rules 同步
	}

	if _, err := storage.SyncBuiltin(db, storage.DomainDetect, builtinStored, builtinComboStored, ruleEngineVersion); err != nil {
		fmt.Fprintf(os.Stderr, "同步 builtin 检测规则失败: %v\n", err)
	}
	// intercept 域:只同步 destructive_commands.yaml 的规则(运行时拦截只用破坏性命令规则)。
	// baseline/injection/skill 等检测规则不进拦截表。combos 传 nil(guard 不消费 combo)。
	// SyncBuiltin 的 deleteStaleBuiltin 会清掉 intercept 表里不在本批的旧 builtin 行
	// (从「全量 builtin」改成「只 destructive」的一次性数据收敛:研发阶段接受)。
	interceptBuiltin, _, _ := ruleengine.LoadInterceptBuiltin()
	interceptStored, err := rulesToStoredRules(interceptBuiltin, ruleEngineVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "转换 intercept builtin 规则失败,跳过同步: %v\n", err)
		return
	}
	if _, err := storage.SyncBuiltin(db, storage.DomainIntercept, interceptStored, nil, ruleEngineVersion); err != nil {
		fmt.Fprintf(os.Stderr, "同步 builtin 拦截规则失败: %v\n", err)
	}
}

// rulesToStoredRules 把 []ruleengine.Rule 转成 []storage.StoredRule(SyncBuiltin 输入)。
// source 统一标 "builtin"(SyncBuiltin 内部也强制 source='builtin',此处保持一致)。
func rulesToStoredRules(rules []ruleengine.Rule, version string) ([]storage.StoredRule, error) {
	out := make([]storage.StoredRule, 0, len(rules))
	for _, r := range rules {
		s, err := ruleengine.RuleToStoredRule(r, "builtin", version)
		if err != nil {
			return nil, fmt.Errorf("convert rule %s: %w", r.ID, err)
		}
		out = append(out, s)
	}
	return out, nil
}

// combosToStoredRules 把 []ruleengine.ComboRule 转成 []storage.StoredCombo(SyncBuiltin 输入)。
func combosToStoredRules(combos []ruleengine.ComboRule, version string) ([]storage.StoredCombo, error) {
	out := make([]storage.StoredCombo, 0, len(combos))
	for _, c := range combos {
		s, err := ruleengine.ComboToStoredCombo(c, "builtin", version)
		if err != nil {
			return nil, fmt.Errorf("convert combo %s: %w", c.ID, err)
		}
		out = append(out, s)
	}
	return out, nil
}

// migrateLegacyRulesFiles 把旧 ~/.code-agent-sentinel/rules/*.yaml 全局规则迁进 db 为 custom 行,
// 然后把旧文件重命名 .legacy(保留回滚,不删)。
//
// 这是解耦版 MigrateLegacyRules 的编排层(main.go 侧职责):
//  1. ruleengine.LoadDir 读旧目录(*.yaml/*.yml);
//  2. ruleengine.RuleToStoredRule 把每条 Rule 转 StoredRule;
//  3. storage.MigrateLegacyRules 把 StoredRule 写进 db 为 custom 行(detect + intercept 两域);
//  4. 把旧 *.yaml/*.yml 重命名 .legacy(只在有规则迁入或文件存在时重命名 —— 即使转换失败
//     也重命名,避免下次启动反复尝试解析坏文件;坏文件的转换错误已由 LoadDir/RuleToStoredRule
//     返回,此处打 stderr)。
//
// 项目级 <project>/.sentinel/rules 不在此迁(随项目走,运行时 LoadDetectRules 直接读文件)。
// 旧目录不存在 → LoadDir 返回 (nil,nil,nil),无操作。
//
// 仅在首次建表(SchemaInitialized=false)时由 main.go 调用,避免每次启动重复扫旧目录。
func migrateLegacyRulesFiles(home string, db *storage.DB) {
	if db == nil {
		return
	}
	oldDir := filepath.Join(home, ".code-agent-sentinel", "rules")
	rules, _, loadErrs := ruleengine.LoadDir(oldDir, "global")
	for _, e := range loadErrs {
		fmt.Fprintf(os.Stderr, "迁移旧规则:加载错误 %s: %s\n", e.Source, e.Reason)
	}
	if len(rules) == 0 {
		return // 无旧规则(目录不存在/空/全解析失败)→ 无操作
	}

	stored := make([]storage.StoredRule, 0, len(rules))
	for _, r := range rules {
		s, err := ruleengine.RuleToStoredRule(r, "custom", "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "迁移旧规则:转换 %s 失败: %v\n", r.ID, err)
			continue
		}
		stored = append(stored, s)
	}

	if len(stored) > 0 {
		rep, migErr := storage.MigrateLegacyRules(db, storage.DomainDetect, stored)
		if migErr != nil {
			fmt.Fprintf(os.Stderr, "迁移旧规则到 db 失败: %v\n", migErr)
		} else if rep.Imported > 0 {
			fmt.Printf("已迁移 %d 条自定义规则到规则库\n", rep.Imported)
		}
		for _, e := range rep.Errors {
			fmt.Fprintf(os.Stderr, "迁移旧规则: %s\n", e)
		}
	}

	// 重命名旧 *.yaml/*.yml 为 .legacy(保留回滚,不删)。
	// 即使转换/写入有部分失败也重命名:坏文件不应每次启动重试解析;已成功迁入的规则在 db 里。
	// 用户若需回滚,把 .legacy 改回 .yaml 即可(下次首次建表时会重迁 —— 但通常只首次跑)。
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return // 目录已不存在(被外部删)或读取失败,无操作
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		p := filepath.Join(oldDir, e.Name())
		if err := os.Rename(p, p+".legacy"); err != nil {
			fmt.Fprintf(os.Stderr, "迁移旧规则:重命名 %s 失败: %v\n", p, err)
		}
	}
}
