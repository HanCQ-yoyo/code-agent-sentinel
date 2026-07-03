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
