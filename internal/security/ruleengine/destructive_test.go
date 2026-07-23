package ruleengine

import (
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// 验证 destructive_commands.yaml 能被 LoadBuiltin 加载,且样例规则可求值。
// Task 3 的骨架测试:文件存在 + 样例规则 destructive.sample.should-exist 注册。
// Task 4 起追加真实 git 域规则后,样例规则被删除,本测试改为验证 git 域规则加载。
func TestDestructiveRules_Load(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) > 0 {
		t.Fatalf("load errors: %v", errs)
	}
	// git 域应至少有 12 条规则
	gitCount := 0
	for _, r := range rules {
		if d, ok := r.Metadata["domain"].(string); ok && d == "git" {
			gitCount++
		}
	}
	if gitCount < 12 {
		t.Errorf("expected ≥12 destructive.git.* rules, got %d", gitCount)
	}
}

// filterRulesByDomain 按 metadata.domain 过滤规则。
func filterRulesByDomain(rules []Rule, domain string) []Rule {
	var out []Rule
	for _, r := range rules {
		if d, ok := r.Metadata["domain"].(string); ok && d == domain {
			out = append(out, r)
		}
	}
	return out
}

// makeAssetWithField 合成 Asset:field=content 写 Content,其余写 Fields[field]。
func makeAssetWithField(field, cmd string) configengine.Asset {
	a := configengine.Asset{Type: configengine.AssetHook}
	if field == "content" {
		a.Content = cmd
	} else {
		a.Fields = map[string]any{field: cmd}
	}
	return a
}

// evalRuleMatch 调 ruleengine.Eval 返回是否命中。
func evalRuleMatch(r Rule, a configengine.Asset) (bool, string) {
	res := Eval(r, a)
	return res.Matched, res.Evidence
}

// evalRulesForCommand 合成 Asset 对 cmd 跑规则,返回首个命中规则 id。
func evalRulesForCommand(t *testing.T, rules []Rule, field, cmd string) string {
	t.Helper()
	asset := makeAssetWithField(field, cmd)
	for _, r := range rules {
		matched, _ := evalRuleMatch(r, asset)
		if matched {
			return r.ID
		}
	}
	return ""
}

// evalAllRulesForCommand 合成 Asset 对 cmd 跑规则,返回所有命中规则 id。
// 用于跨子域重叠 SQL(如 DROP DATABASE 同时命中 mysql 和 postgresql)的验证。
func evalAllRulesForCommand(t *testing.T, rules []Rule, field, cmd string) []string {
	t.Helper()
	asset := makeAssetWithField(field, cmd)
	var hits []string
	for _, r := range rules {
		matched, _ := evalRuleMatch(r, asset)
		if matched {
			hits = append(hits, r.ID)
		}
	}
	return hits
}

// wantAnyHit 检查 wantID 是否在 hits 中(用于跨子域重叠规则的测试)。
func wantAnyHit(t *testing.T, name, cmd string, hits []string, wantID string) {
	t.Helper()
	for _, h := range hits {
		if h == wantID {
			return
		}
	}
	t.Errorf("cmd=%q: got %v, want any hit to contain %q", cmd, hits, wantID)
}

