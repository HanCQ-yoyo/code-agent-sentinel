package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallGuardHook 写 ~/.claude/settings.json 注册 sentinel guard 为 PreToolUse Bash hook。
// 幂等:已存在不重复加。
// sentinel 置首。basename 精确匹配避免误匹配含 "sentinel" 子串的工具。
// 返回 changed=true 表示文件被改写。
func InstallGuardHook(settingsPath, sentinelPath string) (bool, error) {
	settings, err := loadSettings(settingsPath)
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
		return saveSettings(settingsPath, settings)
	}
	// 无 Bash matcher,新建
	newEntry := map[string]any{
		"matcher": "Bash",
		"hooks":   []any{map[string]any{"type": "command", "command": cmd}},
	}
	hooks["PreToolUse"] = append(pre, newEntry)
	return saveSettings(settingsPath, settings)
}

// UninstallGuardHook 移除 sentinel guard hook 条目。幂等。
func UninstallGuardHook(settingsPath string) (bool, error) {
	settings, err := loadSettings(settingsPath)
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
	return saveSettings(settingsPath, settings)
}

// isSentinelGuardCommand 判断 command 是否是当前 sentinelPath 的 guard 调用(精确路径匹配)。
// 用 shlex.split + basename 的精确匹配思路。
func isSentinelGuardCommand(command, sentinelPath string) bool {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return false
	}
	// 精确路径匹配:parts[0] 必须等于 sentinelPath,parts[1] 必须是 "guard"
	return parts[0] == sentinelPath && parts[1] == "guard"
}

func loadSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("settings.json 解析失败: %w", err)
	}
	if s == nil {
		s = map[string]any{}
	}
	return s, nil
}

func saveSettings(path string, s map[string]any) (bool, error) {
	// 用标准库 filepath.Dir 处理路径分隔符(跨平台),替代 brief 中的自定义 filepathDir。
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
