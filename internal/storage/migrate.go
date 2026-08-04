package storage

import (
	"fmt"
)

// MigrateRulesReport 是 MigrateLegacyRules 的返回:导入了多少条 custom 规则 + 逐条错误。
//
// 解耦设计(Task 10 final):本函数只做 db 写入(纯 storage 职责,不依赖 ruleengine)。
// 调用方(main.go 的 migrateLegacyRulesFiles)负责:
//  1. ruleengine.LoadDir(~/.code-agent-sentinel/rules, "global") 读旧文件;
//  2. ruleengine.RuleToStoredRule 把每条 Rule 转 StoredRule;
//  3. 调本函数把 StoredRule 写进 db 为 custom 行;
//  4. 把旧 *.yaml/*.yml 重命名 .legacy(保留回滚,不删)。
//
// 本函数只做第 3 步:把传入的 []StoredRule 作为 source=custom 行写入传入域 + 对侧域。
// 这样 storage 包保持纯净(不 import ruleengine),与 db.go/rules_repo.go/sync.go 一致。
type MigrateRulesReport struct {
	Imported int
	Errors   []string
}

// MigrateLegacyRules 把已转换好的 []StoredRule 作为 custom 行导入 db(两域)。
//
// 调用方(main.go)负责 LoadDir 旧文件 + RuleToStoredRule 转换 + 重命名 .legacy。
// 本函数只做 db 写入(纯 storage 职责,不依赖 ruleengine)。
//
// 语义:
//   - 每条 stored 以 source="custom" 写入传入 domain(UpsertRule:主键冲突走 DO UPDATE,
//     故重复调用同一批 rules 不产生重复行,等价 idempotent)。
//   - 同一条规则同时写对侧域(detect ↔ intercept):现状两侧本共用同一份规则定义,
//     静态检测与运行时拦截都需看到这些 custom 规则。
//   - builtin_version 传 ""(custom 行存 NULL,见 builtinVersionOrNull)。
//   - Imported 统计成功写入传入域的条数(对侧域写入失败不计入 Imported,只记 Errors,
//     因为传入域成功即视为该规则已迁移;对侧域失败不阻断整体,但旧文件仍被调用方重命名
//     .legacy(一次性迁移语义:坏文件不应每次启动重试解析),对侧域缺口不自愈)。
//
// 注意:本函数不重命名旧文件 —— 那是调用方的职责。调用方(main.go 的
// migrateLegacyRulesFiles)在调本函数后无条件重命名所有 *.yaml/*.yml 为 .legacy
// (即使部分转换/写入失败也重命名:避免每次启动重试坏文件;回滚需删 db 重建)。
// 故本函数名虽含 "Migrate",实际只做 "Import custom rows to db",重命名逻辑在 main.go。
func MigrateLegacyRules(db *DB, domain Domain, rules []StoredRule) (MigrateRulesReport, error) {
	var rep MigrateRulesReport
	// 对侧域:detect ↔ intercept 双向。两域规则定义现状共用,custom 规则同时落两侧。
	otherDomain := DomainIntercept
	if domain == DomainIntercept {
		otherDomain = DomainDetect
	}
	for _, stored := range rules {
		if err := UpsertRule(db, domain, "custom", stored, ""); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", stored.ID, err))
			continue
		}
		// 对侧域写入失败不阻断本条计入 Imported(传入域已成功),仅记错误。
		// 对侧域失败时旧文件由调用方保留(无条件重命名,见下),下次启动 schema 已初始化
		// 不会再迁——故对侧域缺口不自愈,但两域同 UpsertRule 同时失败概率极低。
		if err := UpsertRule(db, otherDomain, "custom", stored, ""); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s(%s-side): %v", stored.ID, otherDomain, err))
		}
		rep.Imported++
	}
	return rep, nil
}