// TestDestructive_GitDomain — Task 4:git 域 12 条 dest 规则 + safe post_exclude。
// 覆盖:5 条 dest 命中 + 3 条 safe 不误报(含 git commit -m "rm -rf /" 数据区隔)。
func TestDestructive_GitDomain(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) > 0 {
		t.Fatalf("LoadBuiltin errors: %v", errs)
	}
	gitRules := filterRulesByDomain(rules, "git")
	if len(gitRules) < 12 {
		t.Fatalf("expected ≥12 git rules, got %d", len(gitRules))
	}

	cases := []struct {
		name   string
		cmd    string
		field  string // command / content / allow
		wantID string // 期望命中的规则 id(空=不应命中)
	}{
		// dest 命中
		{"reset-hard", "git reset --hard origin/main", "command", "destructive.git.reset-hard"},
		{"checkout-discard", "git checkout -- file.txt", "command", "destructive.git.checkout-discard"},
		{"clean-force", "git clean -fd", "command", "destructive.git.clean-force"},
		{"branch-force-delete", "git branch -D feature", "command", "destructive.git.branch-force-delete"},
		{"push-force-short", "git push -f origin main", "command", "destructive.git.push-force-short"},
		{"push-force-long", "git push --force origin main", "command", "destructive.git.push-force-long"},
		{"stash-drop", "git stash drop", "command", "destructive.git.stash-drop"},
		{"stash-clear", "git stash clear", "command", "destructive.git.stash-clear"},
		// safe 不误报(对应 safe_pattern 应被 post_exclude 排除)
		{"checkout-new-branch-safe", "git checkout -b feature", "command", ""},
		{"checkout-orphan-safe", "git checkout --orphan newbranch", "command", ""},
		{"clean-dry-run-safe", "git clean -nfd", "command", ""},
		{"git-commit-msg-safe", "git commit -m \"rm -rf /\"", "command", ""},
		// push-force-long/short 的 (?![-a-z]) lookahead 使 --force-with-lease 不匹配
		{"push-force-with-lease-safe", "git push --force-with-lease origin main", "command", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hitID := evalRulesForCommand(t, gitRules, c.field, c.cmd)
			if hitID != c.wantID {
				t.Errorf("cmd=%q field=%s: got %q want %q", c.cmd, c.field, hitID, c.wantID)
			}
		})
	}
}

