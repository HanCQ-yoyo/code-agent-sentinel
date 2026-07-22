// Package service 生成 sentinel 的 OS 服务单元(linux systemd / macOS launchd / Windows sc)。
// 仅生成单元文件内容与落点路径;实际 install/uninstall 走 cmd/sentinel/service_cmd.go 调 systemctl/launchctl/sc.exe。
package service

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// UnitSpec 描述一个服务单元的生成参数。
type UnitSpec struct {
	OS       string // runtime.GOOS;空=用当前平台
	UserMode bool   // true=用户级(无需 root);false=系统级
	Home     string // 用户 home(Environment=HOME / 日志路径)
	ExePath  string // sentinel 二进制绝对路径(os.Executable())
	Token    string // 预置 token(写入 config,非进单元;此处仅记录)
	Bind     string // bind(默认 127.0.0.1)
	Port     int    // port
}

// GenerateUnit 返回 (落点路径, 单元文件内容, error)。
// windows 返回的 content 是待执行的 sc.exe 命令脚本(install 用),非常驻文件。
func GenerateUnit(spec UnitSpec) (string, string, error) {
	os := spec.OS
	if os == "" {
		os = runtime.GOOS
	}
	switch os {
	case "linux":
		return generateSystemd(spec)
	case "darwin":
		return generateLaunchd(spec)
	case "windows":
		return generateWindows(spec)
	}
	return "", "", fmt.Errorf("unsupported OS: %s", os)
}

func generateSystemd(spec UnitSpec) (string, string, error) {
	unitPath := filepath.Join(spec.Home, ".config", "systemd", "user", "sentinel.service")
	if !spec.UserMode {
		unitPath = "/etc/systemd/system/sentinel.service"
	}
	content := fmt.Sprintf(`[Unit]
Description=code-agent-sentinel
After=network.target

[Service]
ExecStart=%s
Restart=on-failure
Environment=HOME=%s

[Install]
WantedBy=default.target
`, spec.ExePath, spec.Home)
	return unitPath, content, nil
}

func generateLaunchd(spec UnitSpec) (string, string, error) {
	unitPath := filepath.Join(spec.Home, "Library", "LaunchAgents", "com.claude-sentinel.sentinel.plist")
	if !spec.UserMode {
		unitPath = "/Library/LaunchDaemons/com.claude-sentinel.sentinel.plist"
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.claude-sentinel.sentinel</string>
  <key>ProgramArguments</key>
  <array><string>%s</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s/.claude-sentinel/sentinel.log</string>
  <key>StandardErrorPath</key><string>%s/.claude-sentinel/sentinel.log</string>
</dict>
</plist>
`, spec.ExePath, spec.Home, spec.Home)
	return unitPath, content, nil
}

func generateWindows(spec UnitSpec) (string, string, error) {
	// windows 无单元文件,返回待执行的 sc 命令(install 时执行)。
	content := fmt.Sprintf(`sc.exe create sentinel binPath= "%s" start= auto
sc.exe start sentinel
`, spec.ExePath)
	return "", content, nil
}
