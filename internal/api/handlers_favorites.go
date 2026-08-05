package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// favoritesResponse 是 GET/PUT /api/favorites 的响应体:收藏的资产 id 列表。
type favoritesResponse struct {
	Favorites []string `json:"favorites"`
}

// getFavorites 返回当前收藏的资产 id 列表(从 SQLite UserPrefsStore 读取)。
func (s *Server) getFavorites(c *gin.Context) {
	c.JSON(http.StatusOK, favoritesResponse{Favorites: s.favoritesList()})
}

// favoritesList 返回去重后的收藏 id 切片(空则为 []string{},非 nil)。
func (s *Server) favoritesList() []string {
	if s.UserPrefs == nil {
		return []string{}
	}
	v, err := s.UserPrefs.Get("favorites")
	if err != nil || v == "" {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal([]byte(v), &ids); err != nil {
		return []string{}
	}
	// 去空去重
	return dedupeFavorites(ids)
}

// putFavoritesBody 是 PUT /api/favorites 的请求体:完整 id 列表(非增量)。
type putFavoritesBody struct {
	Favorites []string `json:"favorites"`
}

// putFavorites 用请求体整体替换收藏列表并持久化到 SQLite UserPrefsStore。
func (s *Server) putFavorites(c *gin.Context) {
	var body putFavoritesBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	// 去空去重
	cleaned := dedupeFavorites(body.Favorites)
	if s.UserPrefs != nil {
		b, _ := json.Marshal(cleaned)
		_ = s.UserPrefs.Set("favorites", string(b))
	}
	c.JSON(http.StatusOK, favoritesResponse{Favorites: cleaned})
}

// dedupeFavorites 去空 + 去重,保序(首次出现顺序)。
func dedupeFavorites(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
