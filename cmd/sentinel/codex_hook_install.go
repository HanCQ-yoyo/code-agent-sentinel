package main

import (
	"os"
	"strings"
)

// InstallCodexHook 写 ~/.codex/hooks.json 注册 sentinel guard 为 PreToolUse Bash hook。
//
// 结构与 Claude settings.json hooks 段同形(spec §3.6):
//
//	{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"<abs>/sentinel guard"}]}]}}
//
// matcher="Bash"(Codex canonical shell tool_name)。复用 R2 hook_install.go 的
// loadSettings/saveSettings/isSentinelGuardCommand(逻辑相同,只是路径不同)。
// 幂等:已存在不重复加。返回 changed=true 表示文件被改写。
func InstallCodexHook(hooksPath, sentinelPath string) (bool, error) {
	settings, err := loadSettings(hooksPath)
	if err != nil {
		return false, err
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}
	pre, _ := hooks["PreToolUse"].([]any)

	cmd := sentinelPath + " guard"
	// 查找已有 Bash matcher
	for _, entry := range pre {
		e, ok := entry.(map[string]any)
		if !ok || e["matcher"] != "Bash" {
			continue
		}
		hl, _ := e["hooks"].([]any)
		for _, h := range hl {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if c, _ := hm["command"].(string); strings.HasSuffix(c, "sentinel guard") && isSentinelGuardCommand(c, sentinelPath) {
				return false, nil // 已存在,幂等
			}
		}
		// 插入到该 matcher 的 hooks 首位
		e["hooks"] = append([]any{map[string]any{"type": "command", "command": cmd}}, hl...)
		hooks["PreToolUse"] = pre
		return saveSettings(hooksPath, settings)
	}
	// 无 Bash matcher,新建
	newEntry := map[string]any{
		"matcher": "Bash",
		"hooks":   []any{map[string]any{"type": "command", "command": cmd}},
	}
	hooks["PreToolUse"] = append(pre, newEntry)
	return saveSettings(hooksPath, settings)
}

// UninstallCodexHook 移除 ~/.codex/hooks.json 的 sentinel guard hook 条目。
// 幂等:文件不存在视为已卸载,二次卸载 changed=false。
func UninstallCodexHook(hooksPath string) (bool, error) {
	settings, err := loadSettings(hooksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return false, nil
	}
	pre, _ := hooks["PreToolUse"].([]any)
	changed := false
	var kept []any
	for _, entry := range pre {
		e, ok := entry.(map[string]any)
		if !ok || e["matcher"] != "Bash" {
			kept = append(kept, entry)
			continue
		}
		hl, _ := e["hooks"].([]any)
		var hk []any
		for _, h := range hl {
			hm, ok := h.(map[string]any)
			if !ok {
				hk = append(hk, h)
				continue
			}
			if c, _ := hm["command"].(string); strings.Contains(c, "sentinel guard") {
				changed = true
				continue // 移除
			}
			hk = append(hk, h)
		}
		if len(hk) == 0 {
			changed = true // matcher 空,整个移除
			continue
		}
		e["hooks"] = hk
		kept = append(kept, e)
	}
	if !changed {
		return false, nil
	}
	hooks["PreToolUse"] = kept
	return saveSettings(hooksPath, settings)
}
