package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"code-agent-sentinel/internal/api"
	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/editor"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/scan"
	"code-agent-sentinel/internal/scheduler"
	"code-agent-sentinel/internal/security"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		log.Fatal(err)
	}
}

func newRootCmd() *cobra.Command {
	var (
		cfgPath       string
		bindFlag      string
		portFlag      int
		noBrowser     bool
		risky         bool
		homeFlag      string
		tokenFlag     string
		claudeDirFlag string
	)
	cmd := &cobra.Command{
		Use:   "sentinel",
		Short: "Claude Code 配置安全态势看板(P1 只读)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cfgPath, bindFlag, portFlag, noBrowser, risky, homeFlag, tokenFlag, claudeDirFlag)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "配置文件路径(默认 ~/.claude-sentinel/config.yaml)")
	cmd.Flags().StringVar(&bindFlag, "bind", "", "覆盖 bind 地址")
	cmd.Flags().IntVar(&portFlag, "port", 0, "覆盖端口(0=随机)")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "不自动打开浏览器")
	cmd.Flags().BoolVar(&risky, "i-know-its-risky", false, "非 loopback 且无白名单时强制启动(危险)")
	cmd.Flags().StringVar(&homeFlag, "home", "", "覆盖 home 目录(调试)")
	// C-BUILD-1: 调试/测试用固定 token,覆盖随机 genToken()。生产场景应留空走随机。
	cmd.Flags().StringVar(&tokenFlag, "token", "", "覆盖随机生成的 token(调试/测试用,生产场景留空)")
	// Task 3:--claude-dir 覆盖 cfg.ResolveClaudeDir(home);空走配置/默认回退。
	cmd.Flags().StringVar(&claudeDirFlag, "claude-dir", "", ".claude 目录绝对路径(默认 home/.claude)")
	// Task 15:baseline / rules 子命令
	cmd.AddCommand(newBaselineCmd())
	cmd.AddCommand(newRulesCmd())
	// Task 9:scan 子命令(一次性扫描写历史,不启动 HTTP server)
	cmd.AddCommand(newScanCmd())
	// Task 11:setup 子命令(huh TUI 交互式配置 code agent)
	cmd.AddCommand(newSetupCmd())
	// Task 22:uninstall 子命令(清理 ~/.claude-sentinel 数据目录)
	cmd.AddCommand(newUninstallCmd())
	return cmd
}

