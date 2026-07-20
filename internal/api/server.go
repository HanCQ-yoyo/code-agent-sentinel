package api

import (
	"context"
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
	"code-agent-sentinel/internal/scan"
	"code-agent-sentinel/internal/scheduler"
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
	Runner          ScanRunner           // HTTP/scheduler/CLI 共用的扫描路径(接口可注入 spy 测试)
	Scheduler       *scheduler.Scheduler // 进程内定时扫描调度器(main.go 注入)
	ScheduleManager *scheduler.Manager   // 多任务调度管理器(Task 6:/api/schedules CRUD 用)
}

// ScanRunner 抽象 *scan.Runner 的公共方法面,让 Server.Runner 可在测试中替换为 spy。
// *scan.Runner 已满足该接口(见 internal/scan/runner.go RunScan/EngineFor)。
type ScanRunner interface {
	RunScan(ctx context.Context, agentID string, detectorIDs []string) (*security.ScanResult, error)
	EngineFor(agentID string) *configengine.Engine
}

func NewServer(eng *configengine.Engine, orch *security.Orchestrator, cfg *config.Config, token string, hist *history.Store, agents []configengine.Agent, ed *editor.Editor) *Server {
	if len(agents) == 0 {
		agents = configengine.DefaultAgents(eng.HomeDir, eng.ClaudeDir)
	}
	current := ""
	if len(agents) > 0 {
		current = agents[0].ID
	}
	// Task 8:Runner 持真实 agents 列表(由 main.go 从 config 解析传入),
	// 内部按 agentID 池化 Engine,扫描时按请求/调度选 agent。
	return &Server{Engine: eng, Orchestrator: orch, Config: cfg, Token: token, History: hist, Agents: agents, SelectedAgentID: current, Editor: ed, Runner: scan.NewRunner(agents, orch, hist)}
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
	api.POST("/assets/:id/preview", s.previewAsset)
	api.PUT("/assets/:id/content", s.commitAsset)
	api.GET("/tree", s.getTree)
	api.GET("/dir-tags", s.getDirTags)
	api.PUT("/dir-tags", s.putDirTags)
	api.GET("/favorites", s.getFavorites)
	api.PUT("/favorites", s.putFavorites)
	api.GET("/raw", s.getRaw)
	api.POST("/scan", s.postScan)
	api.GET("/scan/result", s.getScanResult)
	api.GET("/findings", s.getFindings)
	api.GET("/health", s.getHealth)
	api.GET("/dashboard", s.getDashboard)
	api.GET("/detectors", s.getDetectors)
	api.GET("/detectors/config", s.getDetectorConfig)
	api.PUT("/detectors/config", s.putDetectorConfig)
	api.GET("/agents", s.getAgents)
	api.GET("/project", s.getProject)
	api.GET("/history", s.getHistoryList)
	api.GET("/history/:id", s.getHistoryDetail)
	api.DELETE("/history/:id", s.deleteHistory)
	api.POST("/suppressions", s.postSuppression)
	api.GET("/suppressions", s.getSuppressions)
	api.DELETE("/suppressions/:id", s.deleteSuppression)
	api.POST("/baseline", s.postBaseline)
	api.GET("/scheduler", s.getScheduler)
	api.PUT("/scheduler", s.putScheduler)
	api.GET("/schedules", s.getSchedules)
	api.POST("/schedules", s.postSchedule)
	api.PUT("/schedules/:agent_id", s.putSchedule)
	api.DELETE("/schedules/:agent_id", s.deleteSchedule)
	api.GET("/settings", s.getSettings)
	api.PUT("/settings", s.putSettings)
	api.GET("/pinned-projects", s.getPinnedProjects)
	api.PUT("/pinned-projects", s.putPinnedProjects)
}

func (s *Server) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, errorBody("not_implemented", "endpoint pending"))
}
