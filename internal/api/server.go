package api

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/editor"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

type Server struct {
	Engine          *configengine.Engine
	Orchestrator    *security.Orchestrator
	Config          *config.Config
	ConfigPath      string // 配置文件路径(/api/dir-tags 回写用)
	Token           string
	History         *history.Store
	Agents          []configengine.Agent
	SelectedAgentID string
	Editor          *editor.Editor
}

func NewServer(eng *configengine.Engine, orch *security.Orchestrator, cfg *config.Config, token string, hist *history.Store, agents []configengine.Agent, ed *editor.Editor) *Server {
	if len(agents) == 0 {
		agents = configengine.DefaultAgents(eng.HomeDir)
	}
	current := ""
	if len(agents) > 0 {
		current = agents[0].ID
	}
	return &Server{Engine: eng, Orchestrator: orch, Config: cfg, Token: token, History: hist, Agents: agents, SelectedAgentID: current, Editor: ed}
}

func (s *Server) Router() *gin.Engine {
	r := gin.New()
	// C-SEC-1: gin v1.12 默认 ForwardedByClientIP=true 且信任 0.0.0.0/0,::/0,
	// 使 c.ClientIP() 信任攻击者伪造的 X-Forwarded-For,绕过非 loopback 绑定的
	// IP 白名单。本地单二进制工具不应运行在任何反向代理后,故不信任任何代理:
	// SetTrustedProxies(nil) → isTrustedProxy 恒 false → ClientIP() 返回真实 RemoteAddr。
	if err := r.SetTrustedProxies(nil); err != nil {
		// 理论不会失败;防御性处理。
		panic(fmt.Sprintf("set trusted proxies: %v", err))
	}
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
	api.GET("/tree", s.getTree)
	api.GET("/dir-tags", s.getDirTags)
	api.PUT("/dir-tags", s.putDirTags)
	api.GET("/raw", s.getRaw)
	api.POST("/scan", s.postScan)
	api.GET("/scan/result", s.getScanResult)
	api.GET("/findings", s.getFindings)
	api.GET("/health", s.getHealth)
	api.GET("/dashboard", s.getDashboard)
	api.GET("/detectors", s.getDetectors)
	api.GET("/agents", s.getAgents)
	api.GET("/project", s.getProject)
	api.GET("/history", s.getHistoryList)
	api.GET("/history/:id", s.getHistoryDetail)
	api.DELETE("/history/:id", s.deleteHistory)
}

func (s *Server) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, errorBody("not_implemented", "endpoint pending"))
}
