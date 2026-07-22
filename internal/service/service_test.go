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
