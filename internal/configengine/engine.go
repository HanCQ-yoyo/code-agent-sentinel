package configengine

import "path/filepath"

// Engine 发现并解析 Claude Code 配置资产。所有路径注入,便于测试。
type Engine struct {
	HomeDir    string // 用户的 home(~)
	ClaudeJSON string // ~/.claude.json
	Project    *Project
}

// NewEngine 用默认布局构造 Engine(home/.claude + home/.claude.json)。
func NewEngine(home string) *Engine {
	return &Engine{
		HomeDir:    home,
		ClaudeJSON: filepath.Join(home, ".claude.json"),
	}
}

// SelectProject 设置当前项目。
func (e *Engine) SelectProject(p Project) { e.Project = &p }

// ListProjects 从 ~/.claude.json 的 projects 字段列出已知项目。
func (e *Engine) ListProjects() ([]Project, error) {
	return readProjectList(e.ClaudeJSON)
}