// TestDestructive_DatabaseDomain — Task 6:database 域 7 子域规则(112 dest + 57 safe→post_exclude)。
// 增量构建:每转写完一个子域就追加该子域的测试用例并提交。
// 规则名对齐 dcg database/<sub>.rs 的 pattern name。
//
// 注意跨子域重叠:SQL 关键字(DROP DATABASE/TABLE/TRUNCATE/DELETE FROM)同时
// 命中 mysql 和 postgresql 规则。使用 anyHit 验证期望规则在命中集中;safe 用例
// 期望空命中集。
func TestDestructive_DatabaseDomain(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) > 0 {
		t.Fatalf("LoadBuiltin errors: %v", errs)
	}
	dbRules := filterRulesByDomain(rules, "database")

	// hitCases:期望 wantID 至少在命中集中出现(可能同时命中其他子域的重叠规则)。
	hitCases := []struct {
		name   string
		cmd    string
		field  string
		wantID string
	}{
		// ===== mongodb(6 dest + 5 safe→post_exclude)=====
		{"mongo-drop-database", "db.dropDatabase()", "command", "destructive.database.mongodb.drop-database"},
		{"mongo-drop-collection", "db.users.drop()", "command", "destructive.database.mongodb.drop-collection"},
		{"mongo-delete-all", "db.users.deleteMany({})", "command", "destructive.database.mongodb.delete-all"},
		{"mongo-mongorestore-drop", "mongorestore --drop /backup", "command", "destructive.database.mongodb.mongorestore-drop"},
		// ===== mysql(11 dest + 5 safe)=====
		{"mysql-drop-database", "mysql -e 'DROP DATABASE prod'", "command", "destructive.database.mysql.drop-database"},
		{"mysql-drop-table", "mysql -e 'DROP TABLE users'", "command", "destructive.database.mysql.drop-table"},
		{"mysql-truncate-table", "mysql -e 'TRUNCATE TABLE users'", "command", "destructive.database.mysql.truncate-table"},
		{"mysql-delete-without-where", "mysql -e 'DELETE FROM users;'", "command", "destructive.database.mysql.delete-without-where"},
		{"mysql-mysqladmin-drop", "mysqladmin drop mydb", "command", "destructive.database.mysql.mysqladmin-drop"},
		{"mysql-mysqldump-add-drop-database", "mysqldump --add-drop-database mydb", "command", "destructive.database.mysql.mysqldump-add-drop-database"},
		{"mysql-mysqldump-add-drop-table", "mysqldump --add-drop-table mydb", "command", "destructive.database.mysql.mysqldump-add-drop-table"},
		{"mysql-grant-all", "mysql -e \"GRANT ALL ON *.* TO 'user'@'host'\"", "command", "destructive.database.mysql.grant-all"},
		{"mysql-drop-user", "mysql -e 'DROP USER admin'", "command", "destructive.database.mysql.drop-user"},
		{"mysql-reset-master", "mysql -e 'RESET MASTER'", "command", "destructive.database.mysql.reset-master"},
		// ===== postgresql(8 dest + 2 safe)=====
		{"pg-drop-database", "psql -c 'DROP DATABASE mydb'", "command", "destructive.database.postgresql.drop-database"},
		{"pg-drop-table", "psql -c 'DROP TABLE users'", "command", "destructive.database.postgresql.drop-table"},
		{"pg-drop-schema", "psql -c 'DROP SCHEMA public CASCADE'", "command", "destructive.database.postgresql.drop-schema"},
		{"pg-truncate-table", "psql -c 'TRUNCATE TABLE users'", "command", "destructive.database.postgresql.truncate-table"},
		{"pg-delete-without-where", "psql -c 'DELETE FROM users;'", "command", "destructive.database.postgresql.delete-without-where"},
		{"pg-dropdb-cli", "dropdb mydb", "command", "destructive.database.postgresql.dropdb-cli"},
		{"pg-dump-clean", "pg_dump --clean mydb", "command", "destructive.database.postgresql.pg-dump-clean"},
		{"pg-dump-clean-short", "pg_dump -c mydb", "command", "destructive.database.postgresql.pg-dump-clean"},
		// ===== redis(14 dest + 6 safe)=====
		{"redis-flushall", "redis-cli FLUSHALL", "command", "destructive.database.redis.flushall"},
		{"redis-flushdb", "redis-cli FLUSHDB", "command", "destructive.database.redis.flushdb"},
		{"redis-config-resetstat", "redis-cli CONFIG RESETSTAT", "command", "destructive.database.redis.config-resetstat"},
		{"redis-mass-delete", "redis-cli KEYS '*' | xargs redis-cli DEL", "command", "destructive.database.redis.mass-delete-pipeline"},
		{"redis-debug-crash", "redis-cli DEBUG SEGFAULT", "command", "destructive.database.redis.debug-crash"},
		{"redis-debug-sleep", "redis-cli DEBUG SLEEP 30", "command", "destructive.database.redis.debug-sleep"},
		{"redis-shutdown", "redis-cli SHUTDOWN", "command", "destructive.database.redis.shutdown"},
		{"redis-config-dangerous", "redis-cli CONFIG SET dir /tmp", "command", "destructive.database.redis.config-dangerous"},
		{"redis-config-set-maxmemory", "redis-cli CONFIG SET maxmemory 100mb", "command", "destructive.database.redis.config-set-maxmemory"},
		{"redis-config-set-maxmemory-policy", "redis-cli CONFIG SET maxmemory-policy allkeys-lru", "command", "destructive.database.redis.config-set-maxmemory-policy"},
		{"redis-config-set-save", "redis-cli CONFIG SET save ''", "command", "destructive.database.redis.config-set-save"},
		{"redis-config-set-appendonly", "redis-cli CONFIG SET appendonly no", "command", "destructive.database.redis.config-set-appendonly"},
		{"redis-config-rewrite", "redis-cli CONFIG REWRITE", "command", "destructive.database.redis.config-rewrite"},
		// ===== sqlite(5 dest + 4 safe)=====
		{"sqlite-drop-table", "sqlite3 mydb 'DROP TABLE users'", "command", "destructive.database.sqlite.drop-table"},
		{"sqlite-delete-without-where", "sqlite3 mydb 'DELETE FROM users;'", "command", "destructive.database.sqlite.delete-without-where"},
		{"sqlite-vacuum-into", "sqlite3 mydb 'VACUUM INTO \"backup.db\"'", "command", "destructive.database.sqlite.vacuum-into"},
		{"sqlite3-stdin", "sqlite3 mydb < init.sql", "command", "destructive.database.sqlite.sqlite3-stdin"},
	}
	for _, c := range hitCases {
		t.Run(c.name, func(t *testing.T) {
			hits := evalAllRulesForCommand(t, dbRules, c.field, c.cmd)
			wantAnyHit(t, c.name, c.cmd, hits, c.wantID)
		})
	}

	// safeCases:期望不命中任何 database 规则。
	safeCases := []struct {
		name  string
		cmd   string
		field string
	}{
		// mongodb safe
		{"mongo-find-safe", "db.users.find({status: 'active'})", "command"},
		{"mongo-count-safe", "db.users.countDocuments({})", "command"},
		{"mongo-aggregate-safe", "db.users.aggregate([{$match: {x: 1}}])", "command"},
		{"mongo-explain-safe", "db.users.find({}).explain()", "command"},
		{"mongo-mongodump-no-drop-safe", "mongodump --out=/backup", "command"},
		// mysql safe
		{"mysql-select-safe", "mysql -e 'SELECT 1'", "command"},
		{"mysql-show-safe", "mysql -e 'SHOW DATABASES'", "command"},
		{"mysql-describe-safe", "mysql -e 'DESCRIBE users'", "command"},
		{"mysql-mysqldump-no-drop-safe", "mysqldump mydb > backup.sql", "command"},
		{"mysql-delete-with-where-safe", "mysql -e 'DELETE FROM users WHERE id = 1'", "command"},
		// postgresql safe
		{"pg-select-safe", "psql -c 'SELECT * FROM users'", "command"},
		{"pg-dump-no-clean-safe", "pg_dump mydb > backup.sql", "command"},
		{"pg-delete-with-where-safe", "psql -c 'DELETE FROM users WHERE id = 1'", "command"},
		// redis safe
		{"redis-get-safe", "redis-cli GET foo", "command"},
		{"redis-scan-safe", "redis-cli -n 2 SCAN 0", "command"},
		{"redis-info-safe", "redis-cli INFO", "command"},
		{"redis-keys-safe", "redis-cli KEYS '*'", "command"},
		{"redis-dbsize-safe", "redis-cli DBSIZE", "command"},
		{"redis-config-get-safe", "redis-cli CONFIG GET maxmemory", "command"},
		// sqlite safe
		{"sqlite-select-safe", "sqlite3 mydb 'SELECT * FROM users'", "command"},
		{"sqlite-explain-safe", "sqlite3 mydb 'EXPLAIN SELECT * FROM users'", "command"},
		{"sqlite-dot-command-safe", "sqlite3 mydb '.schema users'", "command"},
		{"sqlite-delete-with-where-safe", "sqlite3 mydb 'DELETE FROM users WHERE id = 1'", "command"},
	}
	for _, c := range safeCases {
		t.Run(c.name, func(t *testing.T) {
			hits := evalAllRulesForCommand(t, dbRules, c.field, c.cmd)
			if len(hits) > 0 {
				t.Errorf("cmd=%q field=%s: expected no hits, got %v", c.cmd, c.field, hits)
			}
		})
	}
}

