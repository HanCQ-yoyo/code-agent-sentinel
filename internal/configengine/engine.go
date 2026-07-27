package configengine

import "path/filepath"

// Engine 发现并解析 Claude Code 配置资产。所有路径注入,便于测试。
type Engine struct {
	HomeDir    string // 用户的 home(~)
	ClaudeDir  string // 全局 .claude 目录(空 → home/.claude);项目级 .claude 不受此影响
	ClaudeJSON string // ~/.claude.json(机器管理文件,不随 .claude 移动)
	Kind       string // "claude-code" | "codex";决定 Discover 用哪套解析器。空=claude(向后兼容)
	// KnownProjects 是项目级发现的已知项目清单(从 sentinel config 桥接,共享于所有 agent)。
	// Codex 项目级发现读此清单(不读 ~/.claude.json);Claude 同样改读此清单(Task 7)。
	// 共享引用,只读;改写须先拷(见 Task 4 控制器补充决议)。
	KnownProjects []Project
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

// ListProjects 返回已知项目清单。优先 Engine.KnownProjects(从 sentinel config 桥接,
// Task 4 起 Codex 不再借 ~/.claude.json);空时回退读 ~/.claude.json projects(Claude 零破坏,
// Codex 空 ClaudeJSON → nil 安全降级)。共享接口:被 discoverProjects / loadProjectRules /
// API handlers / editor 复用,故回退保留而非删除。
func (e *Engine) ListProjects() ([]Project, error) {
	if len(e.KnownProjects) > 0 {
		return e.KnownProjects, nil
	}
	return readProjectList(e.ClaudeJSON)
}

// NewEngineFromAgent 用 agent 描述构造 Engine。显式拷 ClaudeJSON:Claude agent 指向 ~/.claude.json,
// Codex agent 为空(不读 Claude 机器文件)。Kind 决定 Discover 用哪套解析器。
// 不再委托 NewEngine(它硬编码 ~/.claude.json,是 Codex 误用 Claude 机器文件的根因)。
func NewEngineFromAgent(a Agent) *Engine {
	claudeDir := a.RootDir
	if claudeDir == "" {
		claudeDir = filepath.Join(a.HomeDir, ".claude")
	}
	return &Engine{
		HomeDir:       a.HomeDir,
		ClaudeDir:     claudeDir,
		ClaudeJSON:    a.ClaudeJSON, // Codex 为空
		Kind:          a.Kind,
		KnownProjects: a.KnownProjects, // 共享引用,只读;改写须先拷
	}
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
