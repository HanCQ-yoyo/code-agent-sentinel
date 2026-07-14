package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

// favoritesResponse 是 GET/PUT /api/favorites 的响应体:收藏的资产 id 列表。
// 空列表返回 [] 而非 null(前端可直接 .length / 遍历)。
type favoritesResponse struct {
	Favorites []string `json:"favorites"`
}

// getFavorites 返回当前收藏的资产 id 列表。
func (s *Server) getFavorites(c *gin.Context) {
	c.JSON(http.StatusOK, favoritesResponse{Favorites: s.favoritesList()})
}

// favoritesList 返回去重后的收藏 id 切片(空则为 []string{},非 nil)。
func (s *Server) favoritesList() []string {
	seen := make(map[string]struct{}, len(s.Config.Favorites))
	out := make([]string, 0, len(s.Config.Favorites))
	for _, id := range s.Config.Favorites {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// putFavoritesBody 是 PUT /api/favorites 的请求体:完整 id 列表(非增量)。
type putFavoritesBody struct {
	Favorites []string `json:"favorites"`
}

// putFavorites 用请求体整体替换收藏列表并持久化到配置文件。
//
// 整体替换(非增量)语义与 dir-tags 一致:前端持有完整列表,增删后整体回写。
// 校验:仅允许字符串元素,防 payload 写入非字符串污染配置。空元素与重复去重。
// 持久化到 ~/.claude-sentinel/config.yaml(跨重启/跨端口保留,localStorage 受
// origin=host:port 影响在随机端口重启时丢失,故改存后端配置)。
func (s *Server) putFavorites(c *gin.Context) {
	var body putFavoritesBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	// []string 字段遇非字符串元素(如数字 123)在 ShouldBindJSON 阶段即报错
	// (cannot unmarshal number into string),故无需再逐元素类型校验。此处仅去空去重。
	s.Config.Favorites = dedupeFavorites(body.Favorites)
	if s.ConfigPath == "" {
		// 测试场景无路径:仅内存更新,不持久化。
		c.JSON(http.StatusOK, favoritesResponse{Favorites: s.favoritesList()})
		return
	}
	if err := config.Save(s.ConfigPath, s.Config); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, favoritesResponse{Favorites: s.favoritesList()})
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
