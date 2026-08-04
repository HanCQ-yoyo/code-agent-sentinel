package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestSetFindingState 验证 POST /api/finding-state 写入 → GET /api/finding-state/:fp 读回。
func TestSetFindingState(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	r := s.Router()

	body := map[string]any{
		"fingerprint": "abc123",
		"status":      "accepted",
		"priority":    "P2",
		"note":        "已确认安全",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/finding-state", bytes.NewReader(b))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body = %s", w.Code, w.Body.String())
	}

	// 读回:GET /api/finding-state/:fp
	req2 := httptest.NewRequest("GET", "/api/finding-state/abc123", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("GET status = %d", w2.Code)
	}
	var got map[string]any
	json.Unmarshal(w2.Body.Bytes(), &got)
	if got["status"] != "accepted" || got["priority"] != "P2" {
		t.Errorf("got = %+v", got)
	}
	if got["note"] != "已确认安全" {
		t.Errorf("note = %v, 期望 '已确认安全'", got["note"])
	}
}

// TestSetFindingStateValidation 验证空 fingerprint/status 被拒。
func TestSetFindingStateValidation(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	r := s.Router()

	b, _ := json.Marshal(map[string]any{"fingerprint": "", "status": "accepted"})
	req := httptest.NewRequest("POST", "/api/finding-state", bytes.NewReader(b))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("空 fingerprint 应 400, got %d", w.Code)
	}
}

// TestDeleteFindingState 验证 DELETE 重置单条状态。
func TestDeleteFindingState(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	r := s.Router()

	// 先写一条
	b, _ := json.Marshal(map[string]any{"fingerprint": "del1", "status": "accepted"})
	req := httptest.NewRequest("POST", "/api/finding-state", bytes.NewReader(b))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	// DELETE
	req2 := httptest.NewRequest("DELETE", "/api/finding-state/del1", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req2)
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d", w.Code)
	}

	// GET 应 404
	req3 := httptest.NewRequest("GET", "/api/finding-state/del1", nil)
	req3.Host = "127.0.0.1"
	req3.Header.Set("Authorization", "Bearer tok")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Errorf("DELETE 后 GET 应 404, got %d", w3.Code)
	}
}

