package editor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

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
	AgentID    string `json:"agent_id"`    // 资产所属 agent ID,空则回退 Editor.Engine(首 agent)
	NewContent string `json:"new_content"`
	BaseHash   string `json:"base_hash"` // 编辑开始时资产 hash(乐观锁)
}

// PreviewResult 是 Preview 的只读输出(不写盘)。
// OriginalContent 是资产源文件的原始磁盘内容,供前端初始化编辑 draft——
// structured 资产(settings/permissions/hooks/mcp_server/keybinding)的 fields.raw
// 是 json.RawMessage(marshal 为 JSON 对象而非字符串)或根本没有 raw 字段,
// 前端若用 JSON.stringify(fields) 做 draft 会写入整个 fields 包装而非原始文件内容 →
// 损坏文件。OriginalContent 直接来自 os.ReadFile(SourcePath),保证 draft = 真实文件内容。
type PreviewResult struct {
	Diff              string   `json:"diff"`
	Dangerous         []Danger `json:"dangerous"`
	BaseHashOK        bool     `json:"base_hash_ok"`
	CurrentHash       string   `json:"current_hash"`
	Editable          bool     `json:"editable"`
	NotEditableReason string   `json:"not_editable_reason,omitempty"`
	OriginalContent   string   `json:"original_content"`
}

// EditResult 是 Commit 的写盘输出。
type EditResult struct {
	Asset      configengine.Asset `json:"asset"`
	BackupPath string             `json:"backup_path"`
	Diff       string             `json:"diff"`
	Dangerous  []Danger           `json:"dangerous"`
}

// Editor 是配置资产写层。Engine 是首 agent 的回退引擎;Agents + engines 支持多 agent 编辑。
type Editor struct {
	Engine     *configengine.Engine
	Agents     []configengine.Agent
	engines    map[string]*configengine.Engine
	mu         sync.Mutex
	BackupDir  string
	MaxBackups int
}

// New 构造 Editor。backupDir 空则默认 <home>/.code-agent-sentinel/backups;maxBackups<=0 则 20。
// engine 是首 agent 回退引擎;agents 是多 agent 清单(用于按 agentID 发现对应资产)。
func New(engine *configengine.Engine, agents []configengine.Agent, backupDir string, maxBackups int) *Editor {
	if backupDir == "" {
		backupDir = filepath.Join(engine.HomeDir, ".code-agent-sentinel", "backups")
	}
	if maxBackups <= 0 {
		maxBackups = 20
	}
	return &Editor{Engine: engine, Agents: agents, engines: map[string]*configengine.Engine{}, BackupDir: backupDir, MaxBackups: maxBackups}
}

// engineFor 返回 agentID 的 Engine(懒构造)。空 agentID/find 不到则回退至首 agent Engine。
func (e *Editor) engineFor(agentID string) *configengine.Engine {
	e.mu.Lock()
	defer e.mu.Unlock()
	if agentID == "" && len(e.Agents) > 0 {
		agentID = e.Agents[0].ID
	}
	if eng, ok := e.engines[agentID]; ok {
		return eng
	}
	var a configengine.Agent
	for _, x := range e.Agents {
		if x.ID == agentID {
			a = x
			break
		}
	}
	if a.ID == "" {
		return e.Engine // 找不到该 agent,回退首 agent 引擎
	}
	eng := configengine.NewEngineFromAgent(a)
	e.engines[agentID] = eng
	return eng
}

// validateContent 校验新内容语法。JSON 资产须可解析,其余不校验语法。
func (e *Editor) validateContent(a configengine.Asset, content string) error {
	switch a.Type {
	case configengine.AssetSettings, configengine.AssetPermissions,
		configengine.AssetMCPServer, configengine.AssetKeybinding, configengine.AssetHook:
		var v any
		if err := json.Unmarshal([]byte(content), &v); err != nil {
			return ErrBadContent
		}
	}
	return nil
}

// Preview 只读:算 diff + 危险检测 + 乐观锁校验,不写盘。
// 资产不存在 → 返回 (nil, ErrNotFound);不可编辑 → PreviewResult{Editable:false}。
func (e *Editor) Preview(ctx context.Context, req EditRequest) (*PreviewResult, error) {
	eng := e.engineFor(req.AgentID)
	a, ok := e.findAsset(req.AssetID, eng)
	if !ok {
		return nil, ErrNotFound
	}
	editable, reason := e.editable(a)
	current, _ := os.ReadFile(a.SourcePath)
	currentHash := sha256hex(current)
	if !editable {
		return &PreviewResult{Editable: false, NotEditableReason: reason, CurrentHash: currentHash, OriginalContent: string(current)}, nil
	}
	old := string(current)
	diff := computeDiff(old, req.NewContent)
	dangers := detectDanger(a, old, req.NewContent)
	return &PreviewResult{
		Diff:            diff,
		Dangerous:       dangers,
		BaseHashOK:      currentHash == req.BaseHash,
		CurrentHash:     currentHash,
		Editable:        true,
		OriginalContent: old,
	}, nil
}

// Commit 写盘:可编辑校验 + 乐观锁 + 内容校验 + 备份 + 原子写 + 重算。
func (e *Editor) Commit(ctx context.Context, req EditRequest) (*EditResult, error) {
	eng := e.engineFor(req.AgentID)
	a, ok := e.findAsset(req.AssetID, eng)
	if !ok {
		return nil, ErrNotFound
	}
	editable, reason := e.editable(a)
	if !editable {
		_ = reason
		return nil, ErrNotEditable
	}
	current, err := os.ReadFile(a.SourcePath)
	if err != nil {
		return nil, err
	}
	currentHash := sha256hex(current)
	if currentHash != req.BaseHash {
		return nil, ErrConcurrentModification
	}
	if err := e.validateContent(a, req.NewContent); err != nil {
		return nil, err
	}
	// 备份旧内容
	bp, err := e.backup(a, current)
	if err != nil {
		return nil, err
	}
	// 原子写(tmp 同目录 + rename),0o600
	if err := atomicWrite(a.SourcePath, []byte(req.NewContent)); err != nil {
		return nil, err
	}
	// 重算 hash/mtime
	hash, mtime, _ := configengine.HashAndMTime(a.SourcePath)
	a.Hash = hash
	a.MTime = mtime
	a.Content = req.NewContent
	diff := computeDiff(string(current), req.NewContent)
	dangers := detectDanger(a, string(current), req.NewContent)
	return &EditResult{
		Asset:      a,
		BackupPath: bp,
		Diff:       diff,
		Dangerous:  dangers,
	}, nil
}

// atomicWrite 用 tmp+rename 原子写入 path,文件 0o600。tmp 与目标同目录。
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sentinel-edit-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
