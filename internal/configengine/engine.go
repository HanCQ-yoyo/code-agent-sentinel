package configengine

import "path/filepath"

// Engine 发现并解析 Claude Code 配置资产。所有路径注入,便于测试。
type Engine struct {
	HomeDir    string // 用户的 home(~)
	ClaudeJSON string // ~/.claude.json
}

// NewEngine 用默认布局构造 Engine(home/.claude + home/.claude.json)。
func NewEngine(home string) *Engine {
	return &Engine{
		HomeDir:    home,
		ClaudeJSON: filepath.Join(home, ".claude.json"),
	}
}

// ListProjects 从 ~/.claude.json 的 projects 字段列出已知项目。
func (e *Engine) ListProjects() ([]Project, error) {
	return readProjectList(e.ClaudeJSON)
}

// NewEngineFromAgent 用 agent 描述构造 Engine。本轮 Claude Code 等价 NewEngine(a.HomeDir),
// 但 agent 描述显式化,为多 agent 铺路。
func NewEngineFromAgent(a Agent) *Engine {
	return NewEngine(a.HomeDir)
}
