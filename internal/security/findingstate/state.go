// Package findingstate 实现处置生命周期覆盖层(取代 baseline + suppressions)。
//
// 一份 fingerprint→处置状态 的映射,持久化到 sqlite finding_states 表。
// 与扫描快照分离:扫描确定性地报 finding,处置状态按 fingerprint 键挂覆盖层。
// 资产/规则未变则 fingerprint 稳定 → 覆盖层自动重新 attach,已处置状态保留。
//
// 本包刻意保持纯净:不 import security 包(会产生循环依赖——
// security 包的 RulesDetector 调用 applyFindingState,后者引用本包类型)。
// Finding 的变异逻辑放在 security 包的 findingstate_apply.go 里。
package findingstate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"code-agent-sentinel/internal/storage"
)

// Status 是处置生命周期状态。
type Status string

const (
	StatusOpen          Status = "open"           // 未处置(默认)
	StatusInProgress    Status = "in_progress"    // 处置中,已认领待办
	StatusResolved      Status = "resolved"       // 已修复
	StatusFalsePositive Status = "false_positive" // 误报,规则误判
	StatusAccepted      Status = "accepted"       // 已知风险,接受不修
)

// Source 标记处置状态的来源(可追溯)。
type Source string

const (
	SourceManual           Source = "manual"            // 手动单条处置
	SourceBulkAccept       Source = "bulk-accept"       // 批量接受全部当前未处置
	SourceMigratedBaseline Source = "migrated-baseline" // 迁移自 baseline.json
	SourceMigratedInline   Source = "migrated-inline"   // 迁移自 suppressions.yaml
)

// State 是单条处置状态记录,fingerprint 为键。
type State struct {
	Fingerprint string `yaml:"fingerprint" json:"fingerprint"`
	Status      Status `yaml:"status" json:"status"`
	Priority    string `yaml:"priority,omitempty" json:"priority,omitempty"` // P0|P1|P2|P3;空=读时从 severity 派生
	Note        string `yaml:"note,omitempty" json:"note,omitempty"`
	Source      Source `yaml:"source,omitempty" json:"source,omitempty"`
	UpdatedAt   string `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// States 是处置状态集合。持有 sqlite 句柄(db)用于持久化,Items 是内存副本
// 供 Match/BulkAccept/PruneReport 操作。Set/Remove 同步写 db 和内存。
//
// 持久化模型:Set/Remove/BulkAccept 在 db 非 nil 时立即写 db,调用方无需调额外
// 的 Save() 方法。Save(path) 保留仅用于 MigrateFromLegacy(YAML 迁移路径)。
type States struct {
	db    *storage.DB
	Items []State `yaml:"items"`
}

// NewStates 构造 States 并从 db 加载内存副本。db 为 nil 时 Items 为空,
// Set/Remove 仅操作内存(Save 为 no-op),兼容旧测试及 Load() 静态路径。
func NewStates(db *storage.DB) *States {
	s := &States{db: db}
	s.reload()
	return s
}

// reload 从 db 重新加载内存副本。
func (s *States) reload() {
	if s.db == nil {
		return
	}
	rows, err := storage.ListFindingStates(s.db)
	if err != nil {
		return
	}
	items := make([]State, 0, len(rows))
	for _, r := range rows {
		items = append(items, State{
			Fingerprint: r.Fingerprint,
			Status:      Status(r.Status),
			Priority:    r.Priority,
			Note:        r.Note,
			Source:      Source(r.Source),
			UpdatedAt:   r.UpdatedAt,
		})
	}
	s.Items = items
}

// Load 从 YAML 文件加载处置状态(静态函数,供 MigrateFromLegacy 等旧路径用)。
// 文件不存在时返回 (nil, nil)。
func Load(path string) (*States, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read finding_states %s: %w", path, err)
	}
	var s States
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse finding_states %s: %w", path, err)
	}
	return &s, nil
}

// Save 以 YAML 写入指定路径(文件权限 0o600,父目录 0o700)。
// 保留供 MigrateFromLegacy 使用。
func (s *States) Save(path string) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal finding_states: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write finding_states %s: %w", path, err)
	}
	return nil
}

// Match 按 fingerprint 查找处置状态。命中返回 (state, true);nil 接收者或未命中返回 (zero, false)。
func (s *States) Match(fp string) (State, bool) {
	if s == nil {
		return State{}, false
	}
	for _, item := range s.Items {
		if item.Fingerprint == fp {
			return item, true
		}
	}
	return State{}, false
}

// Set 插入或更新(upsert)一条处置状态:同步写 db 和内存副本。
// db 为 nil 时仅写内存(兼容未注入 db 的旧调用路径)。
func (s *States) Set(fp string, st State) {
	st.Fingerprint = fp
	if st.UpdatedAt == "" {
		st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	// 写 db(nil-safe:db 为 nil 时跳过持久化,仅内存)
	if s.db != nil {
		_ = storage.UpsertFindingState(s.db, fp, string(st.Status), st.Priority, st.Note, string(st.Source), st.UpdatedAt)
	}
	// 同步内存副本
	for i := range s.Items {
		if s.Items[i].Fingerprint == fp {
			s.Items[i] = st
			return
		}
	}
	s.Items = append(s.Items, st)
}

// BulkAccept 批量将给定 fingerprint 标为 accepted(若当前为 open 或不存在)。
// 已有非 open 状态(resolved/false_positive/accepted/in_progress)不覆盖,尊重既有处置。
func (s *States) BulkAccept(fps []string, source Source, updatedAt string) {
	for _, fp := range fps {
		if existing, ok := s.Match(fp); ok && existing.Status != StatusOpen && existing.Status != "" {
			continue
		}
		s.Set(fp, State{Status: StatusAccepted, Source: source, UpdatedAt: updatedAt})
	}
}

// PruneReport 返回"已处置但本轮未检出"的孤儿状态(不删除原记录)。
// activeFps 是本轮扫描实际检出的 fingerprint 集合。
func (s *States) PruneReport(activeFps []string) []State {
	active := make(map[string]bool, len(activeFps))
	for _, fp := range activeFps {
		active[fp] = true
	}
	var orphans []State
	for _, item := range s.Items {
		if !active[item.Fingerprint] {
			orphans = append(orphans, item)
		}
	}
	return orphans
}

// Remove 删除一条处置状态:同步删 db 和内存。存在并删除返回 true,不存在返回 false。
// db 为 nil 时仅删内存。
func (s *States) Remove(fp string) bool {
	// 删 db(nil-safe)
	if s.db != nil {
		deleted, _ := storage.DeleteFindingState(s.db, fp)
		if !deleted {
			// db 中不存在,但内存仍可能匹配(如 db 为 nil),继续走内存删。
		}
	}
	// 删内存
	for i := range s.Items {
		if s.Items[i].Fingerprint == fp {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			return true
		}
	}
	return false
}
