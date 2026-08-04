// 本文件放在 security 包(而非 findingstate 子包),因为 applyFindingState 需要修改
// *Finding 字段,而 Finding 定义在 security 包。若放在 findingstate 包会产生循环依赖
// (findingstate → security → findingstate)。与原 suppression_apply.go 同构。
package security

import (
	"time"

	"code-agent-sentinel/internal/security/findingstate"
)

// applyFindingState 将处置生命周期状态合并到单条 Finding。
//
// 命中处置状态:
//   - resolved/false_positive/accepted → Suppressed=true, Suppression="state"
//     (健康分权重 0,不再扣分;但 finding 仍保留在结果中,可查可还原)
//   - in_progress → Suppressed=false(仍可见), Status="in_progress"(健康分 0.5)
//   - open → Suppressed=false, Status="open"(全额扣分)
//
// 未命中(states nil 或无记录)→ Status="open", Suppressed=false。
func applyFindingState(f *Finding, fp string, states *findingstate.States) {
	f.Status = string(findingstate.StatusOpen) // 默认 open
	if states == nil {
		return
	}
	st, ok := states.Match(fp)
	if !ok {
		return
	}
	f.Status = string(st.Status)
	if st.Priority != "" {
		f.Priority = st.Priority
	}
	if st.Note != "" {
		f.Note = st.Note
		f.Reason = st.Note // 兼容旧 Reason 消费方
	}
	switch st.Status {
	case findingstate.StatusResolved, findingstate.StatusFalsePositive, findingstate.StatusAccepted:
		f.Suppressed = true
		f.Suppression = "state"
	case findingstate.StatusInProgress:
		// 仍可见,不设 Suppressed;健康分靠 Status 取 0.5
	}
}

// ApplyFindingStateBatch 对一批 finding 做读路径统一处理:
//  1. 调 applyFindingState 把处置生命周期状态合并进每条 finding(含 resolved/false_positive → Suppressed)。
//  2. 附 StartedAt(来自所属 ScanRecord)。
//  3. 附 SourcePath(通过 assetSourcePath 回调按 AssetID 查 ScanRecord.Inventory 快照)。
//
// 与 rules_detector.go 扫描路径用同一 applyFindingState,保证读路径与扫描路径语义一致。
// states 为 nil 安全(applyFindingState 内部降级 Status="open")。assetSourcePath 为 nil 时跳过 SourcePath。
// 用于 API 读路径(/api/findings),使处置后列表立即反映已抑制状态,无需重扫。
func ApplyFindingStateBatch(findings []Finding, states *findingstate.States, startedAt time.Time, assetSourcePath func(assetID string) string) {
	for i := range findings {
		f := &findings[i]
		applyFindingState(f, f.Fingerprint, states)
		f.StartedAt = startedAt
		if assetSourcePath != nil {
			f.SourcePath = assetSourcePath(f.AssetID)
		}
	}
}
