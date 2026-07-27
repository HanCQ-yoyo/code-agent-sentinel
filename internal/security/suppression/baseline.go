// Package suppression 是只读的旧格式解析器,仅供 findingstate.MigrateFromLegacy 迁移用。
//
// Task 11 删除了 suppression 写路径(suppression_apply.go + BaselineSet.Save/Contains +
// Suppressions.Save/Match)。本包降级为只读:仅保留 LoadBaseline / LoadSuppressions
// 读取旧 baseline.json / suppressions.yaml,供迁移逻辑把旧数据转成 finding_states.yaml。
// 迁移完成后旧文件被重命名为 .legacy,本包不再被运行时调用(仅迁移期一次性使用)。
//
// 本包刻意保持纯净:不 import security 包(会产生循环依赖)。
package suppression

import (
	"encoding/json"
	"fmt"
	"os"
)

// BaselineSet 是一份已知指纹快照(旧 baseline.json 格式,只读)。
// 迁移逻辑读取 Fingerprints 转 findingstate.State(Status=accepted, Source=migrated-baseline)。
type BaselineSet struct {
	Version      string          `json:"version"`
	GeneratedAt  string          `json:"generated_at"`
	Fingerprints map[string]bool `json:"fingerprints"`
}

// LoadBaseline 从 JSON 文件加载 baseline(只读,迁移用)。
// 文件不存在时返回 (nil, nil)(用户尚未生成 baseline,非错误)。
func LoadBaseline(path string) (*BaselineSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read baseline %s: %w", path, err)
	}
	var bs BaselineSet
	if err := json.Unmarshal(data, &bs); err != nil {
		return nil, fmt.Errorf("parse baseline %s: %w", path, err)
	}
	return &bs, nil
}
