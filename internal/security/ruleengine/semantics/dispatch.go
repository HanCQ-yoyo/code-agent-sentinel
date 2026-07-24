package semantics

import (
	"regexp"
	"strings"
)

// Dispatch 按规则域(domain)分发到对应的语义解析器(单域路由)。
//
// domain 来自 rule.Metadata["domain"](destructive.<domain>.<pattern> 规则的 domain 段)。
// 已实现的域:
//   - git        → GitSemanticDecision(argv 解析 git 子命令 + flags)
//   - filesystem → RmSemanticDecision(argv 解析 rm flags)
//   - database   → snowflakeSemanticDecision(提取 snow sql --query SQL,跑 ScanSQL)
//
// 未实现的域(containers/package_managers 等)返回 Unknown,交回正则层处理。
// Unknown 也用于已实现域中解析器无法判定的命令(如 git config 子命令)。
//
// 注意:本函数是单域路由,不处理跨域冲突。RulesDetector 实际调用 DispatchCommand
// (优先级分发),避免 `git commit -m "rm -rf /"` 被 filesystem 解析器误判 Deny
// (rm 在 -m 数据区是字面量,但 filesystem 解析器不识别 git 数据区边界)。
// 本函数保留供单域单元测试与未来按域精准路由的场景使用。
func Dispatch(domain string, command string) SemanticResult {
	switch domain {
	case "git":
		return GitSemanticDecision(command)
	case "filesystem":
		return RmSemanticDecision(command)
	case "database":
		return snowflakeSemanticDecision(command)
	}
	return SemanticResult{Decision: Unknown}
}

// DispatchCommand 对命令文本做优先级语义分发,返回首个非 Unknown 的结果。
//
// 优先级:git > filesystem > database。git 优先级最高,因为 git 子命令有明确
// 结构(subcommand + args),且 git commit -m / git tag -m 会把后续文本标记为
// 数据区(字面量不执行)——这能正确抑制 filesystem 解析器对数据区内 rm 字面量的误判。
//
// 解决的跨域冲突:`git commit -m "rm -rf /"`
//   - git 解析器:commit -m → Safe(数据区字面量)
//   - filesystem 解析器:看到 rm -rf / → Deny(误判,不识别 git 数据区边界)
//   - 优先级分发:git Safe 先返回,filesystem 不再运行 → 正确抑制误报
//
// RulesDetector 在两道关卡调用本函数(见 rules_detector.go Detect 循环):
//   - 关卡 1(正则前):Deny → 直接构造 finding(不经正则,防漏报);Safe → 跳过该规则(防误报)
//   - 关卡 2(正则后):Safe → 丢弃命中(复核正则命中是否误报)
func DispatchCommand(command string) SemanticResult {
	if r := GitSemanticDecision(command); r.Decision != Unknown {
		return r
	}
	if r := RmSemanticDecision(command); r.Decision != Unknown {
		return r
	}
	if r := snowflakeSemanticDecision(command); r.Decision != Unknown {
		return r
	}
	return SemanticResult{Decision: Unknown}
}

// HasParser 返回该域是否已接入语义解析器(git/filesystem/database→true,其余→false)。
// RulesDetector 用它在调用 DispatchCommand 前短路,避免对无解析器域的规则白跑一趟。
// (DispatchCommand 内部会跑全部 3 个解析器,与域无关;HasParser 仅用于 RulesDetector
// 判断「该规则域是否有语义介入可能」,若 false 则两道关卡整体跳过。)
func HasParser(domain string) bool {
	switch domain {
	case "git", "filesystem", "database":
		return true
	}
	return false
}

// snowQueryRe 提取 snow sql --query '...' 的 SQL。
// 兼容单/双引号(--query 'SQL' / --query "SQL")。
// 不处理 --query=SQL(无引号)形式 —— 该形式 SQL 无破坏性 keyword 时会返回 Unknown,
// 交回正则层兜底(正则规则同样覆盖 snow sql --query 场景)。
var snowQueryRe = regexp.MustCompile(`(?i)--query\s+['"]([^'"]*)['"]`)

