package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newServerWithRulesDB 构造带 sqlite 规则库的测试服务器。
//
// Task 11 时规则路由尚未全局注册、newTestServer 也未注入 db,此 helper 独立
// 开 db + 同步 builtin + 注册路由。Task 12 把路由注册并入 registerRoutes、
// db 注入并入 newTestServer(经共享 newTestDB 助手),故此 helper 现仅转发
// 到 newTestServer——保留旧名以免改 8 处测试调用点;语义不变(仍得到带
// builtin 规则 + 已注册 8+8 路由的服务器)。
func newServerWithRulesDB(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return newTestServer(t, t.TempDir())
}

// rmRfRuleID 是测试用的 builtin 规则 ID(原 brief 写 filesystem.rm-rf-root-home,
// 实际 builtin ID 带前缀 destructive.,见 destructive_commands.yaml:234)。
const rmRfRuleID = "destructive.filesystem.rm-rf-root-home"

func TestGetDetectRulesReturnsList(t *testing.T) {
	srv := newServerWithRulesDB(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/detect-rules", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var dtos []ruleDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dtos); err != nil {
		t.Fatal(err)
	}
	if len(dtos) == 0 {
		t.Fatal("expected builtin rules in list")
	}
	// 列表里的 builtin 规则默认 enabled=true,source=builtin,can_edit=false。
	var found bool
	for _, d := range dtos {
		if d.ID == rmRfRuleID {
			found = true
			if d.Source != "builtin" {
				t.Errorf("rule %s source = %q, want builtin", d.ID, d.Source)
			}
			if d.CanEdit {
				t.Errorf("rule %s can_edit = true, want false (builtin 只读)", d.ID)
			}
			if !d.Enabled {
				t.Errorf("rule %s enabled = false, want true (无 override 默认启用)", d.ID)
			}
			if d.Domain != "detect" {
				t.Errorf("rule %s domain = %q, want detect", d.ID, d.Domain)
			}
		}
	}
	if !found {
		t.Fatalf("list 不含 %s;got %d rules", rmRfRuleID, len(dtos))
	}
}

func TestToggleBuiltinRuleDisabled(t *testing.T) {
	srv := newServerWithRulesDB(t)
	body, _ := json.Marshal(map[string]bool{"enabled": false})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/detect-rules/"+rmRfRuleID+"/enabled", bytes.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("toggle status = %d, body = %s", w.Code, w.Body.String())
	}
	// 重新 GET 单条,断言 enabled=false(override 已写入)。
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/detect-rules/"+rmRfRuleID, nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("GET after toggle status = %d, body = %s", w2.Code, w2.Body.String())
	}
	var dto ruleDTO
	if err := json.Unmarshal(w2.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Enabled {
		t.Errorf("after disable, rule %s enabled = true, want false", rmRfRuleID)
	}
	// builtin 规则本身仍存在(source 未变),只是被 override 禁用。
	if dto.Source != "builtin" {
		t.Errorf("after toggle, source = %q, want builtin (override 不改 source)", dto.Source)
	}
}