// TestPruneReport 验证 prune-report 返回孤儿(已处置但 active 列表不含的 fingerprint)。
func TestPruneReport(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	r := s.Router()

	// 先写一条 state
	body, _ := json.Marshal(map[string]any{"fingerprint": "orphan1", "status": "accepted"})
	req := httptest.NewRequest("POST", "/api/finding-state", bytes.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	// prune-report:active=[] → orphan1 是孤儿
	req2 := httptest.NewRequest("GET", "/api/finding-state/prune-report", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req2)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Orphans []map[string]any `json:"orphans"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Orphans) != 1 || resp.Orphans[0]["fingerprint"] != "orphan1" {
		t.Errorf("orphans = %+v", resp.Orphans)
	}

	// prune-report:active=orphan1 → 不报孤儿
	req3 := httptest.NewRequest("GET", "/api/finding-state/prune-report?active=orphan1", nil)
	req3.Host = "127.0.0.1"
	req3.Header.Set("Authorization", "Bearer tok")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("status = %d", w3.Code)
	}
	var resp3 struct {
		Orphans []map[string]any `json:"orphans"`
	}
	json.Unmarshal(w3.Body.Bytes(), &resp3)
	if len(resp3.Orphans) != 0 {
		t.Errorf("active=orphan1 时应无孤儿, got %+v", resp3.Orphans)
	}
}

// TestBulkAccept 验证 POST /api/finding-state/bulk-accept 批量接受。
func TestBulkAccept(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	r := s.Router()

	body, _ := json.Marshal(map[string]any{"fingerprints": []string{"a", "b"}, "source": "bulk-accept"})
	req := httptest.NewRequest("POST", "/api/finding-state/bulk-accept", bytes.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["count"] != float64(2) {
		t.Errorf("count = %v, 期望 2", resp["count"])
	}

	// 验证两条都已 accepted
	for _, fp := range []string{"a", "b"} {
		req2 := httptest.NewRequest("GET", "/api/finding-state/"+fp, nil)
		req2.Host = "127.0.0.1"
		req2.Header.Set("Authorization", "Bearer tok")
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", fp, w2.Code)
		}
		var got map[string]any
		json.Unmarshal(w2.Body.Bytes(), &got)
		if got["status"] != "accepted" {
			t.Errorf("%s status = %v, 期望 accepted", fp, got["status"])
		}
	}
}

// TestBulkAcceptDoesNotOverwriteResolved 验证 BulkAccept 不覆盖已 resolved 的状态。
func TestBulkAcceptDoesNotOverwriteResolved(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	r := s.Router()

	// 先把 fp1 设为 resolved
	b, _ := json.Marshal(map[string]any{"fingerprint": "fp1", "status": "resolved"})
	req := httptest.NewRequest("POST", "/api/finding-state", bytes.NewReader(b))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	// bulk-accept fp1 + fp2
	b2, _ := json.Marshal(map[string]any{"fingerprints": []string{"fp1", "fp2"}})
	req2 := httptest.NewRequest("POST", "/api/finding-state/bulk-accept", bytes.NewReader(b2))
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req2)

	// fp1 应仍是 resolved(不被 accepted 覆盖)
	req3 := httptest.NewRequest("GET", "/api/finding-state/fp1", nil)
	req3.Host = "127.0.0.1"
	req3.Header.Set("Authorization", "Bearer tok")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	var got map[string]any
	json.Unmarshal(w3.Body.Bytes(), &got)
	if got["status"] != "resolved" {
		t.Errorf("fp1 status = %v, 期望 resolved(BulkAccept 不覆盖)", got["status"])
	}
}

// TestPostBaselineBulkAccept 验证 POST /api/baseline 跑全量扫描 → 批量接受 fingerprint 到 finding_states (sqlite)。
// 语义变更(Task 11):旧实现 union 到 baseline.json(已删);新实现调 BulkAccept 写 finding_states 表。
// Task 3 fix:handler 使用 s.FindingStates(sqlite 持久化),验证通过 API 而非读 YAML 文件。
func TestPostBaselineBulkAccept(t *testing.T) {
	dir := t.TempDir()
	// 创建触发规则的资产(dangerous settings.json)
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"skipDangerousModePermissionPrompt":true}`)

	s := newTestServer(t, dir)
	r := s.Router()

	req := httptest.NewRequest("POST", "/api/baseline", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/baseline got %d: %s", w.Code, w.Body.String())
	}

	// 验证响应含 accepted_count(handler 返回计数)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	ac, _ := resp["accepted_count"].(float64)
	scanFindings, _ := resp["scan_findings"].(float64)
	if ac < 1 {
		t.Errorf("accepted_count = %v, 期望 >= 1(scan_findings=%v)", ac, scanFindings)
	}

	// 验证 finding_states 表已写入:prune-report 不带 active 参数时应返回已处置状态
	req2 := httptest.NewRequest("GET", "/api/finding-state/prune-report", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("GET /api/finding-state/prune-report got %d", w2.Code)
	}
	var pr struct {
		Orphans []map[string]any `json:"orphans"`
	}
	json.Unmarshal(w2.Body.Bytes(), &pr)
	if len(pr.Orphans) == 0 {
		t.Error("prune-report 应返回至少 1 条已处置状态(不带 active 参数所有状态均为孤儿)")
	} else {
		for _, o := range pr.Orphans {
			if o["status"] == "accepted" {
				return // 找到了,验证通过
			}
		}
		t.Error("prune-report 返回的状态中应含 accepted 状态")
	}
}

// TestPostBaselineAgentScoped 验证 POST /api/baseline?agent=zzz 经 engineForQuery 返回 400 unknown_agent。
func TestPostBaselineAgentScoped(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	r := s.Router()

	req := httptest.NewRequest("POST", "/api/baseline?agent=zzz", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("未知 agent 应 400: got %d", w.Code)
	}
}

func contains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}