// TestDestructive_FilesystemDomain — Task 5:filesystem 域 26 条 dest 规则 + safe→post_exclude。
// 覆盖:rm-rf /、rm -rf ~、find / -delete、unlink /etc/passwd 等 dest 命中;
// safe 不误报:rm -i file(无 -rf 标志不匹配)、rm /tmp/foo(post_exclude 排除 tmp 路径)。
//
// 规则名对齐 dcg core/filesystem.rs 的 pattern name(如 rm-rf-root-home 而非 rm-root-absolute)。
func TestDestructive_FilesystemDomain(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) > 0 {
		t.Fatalf("LoadBuiltin errors: %v", errs)
	}
	fsRules := filterRulesByDomain(rules, "filesystem")
	if len(fsRules) < 26 {
		t.Fatalf("expected ≥26 filesystem rules, got %d", len(fsRules))
	}

	cases := []struct {
		name   string
		cmd    string
		field  string
		wantID string
	}{
		// dest 命中(Critical:root/home/system 目标)
		{"rm-rf-root", "rm -rf /", "command", "destructive.filesystem.rm-rf-root-home"},
		{"rm-rf-home", "rm -rf ~", "command", "destructive.filesystem.rm-rf-root-home"},
		{"rm-r-f-separate-root", "rm -r -f /", "command", "destructive.filesystem.rm-r-f-separate-root-home"},
		{"rm-recursive-force-root", "rm --recursive --force /", "command", "destructive.filesystem.rm-recursive-force-root-home"},
		{"find-delete-root", "find / -delete", "command", "destructive.filesystem.find-delete-root-home"},
		{"find-delete-etc", "find /etc -delete", "command", "destructive.filesystem.find-delete-root-home"},
		{"unlink-etc", "unlink /etc/passwd", "command", "destructive.filesystem.unlink-root-home"},
		{"truncate-zero-etc", "truncate -s 0 /etc/passwd", "command", "destructive.filesystem.truncate-zero-root-home"},
		{"shred-etc", "shred /etc/passwd", "command", "destructive.filesystem.shred-root-home"},
		{"tar-remove-files-etc", "tar --remove-files -cf out.tar /etc", "command", "destructive.filesystem.tar-remove-files-root-home"},
		{"dd-of-etc", "dd if=/dev/zero of=/etc/passwd", "command", "destructive.filesystem.dd-overwrite-root-home"},
		{"mv-etc", "mv /etc /tmp/x", "command", "destructive.filesystem.mv-sensitive-source-root-home"},
		{"redirect-truncate-etc", "echo data > /etc/passwd", "command", "destructive.filesystem.redirect-truncate-root-home"},
		// dest 命中(High:非 root/home 目标)
		{"rm-rf-general", "rm -rf ./build", "command", "destructive.filesystem.rm-rf-general"},
		{"find-delete-general", "find . -delete", "command", "destructive.filesystem.find-delete-general"},
		{"unlink-general", "unlink ./scratch", "command", "destructive.filesystem.unlink-general"},
		{"truncate-zero-general", "truncate -s 0 ./file", "command", "destructive.filesystem.truncate-zero-general"},
		{"shred-general", "shred ./file", "command", "destructive.filesystem.shred-general"},
		{"tar-remove-files-general", "tar --remove-files -cf out.tar ./src", "command", "destructive.filesystem.tar-remove-files-general"},
		{"dd-overwrite-general", "dd of=./file", "command", "destructive.filesystem.dd-overwrite-general"},
		{"mv-dynamic-path", "mv $VAR /tmp/x", "command", "destructive.filesystem.mv-dynamic-path"},
		// 传播链(Critical:跨段敏感路径传播后强制删除)
		{"cp-sensitive-then-delete", "cp -a /etc /tmp/x && rm -rf /tmp/x", "command", "destructive.filesystem.cp-sensitive-then-delete"},
		{"ln-symlink-sensitive-then-delete", "ln -s /etc /tmp/x && rm -rf /tmp/x", "command", "destructive.filesystem.ln-symlink-sensitive-then-delete"},
		{"rsync-sensitive-then-delete", "rsync -a /etc/ /tmp/dest/ && rm -rf /tmp/dest", "command", "destructive.filesystem.rsync-sensitive-then-delete"},
		// safe 不误报(post_exclude 排除 tmp 路径 / 非破坏形态)
		{"rm-rf-tmp-safe", "rm -rf /tmp/foo", "command", ""},                 // post_exclude: rm-rf-tmp
		{"rm-recursive-force-tmp-safe", "rm --recursive --force /tmp/foo", "command", ""}, // post_exclude: rm-recursive-force-tmp
		{"find-delete-tmp-safe", "find /tmp/foo -delete", "command", ""},     // post_exclude: find-delete-tmp
		{"unlink-tmp-safe", "unlink /tmp/scratch", "command", ""},            // post_exclude: unlink-tmp
		{"rm-interactive-safe", "rm -i file", "command", ""},                 // -i 无 -rf,不匹配
		{"chmod-normal", "chmod 644 file", "command", ""},                    // 无 chmod 规则(filesystem 域)
		{"rm-help-safe", "rm --help", "command", ""},                         // 无 -rf,不匹配
		// 回归守卫(Important #1):`..` 路径遍历不得被 tmp post_exclude 抑制。
		// dcg safe_pattern 含 `(?!\.\.(?:/|\s|$)|[^\s]*/\.\.(?:/|\s|$))` 防护;
		// sentinel post_exclude 加 `(?!.*\.\.(?:/|\s|$))` lookahead 忠实对齐。
		// 未修复前 `rm -rf /tmp/foo/../etc` 被错误抑制(hitCtx 以 /tmp/ 开头 → post_exclude 命中 → 漏报)。
		// rm 规则 hitCtx 贪婪消费完整路径(/tmp/foo/../etc 以 / 起始满足 root-home)→ 命中 root-home;
		// 非 rm 规则(find/unlink/...)root-home 要求敏感目录紧接 / 起始(/etc 等),/tmp/foo/../etc 不满足,
		// 故命中 general 规则。关键:无论 root-home 还是 general,均不得被 post_exclude 抑制(危险方向漏报)。
		{"rm-rf-tmp-traversal-etc", "rm -rf /tmp/foo/../etc", "command", "destructive.filesystem.rm-rf-root-home"},
		{"rm-rf-vartmp-traversal-etc", "rm -rf /var/tmp/foo/../etc", "command", "destructive.filesystem.rm-rf-root-home"},
		{"rm-recursive-force-tmp-traversal", "rm --recursive --force /tmp/foo/../etc", "command", "destructive.filesystem.rm-recursive-force-root-home"},
		{"find-delete-tmp-traversal", "find /tmp/foo/../etc -delete", "command", "destructive.filesystem.find-delete-general"},
		{"unlink-tmp-traversal", "unlink /tmp/foo/../etc/passwd", "command", "destructive.filesystem.unlink-general"},
		{"shred-tmp-traversal", "shred /tmp/foo/../etc/passwd", "command", "destructive.filesystem.shred-general"},
		{"truncate-tmp-traversal", "truncate -s 0 /tmp/foo/../etc/passwd", "command", "destructive.filesystem.truncate-zero-general"},
		{"tar-remove-files-tmp-traversal", "tar --remove-files -cf out.tar /tmp/foo/../etc", "command", "destructive.filesystem.tar-remove-files-general"},
		{"dd-tmp-traversal", "dd if=/dev/zero of=/tmp/foo/../etc/passwd", "command", "destructive.filesystem.dd-overwrite-general"},
		// 回归守卫(Important #2):$TMPDIR 由调用方控制,可能解析为 /etc 或 /。
		// dcg safe_pattern 刻意不含 $TMPDIR;sentinel post_exclude 原误加 → 已移除。
		// 未修复前 `rm -rf $TMPDIR/foo` 被错误抑制(危险方向漏报)。
		{"rm-rf-tmpdir-var", "rm -rf $TMPDIR/foo", "command", "destructive.filesystem.rm-rf-general"},
		{"rm-rf-tmpdir-brace", "rm -rf ${TMPDIR}/foo", "command", "destructive.filesystem.rm-rf-general"},
		{"rm-recursive-force-tmpdir", "rm --recursive --force $TMPDIR/foo", "command", "destructive.filesystem.rm-recursive-force-long"},
		// redirect-truncate-dynamic-path(Important #3):5 分支 regex,此前无测试覆盖。
		// dcg 源测试用例(见 references/.../filesystem.rs:5049):变量路径、命令替换、
		// 通配符路径、^ 转义路径、%! Windows 变量。命中 destructive.filesystem.redirect-truncate-dynamic-path。
		{"redirect-truncate-dynamic-var", "echo data > $LOG_FILE", "command", "destructive.filesystem.redirect-truncate-dynamic-path"},
		{"redirect-truncate-dynamic-backtick", "echo data 2> `dynamic-path`", "command", "destructive.filesystem.redirect-truncate-dynamic-path"},
		{"redirect-truncate-dynamic-tilde", ": > ~root/.ssh/authorized_keys", "command", "destructive.filesystem.redirect-truncate-dynamic-path"},
		{"redirect-truncate-dynamic-wildcard", ": > /et?/passwd", "command", "destructive.filesystem.redirect-truncate-dynamic-path"},
		{"redirect-truncate-dynamic-caret", "echo x >^/etc/passwd", "command", "destructive.filesystem.redirect-truncate-dynamic-path"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hitID := evalRulesForCommand(t, fsRules, c.field, c.cmd)
			if hitID != c.wantID {
				t.Errorf("cmd=%q field=%s: got %q want %q", c.cmd, c.field, hitID, c.wantID)
			}
		})
	}
}
