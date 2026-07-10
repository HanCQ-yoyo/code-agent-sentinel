package editor

import (
	"path/filepath"

	"code-agent-sentinel/internal/configengine"
)

// Danger 标记 diff 中一处危险变更。
type Danger struct {
	Line    int    `json:"line"`    // diff 中的行号(1-based)
	Kind    string `json:"kind"`    // permission_deny_removed / hook_command / mcp_env / secret_like
	Message string `json:"message"` // 人语说明
}

// EditRequest 是 Preview 与 Commit 的共同输入。
type EditRequest struct {
	AssetID    string `json:"asset_id"`
	NewContent string `json:"new_content"`
	BaseHash   string `json:"base_hash"` // 编辑开始时资产 hash(乐观锁)
}

// PreviewResult 是 Preview 的只读输出(不写盘)。
type PreviewResult struct {
	Diff              string   `json:"diff"`
	Dangerous         []Danger `json:"dangerous"`
	BaseHashOK        bool     `json:"base_hash_ok"`
	CurrentHash       string   `json:"current_hash"`
	Editable          bool     `json:"editable"`
	NotEditableReason string   `json:"not_editable_reason,omitempty"`
}

// EditResult 是 Commit 的写盘输出。
type EditResult struct {
	Asset      configengine.Asset `json:"asset"`
	BackupPath string             `json:"backup_path"`
	Diff       string             `json:"diff"`
	Dangerous  []Danger           `json:"dangerous"`
}

// Editor 是配置资产写层。Engine 只读;BackupDir/MaxBackups 注入。
type Editor struct {
	Engine     *configengine.Engine
	BackupDir  string
	MaxBackups int
}

// New 构造 Editor。backupDir 空则默认 <home>/.claude-sentinel/backups;maxBackups<=0 则 20。
func New(engine *configengine.Engine, backupDir string, maxBackups int) *Editor {
	if backupDir == "" {
		backupDir = filepath.Join(engine.HomeDir, ".claude-sentinel", "backups")
	}
	if maxBackups <= 0 {
		maxBackups = 20
	}
	return &Editor{Engine: engine, BackupDir: backupDir, MaxBackups: maxBackups}
}

// Preview 与 Commit 在后续任务实现。
