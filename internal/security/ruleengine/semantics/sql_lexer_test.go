package semantics

import "testing"

// TestScanSQL_DropTable 验证基础破坏性 keyword 命中。
func TestScanSQL_DropTable(t *testing.T) {
	s := ScanSQL("DROP TABLE users")
	if len(s.DestructiveTokens) == 0 {
		t.Fatal("expected destructive token for DROP TABLE")
	}
	if s.DestructiveTokens[0].Kind != "keyword" {
		t.Errorf("kind: got %q want keyword", s.DestructiveTokens[0].Kind)
	}
	if s.DestructiveTokens[0].Text != "DROP" {
		t.Errorf("text: got %q want DROP", s.DestructiveTokens[0].Text)
	}
}

// TestScanSQL_DropInLineCommentNotDestructive 验证 -- 注释里的 DROP 不算执行。
func TestScanSQL_DropInLineCommentNotDestructive(t *testing.T) {
	s := ScanSQL("-- DROP TABLE users\nSELECT 1")
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "DROP" {
			t.Error("DROP in line comment should not be destructive token")
		}
	}
}

// TestScanSQL_DropInSingleQuotedNotDestructive 验证单引号字符串里的 DROP 不算。
func TestScanSQL_DropInSingleQuotedNotDestructive(t *testing.T) {
	s := ScanSQL(`SELECT 'DROP TABLE users'`)
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "DROP" {
			t.Error("DROP in single-quoted string should not be destructive token")
		}
	}
}

// TestScanSQL_DropInDollarQuotedNotDestructive 验证 Snowflake $$ dollar-quoting 内的 DROP 不算。
func TestScanSQL_DropInDollarQuotedNotDestructive(t *testing.T) {
	s := ScanSQL(`SELECT $$DROP TABLE x$$`)
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "DROP" {
			t.Error("DROP in $$ dollar-quoted string should not be destructive token")
		}
	}
}

// TestScanSQL_Truncate 验证 TRUNCATE 命中。
func TestScanSQL_Truncate(t *testing.T) {
	s := ScanSQL("TRUNCATE TABLE log")
	found := false
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "TRUNCATE" {
			found = true
		}
	}
	if !found {
		t.Error("expected TRUNCATE destructive token")
	}
}

// --- 边界用例(对照 dcg snowflake.rs scan_sql 行为补齐)---

// TestScanSQL_DropInBlockCommentNotDestructive 验证 /* 块注释 */ 内的 DROP 不算。
func TestScanSQL_DropInBlockCommentNotDestructive(t *testing.T) {
	s := ScanSQL("/* DROP TABLE x */ SELECT 1")
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "DROP" {
			t.Error("DROP in block comment should not be destructive token")
		}
	}
}

// TestScanSQL_DropInDoubleQuotedNotDestructive 验证双引号标识符内的 DROP 不算
// (SQL 双引号是标识符引用,不是字符串字面量,但 dcg 仍将其作为 quoted 区跳过)。
func TestScanSQL_DropInDoubleQuotedNotDestructive(t *testing.T) {
	s := ScanSQL(`SELECT "DROP TABLE x"`)
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "DROP" {
			t.Error("DROP in double-quoted identifier should not be destructive token")
		}
	}
}

// TestScanSQL_NestedBlockComment 验证嵌套 /* /* */ */ 块注释正确处理(dcg skip_block_comment 用 depth 计数)。
// 外层 */ 不应结束内层注释;只有 depth 归零才退出。
func TestScanSQL_NestedBlockComment(t *testing.T) {
	// 嵌套块注释里的 DROP 不应命中;外层退出后 SELECT 应正常。
	s := ScanSQL("/* outer /* inner DROP TABLE */ still comment */ SELECT 1")
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "DROP" {
			t.Error("DROP in nested block comment should not be destructive token")
		}
	}
}

