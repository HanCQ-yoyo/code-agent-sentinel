package security

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"code-agent-sentinel/internal/configengine"
)

type DependencyDetector struct {
	npmBin      string
	govulncheck string
}

func NewDependencyDetector(npmBin, govulncheck string) *DependencyDetector {
	if npmBin == "" {
		npmBin = "npm"
	}
	if govulncheck == "" {
		govulncheck = "govulncheck"
	}
	return &DependencyDetector{npmBin: npmBin, govulncheck: govulncheck}
}

func (d *DependencyDetector) ID() string { return "dep" }
func (d *DependencyDetector) Covers() []configengine.AssetType {
	return []configengine.AssetType{configengine.AssetScript, configengine.AssetPlugin, configengine.AssetSkill, configengine.AssetCommand}
}
func (d *DependencyDetector) Available() bool {
	return commandExists(d.npmBin) || commandExists(d.govulncheck)
}
func (d *DependencyDetector) Reason() string {
	if d.Available() {
		return ""
	}
	return "npm 与 govulncheck 均未找到(依赖扫描将跳过)"
}

type npmAudit struct {
	Vulnerabilities map[string]struct {
		Severity string `json:"severity"`
	} `json:"vulnerabilities"`
}

func (d *DependencyDetector) Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error) {
	if !d.Available() {
		return nil, nil
	}
	var out []Finding
	scanned := map[string]bool{}
	for _, a := range assets {
		dir := d.auditDir(a)
		if dir == "" || scanned[dir] {
			continue
		}
		scanned[dir] = true
		if commandExists(d.npmBin) && fileExists(filepath.Join(dir, "package.json")) {
			r := runSubprocess(ctx, d.npmBin, []string{"audit", "--json"}, dir, 60*time.Second)
			if r.TimedOut {
				continue
			}
			var aud npmAudit
			if err := json.Unmarshal(r.Stdout, &aud); err != nil {
				continue
			}
			for pkg, v := range aud.Vulnerabilities {
				out = append(out, Finding{
					DetectorID:  d.ID(),
					RuleID:      "dep.npm." + pkg,
					Severity:    toSeverity(v.Severity),
					AssetID:     a.ID,
					AssetType:   a.Type,
					AssetName:   a.Name,
					Message:     "依赖漏洞: " + pkg,
					Evidence:    "npm audit severity=" + v.Severity,
					Remediation: "npm audit fix 或升级 " + pkg,
				})
			}
		}
		if commandExists(d.govulncheck) && hasGoMod(dir) {
			r := runSubprocess(ctx, d.govulncheck, []string{"-json", "./..."}, dir, 120*time.Second)
			out = append(out, parseGovulncheck(d.ID(), r.Stdout, a)...)
		}
	}
	return out, nil
}

func (d *DependencyDetector) auditDir(a configengine.Asset) string {
	if a.SourcePath == "" {
		return ""
	}
	if isDir(a.SourcePath) {
		return a.SourcePath
	}
	return filepath.Dir(a.SourcePath)
}

func toSeverity(s string) Severity {
	switch strings.ToLower(s) {
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "moderate", "medium":
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// 偏差(brief 的 parseGovulncheck 引用了未定义的接收者 d 并附带无用的 IDName() 垫片):
// 将检测器 ID 作为参数传入,删除 IDName() 垫片。经人工确认同意偏离。
func parseGovulncheck(detectorID string, stdout []byte, a configengine.Asset) []Finding {
	// govulncheck -json 输出多行 JSON;P1 简化:按行解析 finding 对象
	var out []Finding
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var obj struct {
			OSV      string `json:"osv"`
			Severity string `json:"severity"`
		}
		if json.Unmarshal([]byte(line), &obj) == nil && obj.OSV != "" {
			out = append(out, Finding{
				DetectorID:  detectorID,
				RuleID:      "dep.govulncheck." + obj.OSV,
				Severity:    SeverityHigh,
				AssetID:     a.ID,
				AssetType:   a.Type,
				AssetName:   a.Name,
				Message:     "Go 漏洞: " + obj.OSV,
				Remediation: "升级依赖修复 " + obj.OSV,
			})
		}
	}
	return out
}
