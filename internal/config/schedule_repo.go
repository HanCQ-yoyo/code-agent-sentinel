package config

import (
	"time"

	"code-agent-sentinel/internal/storage"
)

// ScheduleRepo 管理定时扫描调度,持久化到 sqlite schedules 表。
// 替代 config.yaml 中的 schedules 列表。
type ScheduleRepo struct {
	db *storage.DB
}

func NewScheduleRepo(db *storage.DB) *ScheduleRepo {
	return &ScheduleRepo{db: db}
}

// List 返回全部调度配置。db 为 nil 返回空列表。
func (r *ScheduleRepo) List() ([]ScheduleCfg, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := storage.ListSchedules(r.db)
	if err != nil {
		return nil, err
	}
	out := make([]ScheduleCfg, 0, len(rows))
	for _, row := range rows {
		out = append(out, ScheduleCfg{
			AgentID:  row.AgentID,
			Enabled:  row.Enabled,
			Interval: row.Interval,
		})
	}
	return out, nil
}

// Upsert 插入或更新一条调度。db 为 nil 静默成功。
func (r *ScheduleRepo) Upsert(agentID string, enabled bool, interval string) error {
	if r.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	return storage.UpsertSchedule(r.db, agentID, enabledInt, interval, now)
}

// Delete 删除指定 agent 的调度。db 为 nil 静默成功。
func (r *ScheduleRepo) Delete(agentID string) error {
	if r.db == nil {
		return nil
	}
	return storage.DeleteSchedule(r.db, agentID)
}
