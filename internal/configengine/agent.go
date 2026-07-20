package configengine

import (
	"os"
	"path/filepath"
)

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

// AgentSpec 是一个已知 code agent 的内置描述(setup 渲染可选清单 + 探测默认路径用)。
// 只列已实现解析的 agent;Codex 等待其解析器实现后再加入。
type AgentSpec struct {
	ID                string
	Name              string
	DefaultRootDir    func(home string) string
	DefaultClaudeJSON func(home string) string
	Detect            func(home string) bool
	HasClaudeJSON     bool
}

// KnownAgents 返回内置 agent 清单(本轮仅 Claude Code)。
func KnownAgents() []AgentSpec {
	return []AgentSpec{
		{
			ID:                "claude-code",
			Name:              "Claude Code",
			DefaultRootDir:    func(home string) string { return filepath.Join(home, ".claude") },
			DefaultClaudeJSON: func(home string) string { return filepath.Join(home, ".claude.json") },
			Detect:            func(home string) bool { _, err := os.Stat(filepath.Join(home, ".claude")); return err == nil },
			HasClaudeJSON:     true,
		},
	}
}

// AgentItem 是 AgentSpec 的配置足迹(从 config.AgentCfg 桥接而来,避免 configengine 导入 config)。
type AgentItem struct {
	ID         string
	Enabled    bool
	RootDir    string
	ClaudeJSON string
}

// AgentsFromSpecs 把配置足迹映射成 []Agent(填 HomeDir)。Enabled 过滤由调用方负责。
func AgentsFromSpecs(home string, items []AgentItem) []Agent {
	names := map[string]string{}
	for _, s := range KnownAgents() {
		names[s.ID] = s.Name
	}
	out := make([]Agent, 0, len(items))
	for _, it := range items {
		root := it.RootDir
		if root == "" {
			root = filepath.Join(home, ".claude")
		}
		cj := it.ClaudeJSON
		if cj == "" {
			cj = filepath.Join(home, ".claude.json")
		}
		out = append(out, Agent{
			ID: it.ID, Name: names[it.ID], RootDir: root, ClaudeJSON: cj, HomeDir: home,
		})
	}
	return out
}
