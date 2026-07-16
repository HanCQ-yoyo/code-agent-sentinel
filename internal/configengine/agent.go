package configengine

import "path/filepath"

// Agent 是一个被安全管控的 code agent(其配置足迹)。
// 本轮只注册 Claude Code;未来加 agent(Cursor 等)在 DefaultAgents 注册。
type Agent struct {
	ID         string `json:"id"`          // "claude-code"
	Name       string `json:"name"`        // "Claude Code"
	RootDir    string `json:"root_dir"`    // 配置根:~/.claude
	ClaudeJSON string `json:"claude_json"` // 机器管理文件:~/.claude.json
	HomeDir    string `json:"-"`           // 注入用,不外露
}

// DefaultAgents 返回内置 agent 列表(本轮仅 Claude Code)。
// claudeDir 空 → home/.claude;ClaudeJSON 始终 home/.claude.json。
func DefaultAgents(home, claudeDir string) []Agent {
	if claudeDir == "" {
		claudeDir = filepath.Join(home, ".claude")
	}
	return []Agent{
		{
			ID:         "claude-code",
			Name:       "Claude Code",
			RootDir:    claudeDir,
			ClaudeJSON: filepath.Join(home, ".claude.json"),
			HomeDir:    home,
		},
	}
}
