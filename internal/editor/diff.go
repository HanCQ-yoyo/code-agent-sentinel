package editor

import (
	"encoding/json"
	"fmt"
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
//   - hook_command:            Settings/Permissions(嵌套 hooks 布局)+ Hook(扁平/嵌套)的 command 字符串变更
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
		out = append(out, detectHookCommand(oldObj, newObj)...)
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

// detectHookCommand 检测 hook 的 command 字段变更(可能引入执行/外发)。
//
// 支持 configengine 的两种 hooks 布局:
//  1. settings.json 布局:hooks 嵌套在 "hooks" 键下 → map[event][]entry,
//     entry = {"matcher":..., "hooks":[{"type":..., "command":...}]}。
//  2. hooks.json 布局:顶层即 map[event][]entry(无 "hooks" 包装)。
//
// 另保留扁平退化支持:顶层 "command" 字符串(无 hooks/events 结构)视为单 hook,
// 使 brief 的 TestDetectDangerHookCommand({"command":...}) 仍通过。
//
// 比较策略:按 hook 标识(event/entry-index/hook-index 或扁平)收集 old/new 的
// command 列表,同一标识下 command 值不同(含"无→有")→ 一条 Danger。
func detectHookCommand(old, new map[string]any) []Danger {
	oldCmds := extractHookCommands(old)
	newCmds := extractHookCommands(new)

	// 无任何 hook 结构(old/new 都为空):扁平退化,按旧逻辑直接比顶层 command。
	if len(oldCmds) == 0 && len(newCmds) == 0 {
		oc := flatCommand(old)
		nc := flatCommand(new)
		if oc != nc && (oc != "" || nc != "") {
			return []Danger{{Line: 1, Kind: "hook_command",
				Message: "hook command 变更,可能引入执行/外发"}}
		}
		return nil
	}

	// 按 key 收集 new commands。
	newBy := make(map[string]string, len(newCmds))
	for _, c := range newCmds {
		newBy[c.key] = c.command
	}

	// 先看 old 中的 command 在 new 是否变化/消失。
	var out []Danger
	seen := make(map[string]bool, len(oldCmds))
	for _, c := range oldCmds {
		seen[c.key] = true
		nc, ok := newBy[c.key]
		if !ok {
			continue // 同 key 在 new 不存在:hook 被删,command 变更非安全放宽,跳过。
		}
		if c.command != nc && (c.command != "" || nc != "") {
			out = append(out, Danger{Line: 1, Kind: "hook_command",
				Message: "hook command 变更,可能引入执行/外发"})
		}
	}
	// 再看 new 中新增的 hook(旧无此 key 且有 command)= 新引入 command。
	for _, c := range newCmds {
		if seen[c.key] {
			continue
		}
		if c.command != "" {
			out = append(out, Danger{Line: 1, Kind: "hook_command",
				Message: "hook command 变更,可能引入执行/外发"})
		}
	}
	return out
}

// hookCmd 是一条 hook command 的提取结果:key 为唯一标识(event/entry 索引/hook 索引,
// 或扁平的 "_flat"),command 为 command 字符串值。
type hookCmd struct {
	key     string
	command string
}

// extractHookCommands 从已解析的 JSON map 提取所有 hook command,支持两种布局。
// 返回的 key 形如 "PreToolUse/0/0"(event/entry-index/hook-index),保证跨 old/new 可比。
// 同 key 在 map 里因 entry 顺序确定(json.Unmarshal),跨运行可复现。
func extractHookCommands(m map[string]any) []hookCmd {
	if m == nil {
		return nil
	}
	// 布局 1:settings.json — hooks 嵌套在 "hooks" 键下。
	if hooks, ok := m["hooks"].(map[string]any); ok {
		return hooksMapToCmds(hooks)
	}
	// 布局 2:hooks.json — 顶层即 hooks map。
	if cmds := hooksMapToCmds(m); len(cmds) > 0 {
		return cmds
	}
	return nil
}

// hooksMapToCmds 遍历 map[event][]entry 提取 command。
// entry 为 {"matcher":..., "hooks":[{"type":..., "command":...}]},
// 按位置(event → entry 索引 ei → hook 索引 hi)生成 key,与 configengine
// parseHooksFromData 的 Name 格式一致(event/matcher/ei/hi),但这里省略 matcher
// (matcher 变更属配置变更,不应影响 command 比较的路径稳定性)。
func hooksMapToCmds(hooks map[string]any) []hookCmd {
	var out []hookCmd
	for event, entriesVal := range hooks {
		entries, ok := entriesVal.([]any)
		if !ok {
			continue
		}
		for ei, entryVal := range entries {
			entry, ok := entryVal.(map[string]any)
			if !ok {
				continue
			}
			hooksArr, ok := entry["hooks"].([]any)
			if !ok {
				continue
			}
			for hi, hookVal := range hooksArr {
				hook, ok := hookVal.(map[string]any)
				if !ok {
					continue
				}
				cmd, _ := hook["command"].(string)
				out = append(out, hookCmd{
					key:     fmt.Sprintf("%s/%d/%d", event, ei, hi),
					command: cmd,
				})
			}
		}
	}
	return out
}

// flatCommand 取顶层 "command" 字符串(扁平退化场景);不存在返回 ""。
func flatCommand(obj map[string]any) string {
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
