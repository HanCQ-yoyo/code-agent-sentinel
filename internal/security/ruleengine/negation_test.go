package ruleengine

import "testing"

func TestIsNegatedByContextLineStart(t *testing.T) {
	// 第 2 行行首有"禁止",命中在第 2 行 → 抑制
	content := "title\n禁止使用 rm -rf /\nother"
	locs := []Location{{Line: 2, StartCol: 5, EndCol: 11}}
	if !IsNegatedByContext(content, locs) {
		t.Error("行首禁止词应抑制")
	}
}

func TestIsNegatedByContextBeforeMatch(t *testing.T) {
	// 匹配点前 N 字符内有"do not"
	content := "reminder: do not run rm -rf / here\n"
	locs := []Location{{Line: 1, StartCol: 23, EndCol: 29}} // rm -rf 位置
	if !IsNegatedByContext(content, locs) {
		t.Error("匹配点前 do not 应抑制")
	}
}

func TestIsNegatedByContextNoNegation(t *testing.T) {
	content := "you should run rm -rf / to clean up\n"
	locs := []Location{{Line: 1, StartCol: 16, EndCol: 22}}
	if IsNegatedByContext(content, locs) {
		t.Error("无否定词不应抑制")
	}
}

func TestIsNegatedByContextEmptyLocs(t *testing.T) {
	// command 字段命中(locs 空)→ 不抑制(避免假阴性)
	if IsNegatedByContext("禁止 rm -rf", nil) {
		t.Error("空 locs(command 命中)不应抑制")
	}
}

func TestIsNegatedByContextLineOutOfRange(t *testing.T) {
	// 行号超出 content 行数 → 不抑制(防御性,不误删)
	content := "one line\n"
	locs := []Location{{Line: 5, StartCol: 1, EndCol: 3}}
	if IsNegatedByContext(content, locs) {
		t.Error("行号越界不应抑制")
	}
}

func TestIsNegatedByContextNegationAfterMatch(t *testing.T) {
	// 否定词在匹配点之后 → 不抑制(语义不翻转)
	content := "rm -rf / is forbidden here\n"
	locs := []Location{{Line: 1, StartCol: 1, EndCol: 7}}
	if IsNegatedByContext(content, locs) {
		t.Error("否定词在匹配点后不应抑制")
	}
}