// TestScanSQL_SingleQuotedBackslashEscape 验证 \' 转义不提前结束单引号字符串
// (dcg skip_quoted 处理 backslash escape)。
func TestScanSQL_SingleQuotedBackslashEscape(t *testing.T) {
	// 'it\'s DROP TABLE' — 字符串内的 DROP 不算,且 \' 不结束字符串。
	s := ScanSQL(`SELECT 'it\'s DROP TABLE'`)
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "DROP" {
			t.Error("DROP in backslash-escaped single-quoted string should not be destructive")
		}
	}
}

// TestScanSQL_SingleQuotedDoubledQuote 验证 '' 转义不提前结束单引号字符串
// (dcg skip_quoted 处理 doubled quote)。
func TestScanSQL_SingleQuotedDoubledQuote(t *testing.T) {
	// 'DROP TABLE ''s''' — '' 是字面量单引号,字符串未结束,DROP 不算。
	s := ScanSQL(`SELECT 'DROP TABLE ''s'''`)
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "DROP" {
			t.Error("DROP in doubled-quote single-quoted string should not be destructive")
		}
	}
}

// TestScanSQL_MultipleDestructiveKeywords 验证一条 SQL 里多个破坏性 keyword 都命中。
func TestScanSQL_MultipleDestructiveKeywords(t *testing.T) {
	s := ScanSQL("DROP TABLE a; DELETE FROM b; ALTER TABLE c")
	texts := map[string]bool{}
	for _, tk := range s.DestructiveTokens {
		texts[tk.Text] = true
	}
	for _, want := range []string{"DROP", "DELETE", "ALTER"} {
		if !texts[want] {
			t.Errorf("expected destructive keyword %s, got %v", want, texts)
		}
	}
}

// TestScanSQL_LineTracking 验证 keyword 的 Line 字段指向 keyword 所在行
// (brief 的 flush 闭包在 \n 后才 flush,会把上一行 keyword 的 line 记成下一行;此处校验修正)。
func TestScanSQL_LineTracking(t *testing.T) {
	// DROP 在第 1 行,SELECT 在第 2 行。
	s := ScanSQL("DROP TABLE x\nSELECT 1")
	if len(s.DestructiveTokens) == 0 {
		t.Fatal("expected destructive token")
	}
	if s.DestructiveTokens[0].Line != 1 {
		t.Errorf("DROP line: got %d want 1", s.DestructiveTokens[0].Line)
	}
}

// TestScanSQL_LineTrackingMultiLine 验证多行后 keyword 的 Line 正确。
func TestScanSQL_LineTrackingMultiLine(t *testing.T) {
	// 第 3 行是 TRUNCATE。
	s := ScanSQL("-- comment\nSELECT 1\nTRUNCATE TABLE log")
	found := false
	for _, tk := range s.DestructiveTokens {
		if tk.Text == "TRUNCATE" {
			found = true
			if tk.Line != 3 {
				t.Errorf("TRUNCATE line: got %d want 3", tk.Line)
			}
		}
	}
	if !found {
		t.Error("expected TRUNCATE destructive token")
	}
}

// TestScanSQL_CaseInsensitive 验证 keyword 大小写不敏感(drop/drop/DrOp 都命中)。
func TestScanSQL_CaseInsensitive(t *testing.T) {
	for _, input := range []string{"drop table x", "Drop table x", "DROP table x"} {
		s := ScanSQL(input)
		if len(s.DestructiveTokens) == 0 || s.DestructiveTokens[0].Text != "DROP" {
			t.Errorf("case-insensitive DROP: input %q got %v", input, s.DestructiveTokens)
		}
	}
}

// TestScanSQL_EmptyAndNoDestructive 验证空输入和无破坏性 keyword 返回空列表。
func TestScanSQL_EmptyAndNoDestructive(t *testing.T) {
	for _, input := range []string{"", "SELECT 1", "INSERT INTO t VALUES (1)", "UPDATE t SET x=1"} {
		s := ScanSQL(input)
		if len(s.DestructiveTokens) != 0 {
			t.Errorf("input %q: expected 0 destructive tokens, got %d (%v)",
				input, len(s.DestructiveTokens), s.DestructiveTokens)
		}
	}
}
