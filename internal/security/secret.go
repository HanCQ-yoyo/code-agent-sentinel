package security

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"time"

	"code-agent-sentinel/internal/configengine"
)

type SecretDetector struct {
	binary string // gitleaks 路径或名
}

func NewSecretDetector(binary string) *SecretDetector {
	if binary == "" {
		binary = "gitleaks"
	}
	return &SecretDetector{binary: binary}
}

func (d *SecretDetector) ID() string                       { return "secret" }
func (d *SecretDetector) Covers() []configengine.AssetType { return nil } // 全部:喂源文件路径
func (d *SecretDetector) Available() bool                  { return commandExists(d.binary) }
func (d *SecretDetector) Reason() string {
	if d.Available() {
		return ""
	}
	return "gitleaks 未在 PATH 中找到(密钥扫描将跳过)"
}

type gitleaksFinding struct {
	RuleID    string `json:"RuleID"`
	Secret    string `json:"Secret"`
	File      string `json:"File"`
	StartLine int    `json:"StartLine"`
}

func (d *SecretDetector) Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error) {
	if !d.Available() {
		return nil, nil
	}
	// 收集要扫的源文件路径(去重)
	paths := map[string]configengine.Asset{}
	for _, a := range assets {
		if a.SourcePath != "" {
			paths[a.SourcePath] = a
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	// gitleaks detect --source <dir> --report-format json --report-path -
	// 为简化:扫每个文件所在目录,再用 File 字段回填资产
	var out []Finding
	for path, a := range paths {
		dir := filepath.Dir(path)
		r := runSubprocess(ctx, d.binary, []string{"detect", "--source", dir, "--report-format", "json", "--report-path", "-", "--no-banner"}, "", 60*time.Second)
		if r.TimedOut {
			continue
		}
		var gf []gitleaksFinding
		if err := json.Unmarshal(r.Stdout, &gf); err != nil {
			continue
		}
		for _, f := range gf {
			if filepath.Base(f.File) != filepath.Base(path) {
				continue
			}
			out = append(out, Finding{
				DetectorID:  d.ID(),
				RuleID:      f.RuleID,
				Severity:    SeverityHigh,
				AssetID:     a.ID,
				AssetType:   a.Type,
				AssetName:   a.Name,
				Message:     "检测到疑似密钥泄露",
				Evidence:    f.Secret,
				Remediation: "从文件中移除密钥,改用密钥管理服务",
			})
		}
	}
	return out, nil
}

// 占位避免 exec 未用 import 警告(实际 commandExists 用到 exec)
var _ = exec.ErrNotFound
