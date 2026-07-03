package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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
			// 偏差(brief 原文静默吞掉 npm 扫描器错误):
			// npm audit 退出码 0=无漏洞、1=有漏洞,均正常;其它退出码或非 ExitError 的 Err
			// 表示 npm 自身故障(.npmrc 配置错误、网络失败、lock 文件损坏等)。此时不能静默
			// continue,否则只读看板显示零发现而掩盖扫描器问题。镜像 Task 14 的
			// secret.scanner-error 修复;经人工确认同意偏离。
			scannerFailed := r.ExitCode != 0 && r.ExitCode != 1
			if r.Err != nil {
				if _, ok := r.Err.(*exec.ExitError); !ok {
					scannerFailed = true // 非 ExitError:二进制启动失败等
				}
			}
			if scannerFailed {
				out = append(out, Finding{
					DetectorID:  d.ID(),
					RuleID:      "dep.scanner-error",
					Severity:    SeverityLow,
					AssetID:     a.ID,
					AssetType:   a.Type,
					AssetName:   a.Name,
					Message:     "npm audit 扫描失败",
					Evidence:    scannerEvidence(r.Stderr, r.Err, r.ExitCode),
					Remediation: "检查 npm 配置/网络后重试",
				})
				continue
			}
			var aud npmAudit
			if err := json.Unmarshal(r.Stdout, &aud); err != nil {
				// exit 0 + 空 stdout 属正常(无漏洞);非空 stdout 解析失败 = 扫描器输出异常
				if len(r.Stdout) > 0 {
					out = append(out, Finding{
						DetectorID:  d.ID(),
						RuleID:      "dep.scanner-error",
						Severity:    SeverityLow,
						AssetID:     a.ID,
						AssetType:   a.Type,
						AssetName:   a.Name,
						Message:     "npm audit 输出解析失败",
						Evidence:    truncate(string(r.Stdout), 200),
						Remediation: "检查 npm 版本与输出格式",
					})
				}
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
			if r.TimedOut {
				// 偏差(brief 原文缺此守卫):120s 超时会截断 NDJSON,解析不完整。
				// 镜像 Task 14 + npm 分支的一致性处理;经人工确认同意偏离。
				continue
			}
			// 偏差(brief 原文静默吞掉 govulncheck 扫描器错误):
			// govulncheck 退出码 0=无漏洞、1=有漏洞,均正常;其它退出码或非 ExitError 的 Err
			// 表示 govulncheck 自身故障(安装损坏、go.mod 无效、崩溃等)。此时不能静默 continue,
			// 否则只读看板显示零发现而掩盖扫描器问题。镜像 Task 14 的 secret.scanner-error 修复;
			// 经人工确认同意偏离。
			scannerFailed := r.ExitCode != 0 && r.ExitCode != 1
			if r.Err != nil {
				if _, ok := r.Err.(*exec.ExitError); !ok {
					scannerFailed = true // 非 ExitError:二进制启动失败等
				}
			}
			if scannerFailed {
				out = append(out, Finding{
					DetectorID:  d.ID(),
					RuleID:      "dep.scanner-error",
					Severity:    SeverityLow,
					AssetID:     a.ID,
					AssetType:   a.Type,
					AssetName:   a.Name,
					Message:     "govulncheck 扫描失败",
					Evidence:    scannerEvidence(r.Stderr, r.Err, r.ExitCode),
					Remediation: "检查 govulncheck 安装与 go.mod 后重试",
				})
				continue
			}
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

// scannerEvidence 生成扫描器错误的证据字符串:优先 stderr,其次 Err,最后退回退出码。
func scannerEvidence(stderr []byte, err error, exitCode int) string {
	if len(stderr) > 0 {
		return truncate(string(stderr), 200)
	}
	if err != nil {
		return truncate(err.Error(), 200)
	}
	return fmt.Sprintf("exit code %d", exitCode)
}

// 偏差(brief 的 parseGovulncheck 引用了未定义的接收者 d 并附带无用的 IDName() 垫片):
// 将检测器 ID 作为参数传入,删除 IDName() 垫片。经人工确认同意偏离。
func parseGovulncheck(detectorID string, stdout []byte, a configengine.Asset) []Finding {
	// C-CORR-1: govulncheck -json 输出多行 NDJSON,4 类记录:
	//   {"config":{...}}         —— 扫描配置,跳过
	//   {"osv":{...object...}}   —— 完整 OSV 对象(id 在 .id),可交叉引用,跳过
	//   {"finding":{"osv":"GO-...",...}}  —— 一个漏洞命中,产一条 finding
	//   {"progress":{...}}       —— 进度,跳过
	// 旧解析器期望顶层 {"osv":"<string>"}(不存在)→ 0 finding,govulncheck 后端
	// 实为死代码。新解析器按 finding 记录的 .osv 字段产 finding。
	var out []Finding
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var obj struct {
			// 只取 finding 记录;config/osv-object/progress 记录此字段为空,自然跳过。
			// (Trace/模块信息留待后续增强;P1 只需 OSV ID)
			Finding struct {
				OSV string `json:"osv"`
			} `json:"finding"`
		}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		// 只处理 finding 记录;config/osv-object/progress 记录的 finding 字段为空,跳过。
		if obj.Finding.OSV == "" {
			continue
		}
		osvID := obj.Finding.OSV
		out = append(out, Finding{
			DetectorID:  detectorID,
			RuleID:      "dep.govulncheck." + osvID,
			Severity:    SeverityHigh, // P1 固定 High;精确 severity 交叉引用 OSV 对象属后续优化
			AssetID:     a.ID,
			AssetType:   a.Type,
			AssetName:   a.Name,
			Message:     "Go 漏洞: " + osvID,
			Remediation: "升级依赖修复 " + osvID,
		})
	}
	return out
}
