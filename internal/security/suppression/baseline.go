// Package suppression 实现误报抑制的两个正交机制:
//   - BaselineSet:已知指纹快照(baseline.json),跨运行稳定
//   - Suppressions:行内三档豁免(suppressions.yaml),fingerprint > rule+asset > rule 全局
//
// 本包刻意保持纯净:不 import security 包(会产生循环依赖——
// security 包的 RulesDetector 调用 applySuppression,后者引用本包类型)。
// Finding 的变异逻辑放在 security 包的 suppression_apply.go 里。
package suppression

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// BaselineSet 是一份已知指纹快照,用于 baseline 抑制。
// 指纹命中即认为该 finding 是已知的、可接受的,抑制标记为 "baseline"。
type BaselineSet struct {
	Version      string          `json:"version"`
	GeneratedAt  string          `json:"generated_at"`
	Fingerprints map[string]bool `json:"fingerprints"`
}

// LoadBaseline 从 JSON 文件加载 baseline。
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

// Save 将 baseline 以 JSON 写入指定路径(文件权限 0o600)。
// 父目录不存在时自动创建。
func (b *BaselineSet) Save(path string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write baseline %s: %w", path, err)
	}
	return nil
}

// Contains 判断指纹是否在 baseline 中。
// nil 接收者返回 false(无 baseline = 不抑制)。
func (b *BaselineSet) Contains(fp string) bool {
	if b == nil || b.Fingerprints == nil {
		return false
	}
	return b.Fingerprints[fp]
}
