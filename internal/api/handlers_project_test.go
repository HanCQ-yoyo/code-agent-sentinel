package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestGetProjectNoCurrent(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/project", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var body map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["current"]; ok {
		t.Error("getProject 不应再返回 current(单项目选择已移除)")
	}
	if _, ok := body["projects"]; !ok {
		t.Error("getProject 应返回 projects 列表")
	}
}

func TestPostProjectRemoved(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()
	req := httptest.NewRequest("POST", "/api/project?path=/tmp/foo", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("POST /api/project 应已移除(404),实际 %d", w.Code)
	}
}
