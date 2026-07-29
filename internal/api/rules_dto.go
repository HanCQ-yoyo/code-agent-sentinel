package api

import (
	"code-agent-sentinel/internal/security/ruleengine"
)

// ruleDTO 是规则 API 的响应/请求体(对应 storage.StoredRule + 派生字段)。
// match/paths/deobfuscation/metadata 用 map/结构(经 ruleengine.RuleRow 往返)。
// Task 11:检测/拦截两域共用同一 DTO,Domain 字段标识来源域。
type ruleDTO struct {
	ID            string                 `json:"id"`
	Severity      string                 `json:"severity"`
	AssetType     string                 `json:"asset_type"`
	Match         map[string]any         `json:"match"`
	Deobfuscation []string               `json:"deobfuscation"`
	Dotall        bool                   `json:"dotall"`
	Paths         *ruleengine.PathFilter `json:"paths"`
	PostExclude   []string               `json:"post_exclude"`
	Remediation   string                 `json:"remediation"`
	Description   string                 `json:"description"`
	Metadata      map[string]any         `json:"metadata"`
	Source        string                 `json:"source"`   // builtin | custom
	Enabled       bool                   `json:"enabled"`  // 经 overrides 表 JOIN 派生(默认 true)
	CanEdit       bool                   `json:"can_edit"` // source==custom 才可编辑/删除
	Domain        string                 `json:"domain"`   // detect | intercept
}

// forkRuleBody 是 fork 端点的请求体(只需 new_id)。
type forkRuleBody struct {
	NewID string `json:"new_id"`
}

// enabledBody 是启停端点的请求体。
type enabledBody struct {
	Enabled bool `json:"enabled"`
}
