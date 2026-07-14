package security

import (
	"context"

	"code-agent-sentinel/internal/configengine"
)

// Detector 是一个安全检测器。
type Detector interface {
	ID() string
	Covers() []configengine.AssetType
	Enabled() bool   // 用户是否启用(配置开关);false 时编排器跳过
	Available() bool // 子进程/依赖是否就绪
	Reason() string  // 不可用时的原因
	Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error)
	Meta() DetectorMeta // 能力元数据(规则/引擎/覆盖),纯静态描述,不碰文件系统
}

// DetectorMeta 描述一个检测器具备的扫描能力,供 UI 展示。
type DetectorMeta struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`     // 中文显示名
	Enabled bool         `json:"enabled"`   // 用户启用开关(false=已禁用)
	Engines []EngineInfo `json:"engines"`
	Rules   []RuleInfo   `json:"rules"`    // 内嵌规则,可为空(子进程检测器规则在外部工具内)
	Covers  []string     `json:"covers"`   // 覆盖的资产类型
}

// EngineInfo 描述一个扫描引擎(可能多个,如 dep 有 npm + govulncheck)。
type EngineInfo struct {
	Name      string `json:"name"`      // gitleaks / npm audit / govulncheck / 内嵌 YAML 规则引擎
	Kind      string `json:"kind"`      // subprocess / embedded / native
	Enabled   bool   `json:"enabled"`   // 引擎级启用(dep 的 npm/govulncheck 可独立)
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// RuleInfo 是一条内嵌规则的摘要(含可读语法或正则,供发现页抽屉展示规则表达式)。
type RuleInfo struct {
	ID            string          `json:"id"`
	Severity      string          `json:"severity"`
	AssetType     string          `json:"asset_type"`
	Description   string          `json:"description"`
	Syntax        string          `json:"syntax,omitempty"` // 可读语法(baseline:按 op 拼)或正则原文(injection)
	Remediation   string          `json:"remediation,omitempty"`
	Paths         *PathFilterInfo `json:"paths,omitempty"`
	PostExclude   []string        `json:"post_exclude,omitempty"`
	Deobfuscation []string        `json:"deobfuscation,omitempty"`
	Dotall        bool            `json:"dotall,omitempty"`
	Metadata      map[string]any  `json:"metadata,omitempty"`
	SourceFile    string          `json:"source_file,omitempty"`
	ProjectPath   string          `json:"project_path,omitempty"`
}

// PathFilterInfo 是 RuleInfo 中路径过滤的可读表示(对应 ruleengine.PathFilter)。
type PathFilterInfo struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// Registry 管理已注册检测器。
type Registry struct{ list []Detector }

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) Register(d Detector)   { r.list = append(r.list, d) }
func (r *Registry) Detectors() []Detector { return r.list }

func (r *Registry) Get(id string) Detector {
	for _, d := range r.list {
		if d.ID() == id {
			return d
		}
	}
	return nil
}
