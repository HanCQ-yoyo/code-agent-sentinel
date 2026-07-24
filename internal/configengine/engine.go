package configengine

import "path/filepath"

// Engine 发现并解析 Claude Code 配置资产。所有路径注入,便于测试。
type Engine struct {
	HomeDir    string // 用户的 home(~)
	ClaudeDir  string // 全局 .claude 目录(空 → home/.claude);项目级 .claude 不受此影响
	ClaudeJSON string // ~/.claude.json(机器管理文件,不随 .claude 移动)
	Kind       string // "claude-code" | "codex";决定 Discover 用哪套解析器。空=claude(向后兼容)
	// DisabledAssetTypes 按资产类型关闭发现(空 = 全发现)。由 main.go 从 config 桥接。
	DisabledAssetTypes []AssetType
}

// NewEngine 构造 Engine。claudeDir 空 → home/.claude;.claude.json 始终 home/.claude.json。
func NewEngine(home, claudeDir string) *Engine {
	if claudeDir == "" {
		claudeDir = filepath.Join(home, ".claude")
	}
	return &Engine{
		HomeDir:    home,
		ClaudeDir:  claudeDir,
		ClaudeJSON: filepath.Join(home, ".claude.json"),
	}
}

// ListProjects 从 ~/.claude.json 的 projects 字段列出已知项目。
func (e *Engine) ListProjects() ([]Project, error) {
	return readProjectList(e.ClaudeJSON)
}

// NewEngineFromAgent 用 agent 描述构造 Engine。本轮 Claude Code 等价 NewEngine(a.HomeDir, a.RootDir),
// 但 agent 描述显式化,为多 agent 铺路。
func NewEngineFromAgent(a Agent) *Engine {
	e := NewEngine(a.HomeDir, a.RootDir)
	e.Kind = a.Kind
	return e
}

// isAssetTypeDisabled 判断某资产类型是否被关闭发现。
func (e *Engine) isAssetTypeDisabled(t AssetType) bool {
	for _, d := range e.DisabledAssetTypes {
		if d == t {
			return true
		}
	}
	return false
}
