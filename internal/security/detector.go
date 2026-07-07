package security

import (
	"context"

	"code-agent-sentinel/internal/configengine"
)

// Detector 是一个安全检测器。
type Detector interface {
	ID() string
	Covers() []configengine.AssetType
	Available() bool // 子进程/依赖是否就绪
	Reason() string  // 不可用时的原因
	Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error)
	Meta() DetectorMeta // 能力元数据(规则/引擎/覆盖),纯静态描述,不碰文件系统
}

// DetectorMeta 描述一个检测器具备的扫描能力,供 UI 展示。
type DetectorMeta struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`     // 中文显示名
	Engines []EngineInfo `json:"engines"`
	Rules   []RuleInfo   `json:"rules"`    // 内嵌规则,可为空(子进程检测器规则在外部工具内)
	Covers  []string     `json:"covers"`   // 覆盖的资产类型
}

// EngineInfo 描述一个扫描引擎(可能多个,如 dep 有 npm + govulncheck)。
type EngineInfo struct {
	Name      string `json:"name"`      // gitleaks / npm audit / govulncheck / 内嵌 YAML 规则引擎
	Kind      string `json:"kind"`      // subprocess / embedded / native
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// RuleInfo 是一条内嵌规则的摘要(不含 pattern 等实现细节)。
type RuleInfo struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
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
