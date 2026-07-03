package api

import (
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
)

type Server struct {
	Engine       *configengine.Engine
	Orchestrator *security.Orchestrator
	Config       *config.Config
	Token        string
	lastResult   *security.ScanResult
}

func NewServer(eng *configengine.Engine, orch *security.Orchestrator, cfg *config.Config, token string) *Server {
	return &Server{Engine: eng, Orchestrator: orch, Config: cfg, Token: token}
}

func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsStrict())
	allowedHosts := []string{"127.0.0.1", "localhost"}
	if s.Config.Bind != "" && s.Config.Bind != "127.0.0.1" {
		allowedHosts = append(allowedHosts, s.Config.Bind)
	}
	r.Use(hostMiddleware(allowedHosts))
	if !isLoopback(s.Config.Bind) {
		r.Use(clientIPGuard(s.Config.AllowedCIDRs))
	}
	r.Use(authMiddleware(s.Token))

	api := r.Group("/api")
	s.registerRoutes(api)

	// SPA: 非 /api 路径先尝试 web_dist 下的真实静态文件,找不到再回退 index.html
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, errorBody("not_found", c.Request.URL.Path))
			return
		}
		// 清理路径,防止 .. 遍历;embed.FS 的 Open 已做了一定校验,这里显式 path.Clean
		rel := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if rel == "" || rel == "." || rel == "/" {
			rel = "index.html"
		}
		data, err := webFS.ReadFile("web_dist/" + rel)
		if err != nil {
			// 静态文件不存在 → 回退到 index.html(SPA 客户端路由)
			data, err = webFS.ReadFile("web_dist/index.html")
			if err != nil {
				c.String(http.StatusNotFound, "frontend not built; run `make web`")
				return
			}
			rel = "index.html"
		}
		ct := mime.TypeByExtension(path.Ext(rel))
		if ct == "" {
			ct = "application/octet-stream"
		}
		c.Data(http.StatusOK, ct, data)
	})

	return r
}

func (s *Server) registerRoutes(api *gin.RouterGroup) {
	api.GET("/assets", s.getAssets)
	api.GET("/assets/:id", s.getAsset)
	api.POST("/scan", s.postScan)
	api.GET("/scan/result", s.getScanResult)
	api.GET("/findings", s.getFindings)
	api.GET("/health", s.getHealth)
	api.GET("/dashboard", s.getDashboard)
	api.GET("/detectors", s.getDetectors)
	api.GET("/project", s.getProject)
	api.POST("/project", s.postProject)
}

func (s *Server) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, errorBody("not_implemented", "endpoint pending"))
}
