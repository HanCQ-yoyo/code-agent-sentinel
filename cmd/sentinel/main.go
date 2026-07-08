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
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		log.Fatal(err)
	}
}

func newRootCmd() *cobra.Command {
	var (
		cfgPath   string
		bindFlag  string
		portFlag  int
		noBrowser bool
		risky     bool
		homeFlag  string
		tokenFlag string
	)
	cmd := &cobra.Command{
		Use:   "sentinel",
		Short: "Claude Code 配置安全态势看板(P1 只读)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cfgPath, bindFlag, portFlag, noBrowser, risky, homeFlag, tokenFlag)
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
	return cmd
}

func run(ctx context.Context, cfgPath, bindFlag string, portFlag int, noBrowser, risky bool, homeFlag, tokenFlag string) error {
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

	eng := configengine.NewEngine(home)
	r := security.NewRegistry()
	r.Register(security.NewBaselineDetector())
	r.Register(security.NewInjectionDetector())
	r.Register(security.NewSecretDetector(""))
	r.Register(security.NewDependencyDetector("", ""))
	orch := &security.Orchestrator{Registry: r}

	// C-BUILD-1: --token 非空则用之(调试/测试),否则随机生成。
	token := tokenFlag
	if token == "" {
		token = genToken()
	}
	histPath := filepath.Join(home, ".claude-sentinel", "history")
	hist := history.NewStore(histPath)
	srv := api.NewServer(eng, orch, cfg, token, hist, configengine.DefaultAgents(home))
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
	httpSrv.Serve(ln)
	return nil
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
