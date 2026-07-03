package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/security"
)

func TestPostScan(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"permissions":{"allow":["Bash(*)"]}}`)
	s := newTestServer(t, dir)
	r := s.Router()
	req := httptest.NewRequest("POST", "/api/scan", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var res security.ScanResult
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Findings) == 0 {
		t.Error("应检出通配 Bash")
	}
	if res.HealthScore == nil {
		t.Error("应返回健康分")
	}
}

func TestGetScanResultEmpty(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/scan/result", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}
