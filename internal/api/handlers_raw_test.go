package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func reqRaw(t *testing.T, s *Server, path string) (int, rawResponse) {
	t.Helper()
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/raw?path="+path, nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp rawResponse
	if w.Code == 200 {
		json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return w.Code, resp
}

// TestGetRawReadsFileUnderGlobalRoot 验证:全局根下的无资产文件可读原始内容。
func TestGetRawReadsFileUnderGlobalRoot(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{}`)
	// 一个无资产的文本文件(runtime 噪音,如 history.jsonl)
	writeFile(t, filepath.Join(claude, "history.jsonl"), `line1\nline2`)
	s := newTestServer(t, dir)
	code, resp := reqRaw(t, s, filepath.Join(claude, "history.jsonl"))
	if code != 200 {
		t.Fatalf("got %d", code)
	}
	if !resp.IsText {
		t.Errorf("history.jsonl 应为文本: %+v", resp)
	}
	if resp.Content == "" {
		t.Error("应返回文件内容")
	}
	if resp.Name != "history.jsonl" {
		t.Errorf("Name = %q", resp.Name)
	}
}

// TestGetRawRejectsOutOfRoot 验证:树根之外的路径被拒(防越权读 /etc/passwd)。
func TestGetRawRejectsOutOfRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{}`)
	s := newTestServer(t, dir)
	// /etc/passwd 不在任何树根之下。
	code, _ := reqRaw(t, s, "/etc/passwd")
	if code < 400 || code >= 500 {
		t.Errorf("树根外路径应 4xx, got %d", code)
	}
	// 路径穿越:../../../etc/passwd 也应拒。
	code, _ = reqRaw(t, s, filepath.Join(dir, ".claude", "..", "..", "etc", "passwd"))
	if code < 400 {
		t.Errorf("路径穿越应被拒, got %d", code)
	}
}

// TestGetRawRejectsDirAndMissing 验证:目录与不存在路径的处理。
func TestGetRawRejectsDirAndMissing(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{}`)
	s := newTestServer(t, dir)
	// 目录 → 400 is_dir。
	os.Mkdir(filepath.Join(claude, "sessions"), 0o755)
	code, _ := reqRaw(t, s, filepath.Join(claude, "sessions"))
	if code != 400 {
		t.Errorf("目录应 400, got %d", code)
	}
	// 不存在 → 404。
	code, _ = reqRaw(t, s, filepath.Join(claude, "nope.json"))
	if code != 404 {
		t.Errorf("不存在应 404, got %d", code)
	}
}

// TestGetRawReadsProjectRootFile 验证:项目根下的文件(如 .mcp.json)可读。
func TestGetRawReadsProjectRootFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{}`)
	proj := filepath.Join(dir, "proj")
	writeFile(t, filepath.Join(proj, ".claude", "settings.json"), `{}`)
	writeFile(t, filepath.Join(proj, ".mcp.json"), `{"mcpServers":{}}`)
	writeFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+proj+`":{}}}`)
	s := newTestServer(t, dir)
	code, resp := reqRaw(t, s, filepath.Join(proj, ".mcp.json"))
	if code != 200 {
		t.Fatalf("项目根 .mcp.json 应可读, got %d", code)
	}
	if resp.Content == "" {
		t.Error("应返回内容")
	}
}
