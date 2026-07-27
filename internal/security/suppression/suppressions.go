package suppression

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Suppressions 是行内豁免规则集合(旧 suppressions.yaml 格式,只读)。
// 迁移逻辑读取 Items 转 findingstate.State(fingerprint 项 → Status=accepted, Source=migrated-inline)。
type Suppressions struct {
	Items []Item `yaml:"items"`
}

// Item 是一条旧格式豁免规则(只读,迁移用)。
// 三档由字段填充情况决定:
//   - Fingerprint 非空 → 指纹档(迁移为 accepted)
//   - RuleID + AssetID 非空 → rule+asset 档(迁移为 accepted,以 RuleID+AssetID 合成键)
//   - 仅 RuleID 非空 → rule 全局档(不迁移,计入 GlobalRuleDropped 提示禁用规则)
type Item struct {
	Fingerprint string `yaml:"fingerprint"`
	RuleID      string `yaml:"rule_id"`
	AssetID     string `yaml:"asset_id"`
	Reason      string `yaml:"reason"`
}

// LoadSuppressions 从 YAML 文件加载豁免规则(只读,迁移用)。
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
