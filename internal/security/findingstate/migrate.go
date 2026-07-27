package findingstate

import (
	"fmt"
	"os"
	"path/filepath"

	"code-agent-sentinel/internal/security/suppression"
)

// MigrateReport 记录迁移结果(可追溯)。
type MigrateReport struct {
	BaselineCount     int  // 从 baseline.json 迁移的 fingerprint 数
	InlineCount       int  // 从 suppressions.yaml 迁移的 fingerprint 项数
	GlobalRuleDropped int  // suppressions.yaml 的 rule_id 全局项数(不迁,提示到规则配置)
	Skipped           bool // finding_states.yaml 已存在,跳过迁移
}

// MigrateFromLegacy 将旧 baseline.json + suppressions.yaml 迁移为 finding_states.yaml。
//
// 规则:
//   - statesPath 已存在 → 跳过(不覆盖用户已有处置),返回 Skipped=true。
//   - baseline.json 每条 fingerprint → Status=accepted, Source=migrated-baseline。
//   - suppressions.yaml 每条带 fingerprint 的项 → Status=accepted, Source=migrated-inline, Note=Reason。
//   - suppressions.yaml 的 rule_id 全局项(无 fingerprint)→ 不迁移,计入 GlobalRuleDropped,
//     调用方应在日志/UI 提示"请到规则配置禁用规则 X"。
//   - 迁移后旧文件重命名为 *.legacy(不删,保留回滚)。
//   - 两个旧文件都不存在 → 空报告,不创建 statesPath。
func MigrateFromLegacy(baselinePath, suppressionsPath, statesPath string) (MigrateReport, error) {
	var rep MigrateReport

	// statesPath 已存在 → 跳过
	if _, err := os.Stat(statesPath); err == nil {
		rep.Skipped = true
		return rep, nil
	}

	states := &States{}

	// baseline.json
	if bs, err := suppression.LoadBaseline(baselinePath); err != nil {
		return rep, fmt.Errorf("load baseline: %w", err)
	} else if bs != nil {
		for fp := range bs.Fingerprints {
			states.Set(fp, State{Status: StatusAccepted, Source: SourceMigratedBaseline, Note: "迁移自 baseline"})
			rep.BaselineCount++
		}
	}

	// suppressions.yaml
	if sup, err := suppression.LoadSuppressions(suppressionsPath); err != nil {
		return rep, fmt.Errorf("load suppressions: %w", err)
	} else if sup != nil {
		for _, item := range sup.Items {
			if item.Fingerprint != "" {
				states.Set(item.Fingerprint, State{Status: StatusAccepted, Source: SourceMigratedInline, Note: item.Reason})
				rep.InlineCount++
			} else if item.RuleID != "" {
				// rule_id 全局项:不迁,提示禁用规则
				rep.GlobalRuleDropped++
			}
		}
	}

	// 无任何旧数据 → 不创建 statesPath
	if rep.BaselineCount == 0 && rep.InlineCount == 0 {
		return rep, nil
	}

	if err := states.Save(statesPath); err != nil {
		return rep, fmt.Errorf("save states: %w", err)
	}

	// 重命名旧文件为 .legacy
	for _, p := range []string{baselinePath, suppressionsPath} {
		if _, err := os.Stat(p); err == nil {
			if err := os.Rename(p, p+".legacy"); err != nil {
				return rep, fmt.Errorf("rename %s: %w", filepath.Base(p), err)
			}
		}
	}

	return rep, nil
}
