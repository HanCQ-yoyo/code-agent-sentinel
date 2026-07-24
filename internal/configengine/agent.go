package configengine

import (
	"os"
	"path/filepath"
)

// Agent 是一个被安全管控的 code agent(其配置足迹)。
// 本轮注册 Claude Code 与 Codex CLI;未来加 agent 在 DefaultAgents/KnownAgents 注册。
type Agent struct {
	ID         string `json:"id"`          // "claude-code" | "codex"
	Name       string `json:"name"`        // "Claude Code" | "Codex CLI"
	Kind       string `json:"kind"`        // "claude-code" | "codex";决定用哪套解析器
	RootDir    string `json:"root_dir"`    // 配置根:Claude ~/.claude | Codex ~/.codex
	ClaudeJSON string `json:"claude_json"` // 机器管理文件:Claude ~/.claude.json;Codex 空
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
// Claude 已实现解析;Codex spec 已注册,其解析器(config.toml + AGENTS.md 等)在后续任务实现。
type AgentSpec struct {
	ID                string
	Name              string
	Kind              string // 与 Agent.Kind 对应;决定用哪套解析器
	DefaultRootDir    func(home string) string
	DefaultClaudeJSON func(home string) string
	Detect            func(home string) bool
	HasClaudeJSON     bool
}

// KnownAgents 返回内置 agent 清单(Claude Code + Codex CLI)。
func KnownAgents() []AgentSpec {
	return []AgentSpec{
		{
			ID:                "claude-code",
			Name:              "Claude Code",
			Kind:              "claude-code",
			DefaultRootDir:    func(home string) string { return filepath.Join(home, ".claude") },
			DefaultClaudeJSON: func(home string) string { return filepath.Join(home, ".claude.json") },
			Detect:            func(home string) bool { _, err := os.Stat(filepath.Join(home, ".claude")); return err == nil },
			HasClaudeJSON:     true,
		},
		{
			ID:                "codex",
			Name:              "Codex CLI",
			Kind:              "codex",
			DefaultRootDir:    func(home string) string { return filepath.Join(home, ".codex") },
			DefaultClaudeJSON: func(home string) string { return "" },
			Detect:            func(home string) bool { _, err := os.Stat(filepath.Join(home, ".codex", "config.toml")); return err == nil },
			HasClaudeJSON:     false,
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

// AgentsFromSpecs 把配置足迹映射成 []Agent(填 HomeDir + Kind)。Enabled 过滤由调用方负责。
// Kind 与默认路径按 ID 从 KnownAgents() 查 spec:codex 回退 ~/.codex、claude 回退 ~/.claude。
// config.AgentCfg 不持 Kind——避免 config 层硬编码 agent 厂商清单,统一由 configengine spec 决定。
func AgentsFromSpecs(home string, items []AgentItem) []Agent {
	specs := map[string]AgentSpec{}
	for _, s := range KnownAgents() {
		specs[s.ID] = s
	}
	out := make([]Agent, 0, len(items))
	for _, it := range items {
		spec, ok := specs[it.ID]
		if !ok {
			// 未知 agent ID:回退当 claude-code(向后兼容旧配置/拼写)。
			spec = specs["claude-code"]
		}
		root := it.RootDir
		if root == "" {
			root = spec.DefaultRootDir(home)
		}
		cj := it.ClaudeJSON
		if cj == "" {
			cj = spec.DefaultClaudeJSON(home)
		}
		out = append(out, Agent{
			ID: it.ID, Name: spec.Name, Kind: spec.Kind, RootDir: root, ClaudeJSON: cj, HomeDir: home,
		})
	}
	return out
}
