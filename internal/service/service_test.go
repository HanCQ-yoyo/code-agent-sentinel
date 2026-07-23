package service

import (
	"strings"
	"testing"
)

func TestGenerateUnitLinuxSystemd(t *testing.T) {
	spec := UnitSpec{
		OS:       "linux",
		UserMode: true,
		Home:     "/home/u",
		ExePath:  "/home/u/sentinel",
		Token:    "tok123",
		Bind:     "127.0.0.1",
		Port:     7777,
	}
	path, content, err := GenerateUnit(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, ".config/systemd/user/sentinel.service") {
		t.Errorf("linux user 路径错: %s", path)
	}
	if !strings.Contains(content, "ExecStart=/home/u/sentinel") {
		t.Errorf("ExecStart 缺: %s", content)
	}
	if !strings.Contains(content, "Restart=on-failure") {
		t.Error("缺 Restart=on-failure")
	}
	if !strings.Contains(content, "Environment=HOME=/home/u") {
		t.Error("缺 HOME env")
	}
}

func TestGenerateUnitMacOSLaunchd(t *testing.T) {
	spec := UnitSpec{OS: "darwin", UserMode: true, Home: "/Users/u", ExePath: "/Users/u/sentinel", Token: "t"}
	path, content, err := GenerateUnit(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "Library/LaunchAgents/com.claude-sentinel.sentinel.plist") {
		t.Errorf("macOS 路径错: %s", path)
	}
	if !strings.Contains(content, "<string>/Users/u/sentinel</string>") {
		t.Errorf("ProgramArguments 缺 exe: %s", content)
	}
	if !strings.Contains(content, "<true/>") { // KeepAlive/RunAtLoad
		t.Error("缺 KeepAlive/RunAtLoad")
	}
}

func TestGenerateUnitWindowsSC(t *testing.T) {
	spec := UnitSpec{OS: "windows", ExePath: `C:\sentinel.exe`, Token: "t"}
	_, content, err := GenerateUnit(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `sc.exe create`) || !strings.Contains(content, `C:\sentinel.exe`) {
		t.Errorf("Windows sc 命令错: %s", content)
	}
}

// TestServiceUnitLogPath 验证 UnitSpec.LogPath 被服务单元模板消费:
//   - launchd:LogPath 非空 → StandardOutPath/StandardErrorPath 指向该文件
//   - systemd:LogPath 非空 → StandardOutput=append:<path> + StandardError=append:<path>
//
// LogPath 空 → launchd 回退默认 sentinel.log,systemd 走 journal(无 StandardOutput 行)。
func TestServiceUnitLogPath(t *testing.T) {
	// launchd 带 LogPath
	launchdSpec := UnitSpec{
		OS:       "darwin",
		UserMode: true,
		Home:     "/Users/u",
		ExePath:  "/Users/u/sentinel",
		LogPath:  "/custom/sentinel.log",
	}
	_, launchdContent, err := GenerateUnit(launchdSpec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(launchdContent, "/custom/sentinel.log") {
		t.Errorf("launchd 单元应包含 LogPath: %s", launchdContent)
	}
	// 不应包含默认 sentinel.log(被自定义路径覆盖)
	if strings.Contains(launchdContent, ".claude-sentinel/sentinel.log") {
		t.Errorf("launchd 自定义 LogPath 应覆盖默认 sentinel.log: %s", launchdContent)
	}

	// systemd 带 LogPath
	systemdSpec := UnitSpec{
		OS:       "linux",
		UserMode: true,
		Home:     "/home/u",
		ExePath:  "/home/u/sentinel",
		LogPath:  "/var/log/sentinel.log",
	}
	_, systemdContent, err := GenerateUnit(systemdSpec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(systemdContent, "StandardOutput=append:/var/log/sentinel.log") {
		t.Errorf("systemd 单元应含 StandardOutput=append:<path>: %s", systemdContent)
	}
	if !strings.Contains(systemdContent, "StandardError=append:/var/log/sentinel.log") {
		t.Errorf("systemd 单元应含 StandardError=append:<path>: %s", systemdContent)
	}

	// LogPath 空:systemd 不应有 StandardOutput(走 journal 默认)
	systemdNoLog := UnitSpec{OS: "linux", UserMode: true, Home: "/home/u", ExePath: "/home/u/sentinel"}
	_, systemdNoContent, err := GenerateUnit(systemdNoLog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(systemdNoContent, "StandardOutput=") {
		t.Errorf("systemd 无 LogPath 不应有 StandardOutput 行(走 journal): %s", systemdNoContent)
	}

	// LogPath 空:launchd 回退默认 sentinel.log
	launchdNoLog := UnitSpec{OS: "darwin", UserMode: true, Home: "/Users/u", ExePath: "/Users/u/sentinel"}
	_, launchdNoContent, err := GenerateUnit(launchdNoLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(launchdNoContent, "/Users/u/.claude-sentinel/sentinel.log") {
		t.Errorf("launchd 空 LogPath 应回退默认 sentinel.log: %s", launchdNoContent)
	}
}
