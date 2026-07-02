package security

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
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
		// 偏差(brief 原文静默吞掉 stderr/exit code):
		// gitleaks 退出码 0=无泄露、1=有泄露,均正常;其它退出码或非 ExitError 的 Err
		// 表示 gitleaks 自身故障(配置错误、--source 无效、崩溃等)。此时不能静默 continue,
		// 否则只读看板显示零发现而掩盖扫描器问题。经人工确认同意偏离。
		scannerFailed := r.ExitCode != 0 && r.ExitCode != 1
		if r.Err != nil {
			if _, ok := r.Err.(*exec.ExitError); !ok {
				scannerFailed = true // 非 ExitError:二进制启动失败等
			}
		}
		if scannerFailed {
			evidence := truncate(string(r.Stderr), 200)
			if evidence == "" && r.Err != nil {
				evidence = truncate(r.Err.Error(), 200)
			}
			out = append(out, Finding{
				DetectorID:  d.ID(),
				RuleID:      "secret.scanner-error",
				Severity:    SeverityLow,
				AssetID:     a.ID,
				AssetType:   a.Type,
				AssetName:   a.Name,
				Message:     "gitleaks 扫描器异常(退出码 " + strconv.Itoa(r.ExitCode) + ")",
				Evidence:    evidence,
				Remediation: "检查 gitleaks 配置与运行环境",
			})
			continue
		}
		var gf []gitleaksFinding
		if err := json.Unmarshal(r.Stdout, &gf); err != nil {
			// exit 0 + 空 stdout 属正常(无泄露);非空 stdout 解析失败 = 扫描器输出异常
			if len(r.Stdout) > 0 {
				out = append(out, Finding{
					DetectorID:  d.ID(),
					RuleID:      "secret.scanner-error",
					Severity:    SeverityLow,
					AssetID:     a.ID,
					AssetType:   a.Type,
					AssetName:   a.Name,
					Message:     "gitleaks 输出解析失败",
					Evidence:    truncate(string(r.Stdout), 200),
					Remediation: "检查 gitleaks 版本与输出格式",
				})
			}
			continue
		}
		for _, f := range gf {
			// 偏差(brief 原文用 basename 匹配):
			// gitleaks detect --source <dir> 递归扫描,File 字段是相对 --source 的路径。
			// 不同子目录下同名文件会让 basename 误匹配,把别的文件的泄露归到本资产。
			// 改为完整路径比对:用 filepath.Join(dir, f.File) 重构绝对路径,清洗后与
			// 资产 SourcePath 比较。若 gitleaks 返回绝对路径则直接使用。经人工确认同意偏离。
			var findingPath string
			if filepath.IsAbs(f.File) {
				findingPath = filepath.Clean(f.File)
			} else {
				findingPath = filepath.Clean(filepath.Join(dir, f.File))
			}
			if findingPath != filepath.Clean(path) {
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
