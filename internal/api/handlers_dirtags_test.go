package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
)

func reqDirTags(t *testing.T, s *Server, method, body string) (int, dirTagsResponse) {
	t.Helper()
	r := s.Router()
	var req = httptest.NewRequest(method, "/api/dir-tags", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp dirTagsResponse
	if w.Code == 200 {
		json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return w.Code, resp
}

// TestGetDirTagsReturnsDefaults 验证 GET 返回默认标签 + 空(或 nil)覆盖。
func TestGetDirTagsReturnsDefaults(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	code, resp := reqDirTags(t, s, "GET", "")
	if code != 200 {
		t.Fatalf("got %d", code)
	}
	if resp.Defaults["sessions"] != config.TagRuntime {
		t.Errorf("默认应含 sessions=runtime: %v", resp.Defaults)
	}
	if resp.Defaults["settings.json"] != config.TagConfig {
		t.Errorf("默认应含 settings.json=config: %v", resp.Defaults)
	}
}

// TestPutDirTagsPersistsToFile 验证 PUT 写入覆盖并持久化到配置文件,
// 重新 Load 后覆盖仍在。
func TestPutDirTagsPersistsToFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".claude-sentinel", "config.yaml")
	s := newTestServer(t, dir)
	s.ConfigPath = cfgPath

	body := `{"overrides":{"sessions":"config","foo":"runtime"}}`
	code, resp := reqDirTags(t, s, "PUT", body)
	if code != 200 {
		t.Fatalf("got %d", code)
	}
	if resp.Overrides["sessions"] != "config" {
		t.Errorf("覆盖未返回: %v", resp.Overrides)
	}
	// 持久化:重新 Load 配置文件应见覆盖。
	c2, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if c2.DirTags["sessions"] != "config" || c2.DirTags["foo"] != "runtime" {
		t.Errorf("持久化丢失: %v", c2.DirTags)
	}
}

// TestPutDirTagsRejectsBadTag 验证未知标签值被拒(防污染配置)。
func TestPutDirTagsRejectsBadTag(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	code, _ := reqDirTags(t, s, "PUT", `{"overrides":{"x":"evil"}}`)
	if code != 400 {
		t.Errorf("未知标签应 400, got %d", code)
	}
}

// TestPutDirTagsReplacesNotMerges 验证整体替换语义:第二次 PUT 不含的 key 被移除。
func TestPutDirTagsReplacesNotMerges(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".claude-sentinel", "config.yaml")
	s := newTestServer(t, dir)
	s.ConfigPath = cfgPath

	reqDirTags(t, s, "PUT", `{"overrides":{"sessions":"config","foo":"runtime"}}`)
	// 第二次只留 foo:sessions 应消失。
	code, resp := reqDirTags(t, s, "PUT", `{"overrides":{"foo":"runtime"}}`)
	if code != 200 {
		t.Fatalf("got %d", code)
	}
	if _, exists := resp.Overrides["sessions"]; exists {
		t.Errorf("整体替换:sessions 应被移除, got %v", resp.Overrides)
	}
	if resp.Overrides["foo"] != "runtime" {
		t.Errorf("foo 应保留: %v", resp.Overrides)
	}
}
