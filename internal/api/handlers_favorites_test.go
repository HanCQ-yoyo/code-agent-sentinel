package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
)

func reqFavorites(t *testing.T, s *Server, method, body string) (int, favoritesResponse) {
	t.Helper()
	r := s.Router()
	var req = httptest.NewRequest(method, "/api/favorites", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp favoritesResponse
	if w.Code == 200 {
		json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return w.Code, resp
}

// TestGetFavoritesEmpty 验证未配置时 GET 返回空列表(非 null)。
func TestGetFavoritesEmpty(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	code, resp := reqFavorites(t, s, "GET", "")
	if code != 200 {
		t.Fatalf("got %d", code)
	}
	if resp.Favorites == nil {
		t.Errorf("空时应返回 [] 而非 null")
	}
	if len(resp.Favorites) != 0 {
		t.Errorf("空时应无收藏, got %v", resp.Favorites)
	}
}

// TestPutFavoritesPersists 验证 PUT 整体替换并持久化到 SQLite UserPrefsStore。
func TestPutFavoritesPersists(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.UserPrefs = config.NewUserPrefsStore(s.DB)

	body := `{"favorites":["a1","b2","c3"]}`
	code, resp := reqFavorites(t, s, "PUT", body)
	if code != 200 {
		t.Fatalf("got %d", code)
	}
	if len(resp.Favorites) != 3 || resp.Favorites[0] != "a1" {
		t.Errorf("回写未返回: %v", resp.Favorites)
	}
	// 验证经 UserPrefsStore 持久化
	v, err := s.UserPrefs.Get("favorites")
	if err != nil || v == "" {
		t.Fatalf("favorites 未持久化到 UserPrefsStore: err=%v, val=%q", err, v)
	}
	var favs []string
	json.Unmarshal([]byte(v), &favs)
	if len(favs) != 3 || favs[0] != "a1" {
		t.Errorf("持久化丢失: %v", favs)
	}
}

// TestPutFavoritesReplacesNotMerges 验证整体替换语义:第二次 PUT 不含的 id 被移除。
func TestPutFavoritesReplacesNotMerges(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.UserPrefs = config.NewUserPrefsStore(s.DB)

	reqFavorites(t, s, "PUT", `{"favorites":["a1","b2"]}`)
	code, resp := reqFavorites(t, s, "PUT", `{"favorites":["c3"]}`)
	if code != 200 {
		t.Fatalf("got %d", code)
	}
	if len(resp.Favorites) != 1 || resp.Favorites[0] != "c3" {
		t.Errorf("整体替换应只剩 c3, got %v", resp.Favorites)
	}
}

// TestPutFavoritesRejectsNonString 验证非字符串元素被拒(防配置污染)。
func TestPutFavoritesRejectsNonString(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	code, _ := reqFavorites(t, s, "PUT", `{"favorites":["ok",123]}`)
	if code != 400 {
		t.Errorf("非字符串元素应 400, got %d", code)
	}
}

// TestPutFavoritesDedupes 验证重复 id 去重(收藏是集合语义,无序无重)。
func TestPutFavoritesDedupes(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	code, resp := reqFavorites(t, s, "PUT", `{"favorites":["a1","a1","b2"]}`)
	if code != 200 {
		t.Fatalf("got %d", code)
	}
	if len(resp.Favorites) != 2 {
		t.Errorf("重复应去重, got %v", resp.Favorites)
	}
}
