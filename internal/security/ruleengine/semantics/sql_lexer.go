package semantics

import (
	"strings"
	"unicode"
)

// SQLToken 是 lexer 扫出的一个 token。
type SQLToken struct {
	Kind string // keyword / string / identifier / comment
	Text string
	Line int
}

// SQLScan 是 SQL 扫描结果。
type SQLScan struct {
	Tokens            []SQLToken
	DestructiveTokens []SQLToken // 只含破坏性 keyword(DROP/TRUNCATE/DELETE/ALTER...),已排除注释/字符串内
}

// destructiveKeywords 是破坏性 SQL keyword(对齐 snowflake 50 条规则覆盖的操作)。
// 复合 keyword(DROP TABLE)不在 lexer 层拼装;Task 11 在 RulesDetector 里按相邻 keyword token 判定。
var destructiveKeywords = map[string]bool{
	"DROP":     true,
	"TRUNCATE": true,
	"DELETE":   true,
	"ALTER":    true,
}

// lexerState 是 SQL lexer 状态机状态(对照 dcg snowflake.rs scan_sql / lex_statements)。
type lexerState int

const (
	stateNormal lexerState = iota
	stateLineComment  // -- ... \n (dcg 也在 \r 处终止)
	stateBlockComment // /* ... */ (dcg skip_block_comment 用 depth 计数支持嵌套)
	stateSingleQuoted // '...' (dcg skip_quoted 处理 \' 与 '' 转义)
	stateDoubleQuoted // "..." (dcg skip_quoted 处理 "" 转义)
	stateDollarQuoted // $$...$$ (Snowflake dollar-quoting)
)

// ScanSQL 逐字节扫 SQL,产出 token。状态机确保注释/字符串内的 keyword 不进 DestructiveTokens。
//
// 对照 dcg database/snowflake.rs:1240 scan_sql / 1508 lex_statements / 1659 skip_block_comment /
// 1630 skip_quoted。与 dcg 对齐的关键修正(超出 brief 原版):
//   - 块注释嵌套(dcg skip_block_comment depth 计数);brief 原版只看第一个 */。
//   - 单引号字符串支持 \' 反斜杠转义与 '' 双引号转义(dcg skip_quoted);brief 原版遇 ' 即结束。
//   - 双引号标识符支持 "" 转义(dcg skip_quoted);brief 原版遇 " 即结束。
//   - 行注释在 \r 处也终止(dcg lex_statements line 1521 匹配 \n | \r)。
//   - keyword Line 指向 keyword 起始行,而非 flush 时的当前行
//     (brief 原版 flush 闭包在 \n 后才 flush,会把上一行 keyword 记成下一行)。
func ScanSQL(payload string) *SQLScan {
	scan := &SQLScan{}
	state := stateNormal
	var tokenBuf strings.Builder
	line := 1
	// tokenStartLine 记录当前 token 起始行,避免 \n 后 flush 把 keyword 记到下一行。
	tokenStartLine := 1
	// blockDepth 是 stateBlockComment 的嵌套深度计数器(dcg skip_block_comment line 1659)。
	// 进入块注释时置 1,遇嵌套 /* 加 1,遇 */ 减 1,归零即退出。
	blockDepth := 0

	// flush 把 tokenBuf 里的 keyword 刷出。brief 原版用闭包捕获 line,
	// 但 \n 在 flush 之前已递增 line,导致跨行 keyword 记错行;改用 tokenStartLine。
	flush := func() {
		t := strings.ToUpper(strings.TrimSpace(tokenBuf.String()))
		tokenBuf.Reset()
		if t == "" {
			return
		}
		if destructiveKeywords[t] {
			tok := SQLToken{Kind: "keyword", Text: t, Line: tokenStartLine}
			scan.Tokens = append(scan.Tokens, tok)
			scan.DestructiveTokens = append(scan.DestructiveTokens, tok)
		}
	}

	bytes := []byte(payload)
	n := len(bytes)
	for i := 0; i < n; i++ {
		c := bytes[i]
		switch state {
		case stateNormal:
			switch {
			case c == '-' && i+1 < n && bytes[i+1] == '-':
				flush()
				state = stateLineComment
				i++ // 消费第二个 '-'
			case c == '/' && i+1 < n && bytes[i+1] == '*':
				flush()
				state = stateBlockComment
				blockDepth = 1
				i++ // 消费 '*'
			case c == '\'':
				flush()
				state = stateSingleQuoted
			case c == '"':
				flush()
				state = stateDoubleQuoted
			case c == '$' && i+1 < n && bytes[i+1] == '$':
				flush()
				state = stateDollarQuoted
				i++ // 消费第二个 '$'
			case isSQLWordChar(rune(c)):
				if tokenBuf.Len() == 0 {
					// token 起始:锁定起始行(当前 line,因为此字符在 token 起点)。
					tokenStartLine = line
				}
				tokenBuf.WriteByte(c)
			default:
				flush()
			}
		case stateLineComment:
			// dcg lex_statements line 1521:遇 \n 或 \r 即终止行注释。
			if c == '\n' || c == '\r' {
				state = stateNormal
			}
		case stateBlockComment:
			// dcg skip_block_comment line 1659:depth 计数支持嵌套 /* /* */ */。
			// brief 原版无嵌套,会在外层 */ 提前退出,把内层 DROP 当代码;此处对齐 dcg。
			if c == '/' && i+1 < n && bytes[i+1] == '*' {
				blockDepth++
				i++ // 消费 '*'
			} else if c == '*' && i+1 < n && bytes[i+1] == '/' {
				blockDepth--
				if blockDepth == 0 {
					state = stateNormal
				}
				i++ // 消费 '/'
			}
		case stateSingleQuoted:
			// dcg skip_quoted line 1634:'\' 反斜杠转义下一字符;'' 双引号转义为字面量单引号。
			if c == '\\' && i+1 < n {
				i++ // 跳过被转义字符
			} else if c == '\'' {
				if i+1 < n && bytes[i+1] == '\'' {
					i++ // '' 转义,字符串未结束
				} else {
					state = stateNormal
				}
			}
		case stateDoubleQuoted:
			// dcg skip_quoted line 1643:"" 双引号转义为字面量双引号。
			if c == '"' {
				if i+1 < n && bytes[i+1] == '"' {
					i++ // "" 转义,字符串未结束
				} else {
					state = stateNormal
				}
			}
		case stateDollarQuoted:
			// dcg lex_statements line 1539:遇 $$ 结束 dollar-quoted。
			if c == '$' && i+1 < n && bytes[i+1] == '$' {
				state = stateNormal
				i++ // 消费第二个 '$'
			}
		}
		// 换行计数(在状态处理之后,保证下一 token 起始行正确)。
		// tokenStartLine 在 tokenBuf.Len()==0 时锁定,故 \n 递增 line 后,
		// 下一个 word char 会用新的 line 值;此顺序正确。
		if c == '\n' {
			line++
		}
	}
	flush()
	return scan
}

// isSQLWordChar 判断 SQL word 字符(字母/数字/下划线)。
// dcg lex_statements line 1586 用 ascii_alphabetic + _;此处用 unicode.IsLetter
// 以支持非 ASCII 标识符(Snowflake 允许 Unicode 标识符),与 brief 一致。
func isSQLWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
