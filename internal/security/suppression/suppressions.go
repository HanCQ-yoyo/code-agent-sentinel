package suppression

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Suppressions 是行内豁免规则集合(suppressions.yaml)。
// 通过 Match 方法按三档优先级匹配:fingerprint > rule_id+asset_id > rule_id。
type Suppressions struct {
	Items []Item `yaml:"items"`
}

// Item 是一条豁免规则。三档由字段填充情况决定:
//   - Fingerprint 非空 → 指纹档(最高优先级)
//   - RuleID + AssetID 非空 → rule+asset 档
//   - 仅 RuleID 非空 → rule 全局档(最低优先级,匹配任意 asset)
type Item struct {
	Fingerprint string `yaml:"fingerprint"`
	RuleID      string `yaml:"rule_id"`
	AssetID     string `yaml:"asset_id"`
	Reason      string `yaml:"reason"`
}

// LoadSuppressions 从 YAML 文件加载豁免规则。
// 文件不存在时返回 (nil, nil)(用户尚未创建豁免文件,非错误)。
func LoadSuppressions(path string) (*Suppressions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read suppressions %s: %w", path, err)
	}
	var s Suppressions
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse suppressions %s: %w", path, err)
	}
	return &s, nil
}

// Save 将豁免规则以 YAML 写入指定路径(文件权限 0o600)。
// 父目录不存在时自动创建。
func (s *Suppressions) Save(path string) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal suppressions: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write suppressions %s: %w", path, err)
	}
	return nil
}

// Match 按三档优先级检查是否命中豁免:
//  1. 指纹档(精确匹配 Fingerprint)
//  2. rule_id + asset_id 档(两者均精确匹配)
//  3. rule_id 全局档(仅 RuleID 精确匹配,任意 asset)
//
// 每档内首次命中即返回其 Reason。nil 接收者返回 (false, "")。
func (s *Suppressions) Match(ruleID, assetID, fp string) (suppressed bool, reason string) {
	if s == nil {
		return false, ""
	}

	// 档 1:指纹(精确)
	for _, item := range s.Items {
		if item.Fingerprint != "" && item.Fingerprint == fp {
			return true, item.Reason
		}
	}

	// 档 2:rule_id + asset_id(均精确)
	for _, item := range s.Items {
		if item.Fingerprint == "" && item.RuleID != "" && item.RuleID == ruleID &&
			item.AssetID != "" && item.AssetID == assetID {
			return true, item.Reason
		}
	}

	// 档 3:rule_id 全局(任意 asset)
	for _, item := range s.Items {
		if item.Fingerprint == "" && item.AssetID == "" && item.RuleID != "" && item.RuleID == ruleID {
			return true, item.Reason
		}
	}

	return false, ""
}
