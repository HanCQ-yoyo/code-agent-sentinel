package api

import (
	"net/http"

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
	return r
}

func (s *Server) registerRoutes(api *gin.RouterGroup) {
	api.GET("/assets", s.getAssets)
	api.GET("/assets/:id", s.getAsset)
	api.POST("/scan", s.notImplemented)
	api.GET("/scan/result", s.notImplemented)
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