// snowflakeSemanticDecision 对 snow sql 命令做语义判断:
//   - 命令不含 "snow sql" → Unknown(其他 database CLI 如 mysql/psql 不归本解析器,交回正则)
//   - 提取 --query 'SQL' 的 SQL,跑 ScanSQL
//   - SQL 含破坏性 keyword(DROP/TRUNCATE/...)→ Deny
//   - SQL 无破坏性 keyword → Safe(抑制正则对 --query 内 SQL 的误报)
//   - 无 --query 参数 → Unknown(snow sql 可能是交互式会话,交回正则)
//
// 对照 dcg database/snowflake.rs:本函数是 CLI --query 内联 SQL 的语义层入口;
// snow sql 交互式 stdin 输入的 SQL 由 destructive.snowflake.stdin-unverified 正则规则覆盖
// (语义层不处理 stdin,因 stdin 内容不在命令文本里)。
func snowflakeSemanticDecision(command string) SemanticResult {
	if !strings.Contains(strings.ToLower(command), "snow sql") {
		return SemanticResult{Decision: Unknown}
	}
	m := snowQueryRe.FindStringSubmatch(command)
	if m == nil {
		return SemanticResult{Decision: Unknown}
	}
	scan := ScanSQL(m[1])
	if len(scan.DestructiveTokens) > 0 {
		var kws []string
		for _, tk := range scan.DestructiveTokens {
			kws = append(kws, tk.Text)
		}
		return SemanticResult{
			Decision: Deny,
			RuleID:   snowflakeRuleIDForSQL(m[1]),
			Reason:   "SQL 破坏性 keyword: " + strings.Join(kws, ","),
		}
	}
	return SemanticResult{Decision: Safe, Reason: "SQL 无破坏性 keyword"}
}

// snowRuleTargetRe 捕获破坏性 SQL 构造的 leading keyword + 紧邻目标 keyword,
// 用于映射到具体 dcg_rule_id(修复最终 review C2)。
// ScanSQL 的 lexer 只保留破坏性 keyword token(DROP/TRUNCATE/...),不保留 DATABASE/
// TABLE/SCHEMA 等目标 keyword,故无法从 DestructiveTokens 序列判定 DROP 的对象类型。
// 这里在原始 SQL 文本上做正则提取(绕过 lexer 的 token 保留限制),已排除注释/字符串内
// 的误命中由 ScanSQL 的 DestructiveTokens 把关(调用者仅在 DestructiveTokens 非空时进入本函数)。
var snowRuleTargetRe = regexp.MustCompile(`(?i)\b(DROP|TRUNCATE|DELETE|UPDATE)\s+(DATABASE|SCHEMA|TABLE|FROM|SET)\b`)

// snowflakeRuleIDForSQL 把 SQL 文本里的破坏性构造映射到具体 dcg_rule_id,
// 对齐 destructive_commands.yaml 的 snowflake.* 条目(修复最终 review C2)。
//
// 修前:snowflakeSemanticDecision 统一返回 RuleID="snowflake.drop"(通用语义 RuleID,
// 无对应 dcg_rule_id)→ pickSemanticCarrier strategy 1 miss → 回退 strategy 2 = 该域首条
// 规则(按文件序 destructive.database.mongodb.stdin-unverified,severity high)→
// (1) semantic finding severity 被扭曲成 high(DROP DATABASE 本应 critical);
// (2) Gate 1 continue 抑制了正确的 critical 正则规则(snowflake.drop-database 等)。
//
// 修后:按破坏性构造返回具体 dcg_rule_id(如 DROP DATABASE → snowflake.drop-database),
// pickSemanticCarrier strategy 1 精确命中对应 YAML 规则,继承正确 severity/remediation。
//
// 映射规则(对齐 YAML 的 dcg_rule_id):
//   - DROP DATABASE → snowflake.drop-database   (critical)
//   - DROP SCHEMA   → snowflake.drop-schema      (critical)
//   - DROP TABLE    → snowflake.drop-table       (critical)
//   - TRUNCATE      → snowflake.truncate-table   (critical)
//   - DELETE FROM   → snowflake.delete-all       (critical)
//   - UPDATE ... SET → snowflake.update-all     (critical)
//   - 其他(DROP 其他目标 / 无法细分)→ snowflake.drop(回退,保持旧行为)
//
// WHERE 子句区分(bounded-delete/bounded-update)由正则规则承担,语义层只负责给 Deny
// 配正确 carrier,故任何 DELETE/UPDATE keyword(无论有无 WHERE)都映射到 delete-all/update-all。
func snowflakeRuleIDForSQL(sql string) string {
	m := snowRuleTargetRe.FindStringSubmatch(sql)
	if m == nil {
		return "snowflake.drop"
	}
	lead := strings.ToUpper(m[1])
	target := strings.ToUpper(m[2])
	switch lead {
	case "DROP":
		switch target {
		case "DATABASE":
			return "snowflake.drop-database"
		case "SCHEMA":
			return "snowflake.drop-schema"
		case "TABLE":
			return "snowflake.drop-table"
		}
	case "TRUNCATE":
		return "snowflake.truncate-table"
	case "DELETE":
		return "snowflake.delete-all"
	case "UPDATE":
		return "snowflake.update-all"
	}
	return "snowflake.drop"
}
