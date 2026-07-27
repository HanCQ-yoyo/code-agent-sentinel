// 本文件放在 security 包(而非 findingstate 子包),因为 applyFindingState 需要修改
// *Finding 字段,而 Finding 定义在 security 包。若放在 findingstate 包会产生循环依赖
// (findingstate → security → findingstate)。与原 suppression_apply.go 同构。
package security

import "code-agent-sentinel/internal/security/findingstate"

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
