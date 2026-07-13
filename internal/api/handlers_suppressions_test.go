package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/security/suppression"
)

// reqSuppression 发送 /api/suppressions 或 /api/baseline 请求,返回状态码 + 响应体。
func reqSuppression(t *testing.T, s *Server, method, path, body string) (int, []byte) {
	t.Helper()
	r := s.Router()
	var req = httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// TestAPIAddSuppression 验证 POST 添加豁免 → 写入文件 → GET 返回含该条目。
func TestAPIAddSuppression(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	// 隔离:指向临时目录,不污染真实 ~/.claude-sentinel/
	suppressPath := filepath.Join(dir, "suppressions.yaml")
	s.Config.SuppressPath = suppressPath

	body := `{"fingerprint":"fp1","reason":"known safe"}`
	code, respBody := reqSuppression(t, s, "POST", "/api/suppressions", body)
	if code != 200 {
		t.Fatalf("POST got %d: %s", code, respBody)
	}

	// 验证文件写入
	data, err := os.ReadFile(suppressPath)
	if err != nil {
		t.Fatalf("读 suppressions.yaml: %v", err)
	}
	if !strings.Contains(string(data), "fp1") {
		t.Errorf("文件不含 fp1: %s", data)
	}

	// GET 返回含 fp1
	code, respBody = reqSuppression(t, s, "GET", "/api/suppressions", "")
	if code != 200 {
		t.Fatalf("GET got %d: %s", code, respBody)
	}
	var resp suppressionsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Fingerprint != "fp1" {
		t.Errorf("GET 返回不符: %+v", resp.Items)
	}
	if resp.Items[0].ID != "fp:fp1" {
		t.Errorf("ID = %q, 期望 fp:fp1", resp.Items[0].ID)
	}
}

// TestAPIAddSuppressionRuleTier 验证 rule+asset 档和 rule 全局档的添加与 ID 计算。
func TestAPIAddSuppressionRuleTier(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.Config.SuppressPath = filepath.Join(dir, "suppressions.yaml")

	// rule+asset 档
	body := `{"rule_id":"baseline.wildcard-bash","asset_id":"perm-1","reason":"known"}`
	code, _ := reqSuppression(t, s, "POST", "/api/suppressions", body)
	if code != 200 {
		t.Fatalf("POST rule+asset got %d", code)
	}

	// rule 全局档
	body = `{"rule_id":"baseline.dangerous-skip-permission","reason":"global accept"}`
	code, _ = reqSuppression(t, s, "POST", "/api/suppressions", body)
	if code != 200 {
		t.Fatalf("POST rule-global got %d", code)
	}

	// GET 返回两条,ID 分别为 ra: 和 rg: 前缀
	code, respBody := reqSuppression(t, s, "GET", "/api/suppressions", "")
	if code != 200 {
		t.Fatalf("GET got %d", code)
	}
	var resp suppressionsResponse
	json.Unmarshal(respBody, &resp)
	if len(resp.Items) != 2 {
		t.Fatalf("期望 2 条, got %d", len(resp.Items))
	}
	hasRA := false
	hasRG := false
	for _, item := range resp.Items {
		if strings.HasPrefix(item.ID, "ra:") {
			hasRA = true
		}
		if strings.HasPrefix(item.ID, "rg:") {
			hasRG = true
		}
	}
	if !hasRA {
		t.Error("缺少 ra: 前缀 ID")
	}
	if !hasRG {
		t.Error("缺少 rg: 前缀 ID")
	}
}

// TestAPIAddSuppressionValidation 验证空请求体(fingerprint 和 rule_id 均空)被拒。
func TestAPIAddSuppressionValidation(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.Config.SuppressPath = filepath.Join(dir, "suppressions.yaml")

	code, _ := reqSuppression(t, s, "POST", "/api/suppressions", `{"reason":"no fp or rule"}`)
	if code != 400 {
		t.Errorf("空 fingerprint+rule_id 应 400, got %d", code)
	}
}

// TestAPIDeleteSuppression 验证 POST 添加 → DELETE 删除 → GET 返回空。
func TestAPIDeleteSuppression(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.Config.SuppressPath = filepath.Join(dir, "suppressions.yaml")

	// 添加一条
	body := `{"fingerprint":"fp-delete-me","reason":"temp"}`
	code, _ := reqSuppression(t, s, "POST", "/api/suppressions", body)
	if code != 200 {
		t.Fatalf("POST got %d", code)
	}

	// DELETE
	code, _ = reqSuppression(t, s, "DELETE", "/api/suppressions/fp:fp-delete-me", "")
	if code != 200 {
		t.Fatalf("DELETE got %d", code)
	}

	// GET 返回空
	code, respBody := reqSuppression(t, s, "GET", "/api/suppressions", "")
	if code != 200 {
		t.Fatalf("GET got %d", code)
	}
	var resp suppressionsResponse
	json.Unmarshal(respBody, &resp)
	if len(resp.Items) != 0 {
		t.Errorf("删除后应空, got %d items", len(resp.Items))
	}
}

// TestAPIDeleteSuppressionRuleTier 验证按 rule+asset 档 ID 删除。
func TestAPIDeleteSuppressionRuleTier(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.Config.SuppressPath = filepath.Join(dir, "suppressions.yaml")

	body := `{"rule_id":"baseline.wildcard-bash","asset_id":"perm-1","reason":"known"}`
	code, _ := reqSuppression(t, s, "POST", "/api/suppressions", body)
	if code != 200 {
		t.Fatalf("POST got %d", code)
	}

	// DELETE ra:baseline.wildcard-bash:perm-1
	code, _ = reqSuppression(t, s, "DELETE", "/api/suppressions/ra:baseline.wildcard-bash:perm-1", "")
	if code != 200 {
		t.Fatalf("DELETE got %d", code)
	}

	code, respBody := reqSuppression(t, s, "GET", "/api/suppressions", "")
	if code != 200 {
		t.Fatalf("GET got %d", code)
	}
	var resp suppressionsResponse
	json.Unmarshal(respBody, &resp)
	if len(resp.Items) != 0 {
		t.Errorf("删除后应空, got %d items", len(resp.Items))
	}
}

// TestAPIDeleteSuppressionNotFound 验证删除不存在的 id 返回 404。
func TestAPIDeleteSuppressionNotFound(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.Config.SuppressPath = filepath.Join(dir, "suppressions.yaml")

	code, _ := reqSuppression(t, s, "DELETE", "/api/suppressions/fp:nonexistent", "")
	if code != 404 {
		t.Errorf("删除不存在的 id 应 404, got %d", code)
	}
}

// TestAPIGenerateBaseline 验证 POST /api/baseline 跑扫描 → 生成 baseline.json。
func TestAPIGenerateBaseline(t *testing.T) {
	dir := t.TempDir()
	// 创建触发规则的资产(dangerous settings.json)
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"skipDangerousModePermissionPrompt":true}`)

	s := newTestServer(t, dir)
	// 隔离 baseline 路径
	baselinePath := filepath.Join(dir, "baseline.json")
	s.Config.BaselinePath = baselinePath

	code, respBody := reqSuppression(t, s, "POST", "/api/baseline", "")
	if code != 200 {
		t.Fatalf("POST /api/baseline got %d: %s", code, respBody)
	}

	// 验证文件写入
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("读 baseline.json: %v", err)
	}
	var bs suppression.BaselineSet
	if err := json.Unmarshal(data, &bs); err != nil {
		t.Fatal(err)
	}
	if len(bs.Fingerprints) == 0 {
		t.Error("baseline 无指纹(期望 dangerous-skip-permission 触发至少 1 条)")
	}
}

// TestAPIGenerateBaselineAppends 验证 POST /api/baseline 追加到已有 baseline(不覆盖已有指纹)。
func TestAPIGenerateBaselineAppends(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"skipDangerousModePermissionPrompt":true}`)

	s := newTestServer(t, dir)
	baselinePath := filepath.Join(dir, "baseline.json")
	s.Config.BaselinePath = baselinePath

	// 预置一条已有指纹
	existing := &suppression.BaselineSet{
		Version:      "1",
		GeneratedAt:  "2026-01-01T00:00:00Z",
		Fingerprints: map[string]bool{"preexisting-fp": true},
	}
	if err := existing.Save(baselinePath); err != nil {
		t.Fatal(err)
	}

	// POST /api/baseline 应保留已有 + 添加新指纹
	code, respBody := reqSuppression(t, s, "POST", "/api/baseline", "")
	if code != 200 {
		t.Fatalf("POST /api/baseline got %d: %s", code, respBody)
	}

	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	var bs suppression.BaselineSet
	json.Unmarshal(data, &bs)
	if !bs.Fingerprints["preexisting-fp"] {
		t.Error("已有指纹被覆盖(应保留)")
	}
}