func run(ctx context.Context, cfgPath, bindFlag string, portFlag int, noBrowser, risky bool, homeFlag, tokenFlag, claudeDirFlag string) error {
	if cfgPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return err
		}
		cfgPath = p
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if bindFlag != "" {
		cfg.Bind = bindFlag
	}
	if portFlag != 0 {
		cfg.Port = portFlag
	}
	if err := api.ValidateBindPolicy(cfg, risky); err != nil {
		return err
	}
	home := cfg.HomeDir
	if homeFlag != "" {
		home = homeFlag
	}
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		home = h
	}
	// #2:.claude 目录解析:--claude-dir > cfg.ClaudeDir > home/.claude。
	// Task 8:--claude-dir 写回 cfg.ClaudeDir,使 ResolveAgents 的回退路径
	// (Agents 空 → 用 ClaudeDir 构造单项 claude-code)与 shouldPromptSetup 都能 honor flag。
	// ResolveAgents/shouldPromptSetup 直接读 cfg.ClaudeDir,无需保留本地 claudeDir 变量。
	if claudeDirFlag != "" {
		cfg.ClaudeDir = claudeDirFlag
	}

	cfg.EnsureDetectors() // 确保 Detectors 非 nil,检测器持其指针,API 写原地生效

	// 多 agent:从 config 解析 enabled agents,桥接为 configengine.Agent。
	agentCfgs := cfg.ResolveAgents(home)
	agentItems := make([]configengine.AgentItem, 0, len(agentCfgs))
	for _, a := range agentCfgs {
		if a.Enabled {
			agentItems = append(agentItems, configengine.AgentItem{ID: a.ID, Enabled: a.Enabled, RootDir: a.RootDir, ClaudeJSON: a.ClaudeJSON})
		}
	}
	engAgents := configengine.AgentsFromSpecs(home, agentItems)
	if len(engAgents) == 0 {
		return fmt.Errorf("无启用的 code agent,运行 sentinel setup 配置")
	}
	// 本轮 Engine 仍取首个(Runner 内部按 agentID 池化,扫描时选)
	eng := configengine.NewEngineFromAgent(engAgents[0])
	// #2:发现范围桥接(config 不导入 configengine,在此转 []AssetType)。
	// NewEngineFromAgent 返回的 Engine 的 DisabledAssetTypes 为空,这里从 config 回填。
	if cfg.Discovery != nil {
		for _, s := range cfg.Discovery.DisabledAssetTypes {
			eng.DisabledAssetTypes = append(eng.DisabledAssetTypes, configengine.AssetType(s))
		}
	}
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(home, cfg.Detectors))
	r.Register(security.NewSecretDetector(cfg.Detectors))
	r.Register(security.NewDependencyDetector(cfg.Detectors))
	orch := &security.Orchestrator{Registry: r}

	// C-BUILD-1: --token 非空则用之(调试/测试),否则随机生成。
	token := tokenFlag
	if token == "" {
		token = genToken()
	}
	histPath := filepath.Join(home, ".claude-sentinel", "history")
	hist := history.NewStore(histPath)
	ed := editor.New(eng, cfg.BackupDir, cfg.MaxBackups)
	srv := api.NewServer(eng, orch, cfg, token, hist, engAgents, ed)
	srv.ConfigPath = cfgPath
	// 多任务调度:每 agent 一个 Scheduler,Manager 增量同步。
	// makeRun 按 agentID 闭包 srv.Runner.RunScan(内部 EngineFor 按 agentID 池化选 Engine)。
	mgr := scheduler.NewManager(func(agentID string) func(context.Context) error {
		return func(ctx context.Context) error {
			// 调度器始终跑 global scope 扫描(Task 14+ 可能按 schedule 配置细化)。
			_, err := srv.Runner.RunScan(ctx, agentID, scan.ScanScope{Type: "global"}, nil)
			return err
		}
	})
	srv.ScheduleManager = mgr
	mgr.Apply(cfg.ResolveSchedules(agentCfgs))
	defer mgr.Stop()
	httpSrv := &http.Server{Handler: srv.Router()}

	ln, err := net.Listen("tcp", api.ResolveListenAddr(cfg))
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	am := resolveAccessMethod(cfg.Bind, port, home)
	fmt.Println("==================================================")
	fmt.Printf("sentinel 已启动 | token: %s\n", token)
	fmt.Printf("本地访问:   %s#token=%s\n", am.URL, token)
	if am.TunnelCmd != "" {
		fmt.Printf("远程访问(SSH 隧道): %s\n", am.TunnelCmd)
	}
	// I-SEC-3: 仅在确有可解析白名单时提示"已启用";--i-know-its-risky 旁路空白名单时
	// 明确警告无白名单,避免误导。
	if !isLoopback(cfg.Bind) {
		if len(cfg.AllowedCIDRs) > 0 {
			fmt.Println("⚠ bind 非 loopback,已启用 IP 白名单。请确认访问来源。")
		} else {
			fmt.Println("⚠ 无 IP 白名单 —— 所有网络均可访问,请确认访问来源。")
		}
	}
	fmt.Println("==================================================")
	if !noBrowser {
		// I-SEC-5: 非 loopback 绑定不自动打开浏览器。token 经 URL fragment 传入,
		// openBrowser 把含 token 的 URL 作为 argv 传给 xdg-open(多为 shell 脚本),
		// 多用户主机上 ps aux | grep xdg-open 会泄露 token。loopback 为单用户工作站,
		// 风险低,保留原行为。
		if isLoopback(cfg.Bind) {
			openBrowser(am.URL + "#token=" + token)
		} else {
			fmt.Printf("非 loopback 绑定未自动打开浏览器;请手动复制访问: %s#token=%s\n", am.URL, token)
		}
	}
	// Task 8:首次无配置提示。Agents 空 且 ClaudeDir 空 且 默认 ~/.claude 不存在时,
	// 提示用户运行 setup(ResolveAgents 会回退到默认 home/.claude,服务仍可启动)。
	if shouldPromptSetup(cfg, filepath.Join(home, ".claude")) {
		fmt.Println("提示:尚未配置 code agent。运行 `sentinel setup` 进行交互式配置。")
	}
	httpSrv.Serve(ln)
	return nil
}

// shouldPromptSetup 判断是否提示运行 setup:Agents 空 且 ClaudeDir 空 且 默认 ~/.claude 不存在。
// 回退路径(claude_dir 非空或 ~/.claude 存在)属正常默认,不打扰。
func shouldPromptSetup(cfg *config.Config, defaultClaudeDir string) bool {
	if len(cfg.Agents) > 0 {
		return false
	}
	if cfg.ClaudeDir != "" {
		return false
	}
	if _, err := os.Stat(defaultClaudeDir); err == nil {
		return false // 默认 .claude 存在,用回退即可
	}
	return true
}

type accessMethod struct {
	URL       string
	TunnelCmd string
}

func resolveAccessMethod(bind string, port int, home string) accessMethod {
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	var tunnel string
	if isLoopback(bind) {
		// 远程:ssh -L <port>:127.0.0.1:<port> <devhost>
		tunnel = fmt.Sprintf("ssh -L %d:127.0.0.1:%d <你的开发机>", port, port)
	} else {
		url = fmt.Sprintf("http://%s:%d/", bind, port)
	}
	return accessMethod{URL: url, TunnelCmd: tunnel}
}

func isLoopback(a string) bool { return a == "127.0.0.1" || a == "localhost" || a == "::1" }

func genToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}