func TestPutBuiltinRuleReturns409(t *testing.T) {
	srv := newServerWithRulesDB(t)
	body, _ := json.Marshal(ruleDTO{ID: rmRfRuleID, Severity: "low", Source: "custom"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/detect-rules/"+rmRfRuleID, bytes.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("expected 409 for builtin PUT, got %d; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"]["code"] != "builtin_readonly" {
		t.Errorf("error code = %q, want builtin_readonly", resp["error"]["code"])
	}
}

// TestPostOverBuiltinReturns409 回归测试:POST /api/detect-rules 携带一个已存在的 builtin
// rule id 时必须 409,绝不能让 UpsertRule 的 ON CONFLICT DO UPDATE 把 builtin 行改成 custom
// 并覆盖其 match(那会静默禁用安全规则,且后续 PUT/DELETE 的 builtin_readonly 409 关卡失效——
// "内置只读"铁律被完整绕过)。POST 是 create(非 upsert),任何已存在 id(含 custom)都应拒。
func TestPostOverBuiltinReturns409(t *testing.T) {
	srv := newServerWithRulesDB(t)

	// 先 GET 该 builtin id 确认存在且 source=builtin/severity=critical/asset_type=hook,
	// 记录原始 match 作为"未被篡改"基准。
	w0 := httptest.NewRecorder()
	req0, _ := http.NewRequest("GET", "/api/detect-rules/"+rmRfRuleID, nil)
	req0.Host = "127.0.0.1"
	req0.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w0, req0)
	if w0.Code != 200 {
		t.Fatalf("GET builtin %s status = %d, want 200; body = %s", rmRfRuleID, w0.Code, w0.Body.String())
	}
	var orig ruleDTO
	if err := json.Unmarshal(w0.Body.Bytes(), &orig); err != nil {
		t.Fatal(err)
	}
	if orig.Source != "builtin" {
		t.Fatalf("baseline source = %q, want builtin", orig.Source)
	}
	if orig.Severity != "critical" {
		t.Fatalf("baseline severity = %q, want critical", orig.Severity)
	}
	if orig.AssetType != "hook" {
		t.Fatalf("baseline asset_type = %q, want hook", orig.AssetType)
	}
	origMatchJSON, _ := json.Marshal(orig.Match)

	// POST 同一 builtin id + 一个永不命中的 benign body,企图覆盖。
	attack, _ := json.Marshal(ruleDTO{
		ID:        rmRfRuleID,
		Severity:  "info",
		AssetType: "command",
		Match:     map[string]any{"field": "command", "op": "contains", "value": "zzz-never-match-zzz"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/detect-rules", bytes.NewReader(attack))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("POST over builtin id status = %d, want 409; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"]["code"] != "builtin_readonly" {
		t.Errorf("error code = %q, want builtin_readonly", resp["error"]["code"])
	}

	// 关键回归断言:builtin 规则原样不动——source 仍 builtin,severity/match 未被覆盖。
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/detect-rules/"+rmRfRuleID, nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("GET builtin after rejected POST status = %d, want 200; body = %s", w2.Code, w2.Body.String())
	}
	var after ruleDTO
	if err := json.Unmarshal(w2.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Source != "builtin" {
		t.Errorf("after rejected POST, source = %q, want builtin (POST 不应改 source)", after.Source)
	}
	if after.Severity != "critical" {
		t.Errorf("after rejected POST, severity = %q, want critical (未被覆盖)", after.Severity)
	}
	if after.AssetType != "hook" {
		t.Errorf("after rejected POST, asset_type = %q, want hook (未被覆盖)", after.AssetType)
	}
	afterMatchJSON, _ := json.Marshal(after.Match)
	if string(afterMatchJSON) != string(origMatchJSON) {
		t.Errorf("after rejected POST, match changed:\n  was %s\n  now %s", origMatchJSON, afterMatchJSON)
	}
}

func TestCreateCustomRuleValidates(t *testing.T) {
	srv := newServerWithRulesDB(t)
	// 合法规则 → 200
	body, _ := json.Marshal(ruleDTO{
		ID: "custom.test", Severity: "high", AssetType: "command",
		Match: map[string]any{"field": "command", "op": "contains", "value": "evil"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/detect-rules", bytes.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var dto ruleDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Source != "custom" || !dto.CanEdit || !dto.Enabled {
		t.Errorf("created rule dto = %+v, want source=custom/can_edit/enabled", dto)
	}
	// 非法规则(缺 op)→ 400
	badBody, _ := json.Marshal(ruleDTO{
		ID: "custom.bad", Severity: "high", AssetType: "command",
		Match: map[string]any{"field": "command"}, // 缺 op
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/detect-rules", bytes.NewReader(badBody))
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	req2.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w2, req2)
	if w2.Code != 400 {
		t.Fatalf("expected 400 for invalid rule, got %d; body = %s", w2.Code, w2.Body.String())
	}
}

func TestForkBuiltinToCustom(t *testing.T) {
	srv := newServerWithRulesDB(t)
	body, _ := json.Marshal(map[string]string{"new_id": "custom.forked"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/detect-rules/"+rmRfRuleID+"/fork", bytes.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("fork status = %d, body = %s", w.Code, w.Body.String())
	}
	var dto ruleDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Source != "custom" || dto.ID != "custom.forked" {
		t.Fatalf("fork result = %+v, want source=custom id=custom.forked", dto)
	}
	if !dto.CanEdit {
		t.Errorf("forked rule can_edit = false, want true (custom 可编辑)")
	}
	// 原 builtin 规则仍存在(没被覆盖/禁用):GET 原 id 仍 200,source=builtin。
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/detect-rules/"+rmRfRuleID, nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("builtin rule after fork status = %d, want 200 (原 builtin 应不受影响)", w2.Code)
	}
	var orig ruleDTO
	if err := json.Unmarshal(w2.Body.Bytes(), &orig); err != nil {
		t.Fatal(err)
	}
	if orig.Source != "builtin" {
		t.Errorf("after fork, original rule source = %q, want builtin (fork 不应改原行)", orig.Source)
	}
	// fork 出的新 id 也能 GET 到。
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/api/detect-rules/custom.forked", nil)
	req3.Host = "127.0.0.1"
	req3.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w3, req3)
	if w3.Code != 200 {
		t.Fatalf("GET forked rule status = %d, want 200", w3.Code)
	}
}

func TestValidateDraftDoesNotPersist(t *testing.T) {
	srv := newServerWithRulesDB(t)
	body, _ := json.Marshal(ruleDTO{
		ID: "draft.test", Severity: "high", AssetType: "command",
		Match: map[string]any{"field": "command", "op": "contains", "value": "x"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/detect-rules/validate", bytes.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("validate status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["valid"] != true {
		t.Errorf("validate resp = %+v, want valid=true", resp)
	}
	// draft.test 不应被持久化:GET 返回 404。
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/detect-rules/draft.test", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w2, req2)
	if w2.Code != 404 {
		t.Fatalf("GET draft.test after validate status = %d, want 404 (validate 不应落库)", w2.Code)
	}
}

// TestGetInterceptRulesReturnsList 验证拦截域对称:同 handler 经路径前缀分流返回 intercept 域规则。
func TestGetInterceptRulesReturnsList(t *testing.T) {
	srv := newServerWithRulesDB(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/intercept-rules", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var dtos []ruleDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dtos); err != nil {
		t.Fatal(err)
	}
	if len(dtos) == 0 {
		t.Fatal("expected builtin rules in intercept list")
	}
	for _, d := range dtos {
		if d.Domain != "intercept" {
			t.Errorf("rule %s domain = %q, want intercept", d.ID, d.Domain)
		}
	}
}

// TestDeleteCustomRule 验证 custom 规则可删,builtin 不可删。
func TestDeleteCustomRule(t *testing.T) {
	srv := newServerWithRulesDB(t)
	// 先建一条 custom 规则。
	create, _ := json.Marshal(ruleDTO{
		ID: "custom.todelete", Severity: "medium", AssetType: "command",
		Match: map[string]any{"field": "command", "op": "contains", "value": "bad"},
	})
	w0 := httptest.NewRecorder()
	req0, _ := http.NewRequest("POST", "/api/detect-rules", bytes.NewReader(create))
	req0.Host = "127.0.0.1"
	req0.Header.Set("Authorization", "Bearer tok")
	req0.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w0, req0)
	if w0.Code != 200 {
		t.Fatalf("create status = %d, body = %s", w0.Code, w0.Body.String())
	}
	// 删除 custom → 200。
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/detect-rules/custom.todelete", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("delete custom status = %d, body = %s", w.Code, w.Body.String())
	}
	// 再 GET → 404。
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/detect-rules/custom.todelete", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w2, req2)
	if w2.Code != 404 {
		t.Errorf("GET deleted rule status = %d, want 404", w2.Code)
	}
	// 删 builtin → 409。
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("DELETE", "/api/detect-rules/"+rmRfRuleID, nil)
	req3.Host = "127.0.0.1"
	req3.Header.Set("Authorization", "Bearer tok")
	srv.Router().ServeHTTP(w3, req3)
	if w3.Code != 409 {
		t.Errorf("delete builtin status = %d, want 409", w3.Code)
	}
}
