package editor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"

	"code-agent-sentinel/internal/configengine"
)

// computeDiff 返回 old→new 的 unified-style diff 文本。无变更返 ""。
// 用 diffmatchpatch 做行级 diff(checklines+字符模式可能把 line2→lineX 拆成
// 字符级 "2"/"X" 片段,导致整行文本不出现在输出里),因此先用 DiffLinesToChars
// 把每行编码成单字符,DiffMain 后再用 DiffCharsToLines 还原,保证输出的是整行。
func computeDiff(old, new string) string {
	if old == new {
		return ""
	}
	dmp := diffmatchpatch.New()
	t1, t2, lineArray := dmp.DiffLinesToChars(old, new)
	diffs := dmp.DiffMain(t1, t2, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)
	var b strings.Builder
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			for _, line := range splitLines(d.Text) {
				b.WriteString("+ " + line + "\n")
			}
		case diffmatchpatch.DiffDelete:
			for _, line := range splitLines(d.Text) {
				b.WriteString("- " + line + "\n")
			}
		}
	}
	return b.String()
}

// splitLines 把文本按 \n 切成行,去掉末尾空行。
func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// secretRe 匹配常见密钥前缀:GitHub ghp_/Stripe sk-/AWS AKIA/OpenAI 风格 + PEM 私钥头。
var secretRe = regexp.MustCompile(`(ghp_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]+PRIVATE KEY-----)`)

// detectDanger 按资产类型启发检测 diff 中的危险变更(只检测不拦截)。
//
// 实现采用【结构化比较】而非 grep diff 行:diffmatchpatch 的 +/- 行只含变更片段,
// "deny"/"command"/"env" 等 JSON 键通常落在未变更的上下文里,不会出现在 diff 输出中。
// 因此这里直接解析 old/new 的原始 JSON,按字段语义对比:
//   - permission_deny_removed: permissions.deny/ask 数组的集合差(旧有新无=放宽)
//   - hook_command:            command 字符串变更
//   - mcp_env:                 env 对象新增键或由无变有
//   - secret_like:             新内容里命中密钥正则、且旧内容不含该串(新增密钥)
//
// 非 JSON 资产(skill/command/memory 等纯文本)解析失败时跳过结构化检查,
// 仅 secret_like 正则扫描仍生效(对原始文本)。
func detectDanger(a configengine.Asset, old, new string) []Danger {
	var out []Danger

	oldObj := parseJSONMap(old)
	newObj := parseJSONMap(new)

	switch a.Type {
	case configengine.AssetSettings, configengine.AssetPermissions:
		out = append(out, detectPermissionDenyRemoved(oldObj, newObj)...)
	case configengine.AssetHook:
		out = append(out, detectHookCommand(oldObj, newObj)...)
	case configengine.AssetMCPServer:
		out = append(out, detectMCPEnv(oldObj, newObj)...)
	}

	// secret_like 对所有资产类型生效:扫描新内容命中正则、且旧内容不含该匹配串。
	for _, m := range secretRe.FindAllString(new, -1) {
		if !strings.Contains(old, m) {
			out = append(out, Danger{Line: 1, Kind: "secret_like",
				Message: "新增内容疑似密钥"})
		}
	}

	return out
}

// parseJSONMap 解析 JSON 为 map[string]any;失败返回 nil。
func parseJSONMap(s string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// detectPermissionDenyRemoved 检测 permissions.deny / permissions.ask 中被移除的条目。
// 移除 deny/ask 规则 = 放宽安全管控。每个被移除的条目产生一条 Danger。
func detectPermissionDenyRemoved(old, new map[string]any) []Danger {
	oldDeny := permissionList(old, "deny")
	newDeny := permissionList(new, "deny")
	oldAsk := permissionList(old, "ask")
	newAsk := permissionList(new, "ask")

	newSet := make(map[string]bool, len(newDeny)+len(newAsk))
	for _, e := range newDeny {
		newSet[e] = true
	}
	for _, e := range newAsk {
		newSet[e] = true
	}

	var out []Danger
	check := func(list []string) {
		for _, e := range list {
			if !newSet[e] {
				out = append(out, Danger{Line: 1, Kind: "permission_deny_removed",
					Message: "移除 deny/ask 限制可能放宽安全管控"})
			}
		}
	}
	check(oldDeny)
	check(oldAsk)
	return out
}

// permissionList 从 obj["permissions"][key] 取 string 数组,防御性解析。
func permissionList(obj map[string]any, key string) []string {
	if obj == nil {
		return nil
	}
	perms, ok := obj["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	arr, ok := perms[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// detectHookCommand 检测 hook 的 command 字段变更。
// 任意 command 值变化都视为危险(可能引入执行/外发)。
func detectHookCommand(old, new map[string]any) []Danger {
	oc := commandOf(old)
	nc := commandOf(new)
	// 一边解析出 command、另一边没有,或两边都有但值不同,都视为变更。
	if oc != nc {
		// 只在至少一边能取到 command 时才报(避免无 command 字段的空对象误报)。
		if oc != "" || nc != "" {
			return []Danger{{Line: 1, Kind: "hook_command",
				Message: "hook command 变更,可能引入执行/外发"}}
		}
	}
	return nil
}

// commandOf 从对象取 "command" 字符串字段;不存在返回 ""。
func commandOf(obj map[string]any) string {
	if obj == nil {
		return ""
	}
	if s, ok := obj["command"].(string); ok {
		return s
	}
	return ""
}

// detectMCPEnv 检测 MCP server 的 env 对象变更(新增键或由无变有)。
// env 常含凭据,任何键集合扩大都视为危险。
func detectMCPEnv(old, new map[string]any) []Danger {
	oldEnv := envMap(old)
	newEnv := envMap(new)
	// 旧无 env、新有 env → 新增 env。
	if len(oldEnv) == 0 && len(newEnv) > 0 {
		return []Danger{{Line: 1, Kind: "mcp_env",
			Message: "MCP env 变更,可能含凭据外发"}}
	}
	// 旧有 env,检查新 env 是否多了键。
	for k := range newEnv {
		if _, ok := oldEnv[k]; !ok {
			return []Danger{{Line: 1, Kind: "mcp_env",
				Message: "MCP env 变更,可能含凭据外发"}}
		}
	}
	return nil
}

// envMap 从对象取 "env" 对象为 map[string]bool(键集合);不存在返回 nil。
func envMap(obj map[string]any) map[string]bool {
	if obj == nil {
		return nil
	}
	env, ok := obj["env"].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]bool, len(env))
	for k := range env {
		out[k] = true
	}
	return out
}
