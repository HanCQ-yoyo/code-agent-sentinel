package config

import "testing"

func TestResolveDirTagOverrideWins(t *testing.T) {
	// 用户把 sessions 从默认 runtime 改成 config。
	ov := DirTags{"sessions": TagConfig}
	if got := ResolveDirTag("sessions", ov); got != TagConfig {
		t.Errorf("override 应胜过默认: got %q, want %q", got, TagConfig)
	}
}

func TestResolveDirTagDefaultWhenNoOverride(t *testing.T) {
	// 无覆盖时取默认:sessions = runtime。
	if got := ResolveDirTag("sessions", nil); got != TagRuntime {
		t.Errorf("默认 sessions 应 runtime: got %q", got)
	}
	if got := ResolveDirTag("settings.json", nil); got != TagConfig {
		t.Errorf("默认 settings.json 应 config: got %q", got)
	}
}

func TestResolveDirTagInheritanceByLongestPrefix(t *testing.T) {
	// sessions 子文件继承 sessions 的 runtime 标签(最长前缀匹配)。
	if got := ResolveDirTag("sessions/3000363.json", nil); got != TagRuntime {
		t.Errorf("sessions 子文件应继承 runtime: got %q", got)
	}
	// plugins 顶层默认 config,但 plugins/data 默认 runtime:更具体的前缀胜出。
	if got := ResolveDirTag("plugins", nil); got != TagConfig {
		t.Errorf("plugins 顶层应 config: got %q", got)
	}
	if got := ResolveDirTag("plugins/data/x.json", nil); got != TagRuntime {
		t.Errorf("plugins/data 子文件应 runtime(更具体前缀): got %q", got)
	}
}

func TestResolveDirTagSegmentBoundary(t *testing.T) {
	// "sessions" 不得匹配 "sessions-archive"(非路径段前缀)。
	if got := ResolveDirTag("sessions-archive/x", nil); got != "" {
		t.Errorf("sessions-archive 不应匹配 sessions 标签: got %q", got)
	}
	// 未登记路径 → untagged。
	if got := ResolveDirTag("some-random-dir/x", nil); got != "" {
		t.Errorf("未登记路径应 untagged: got %q", got)
	}
}

func TestSegHasPrefix(t *testing.T) {
	cases := []struct {
		path, prefix string
		want         bool
	}{
		{"sessions", "sessions", true},
		{"sessions/x", "sessions", true},
		{"sessions/x/y", "sessions", true},
		{"sessions-foo", "sessions", false},
		{"sessionsfoo", "sessions", false},
		{"", "", true},
		{"x", "", true},
		{"plugins/data", "plugins", true},
		{"plugins-data", "plugins", false},
	}
	for _, c := range cases {
		if got := segHasPrefix(c.path, c.prefix); got != c.want {
			t.Errorf("segHasPrefix(%q,%q) = %v, want %v", c.path, c.prefix, got, c.want)
		}
	}
}

// TestConfigRoundTripDirTags 验证 DirTags 字段经 yaml 存取保留。
func TestConfigRoundTripDirTags(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/config.yaml"
	c := DefaultConfig()
	c.DirTags = DirTags{"sessions": "config", "foo": "runtime"}
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c2.DirTags["sessions"] != "config" || c2.DirTags["foo"] != "runtime" {
		t.Errorf("DirTags 往返丢失: %v", c2.DirTags)
	}
}
