package configengine

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// codexConfig 是 ~/.codex/config.toml 的解析结构(只建模安全相关字段;
// 其余字段不单独建模,通过 settings 资产的 raw 字段保留全文本供检测器匹配)。
type codexConfig struct {
	Model          string                       `toml:"model"`
	ApprovalPolicy string                       `toml:"approval_policy"`
	SandboxMode    string                       `toml:"sandbox_mode"`
	MCPServers     map[string]codexMCPEntry     `toml:"mcp_servers"`
	Profiles       map[string]codexProfileEntry `toml:"profiles"`
	// Hooks 对应 [hooks] 表;其 State 子字段对应 [hooks.state] 表(规格 §3.2:name→trusted)。
	//
	// Hooks 对应 [hooks] 表;其 State 子字段对应 [hooks.state] 表(规格 §3.2)。
	//
	// 落地偏差#1:计划与控制器补充决议原用扁平 `HooksState map[string]string \`toml:"hooks_state"\``,
	// 但 go-toml/v2 把表头 `hooks.state` 按点号拆成嵌套表,扁平字段 + tag 无法捕获
	// (实测:tag=`hooks_state` 时 HooksState 读到空 map;必须用嵌套 struct 才能读到值)。
	// 故改用 `Hooks struct { State map[string]... \`toml:"state"\` } \`toml:"hooks"\``。
	//
	// 落地偏差#2:原建模 hooks.state 值为 string(如 "trusted"/"untrusted"),
	// 但 Codex 实际格式每个 hooks.state 键是一个表 {enabled: bool, trusted_hash: string},
	// 非简单 string。原 map[string]string 导致 go-toml 报 "unhandled kv part: string"
	// (表值无法放入 string 槽位)。改用 map[string]codexHookStateEntry 匹配实际格式。
	Hooks struct {
		State map[string]codexHookStateEntry `toml:"state"`
	} `toml:"hooks"`
}

type codexMCPEntry struct {
	Command string            `toml:"command"`
	Args    []string          `toml:"args"`
	URL     string            `toml:"url"`
	Env     map[string]string `toml:"env"`
}

type codexProfileEntry struct {
	Model          string `toml:"model"`
	ApprovalPolicy string `toml:"approval_policy"`
	SandboxMode    string `toml:"sandbox_mode"`
}

// codexHookStateEntry 对应 [hooks.state.<key>] 子表(Codex 实际格式)。
// 原建模误用 map[string]string,但 Codex 的 hooks.state 值是一个表
// {enabled: bool, trusted_hash: string},非简单字符串。
type codexHookStateEntry struct {
	Enabled     bool   `toml:"enabled"`
	TrustedHash string `toml:"trusted_hash"`
}

// parseCodexConfig 解析 ~/.codex/config.toml,产出 settings + mcp_server + profile 资产。
//
// Codex 把 MCP 内联在 config.toml(无独立 .mcp.json),此处每个 [mcp_servers.<name>]
// 表抽成独立 mcp_server 资产,与 Claude 的 MCP 资产走同一条检测路径。
// profiles 是 Codex 配置预设,每个 [profiles.<name>] 产出一条 settings 资产(轻量:
// 有则解析,无则跳过,合并/覆盖语义不在本轮建模)。
//
// 损坏 TOML:产出 1 条带 parse_error 的 settings 占位资产(与 parseSettings 损坏分支
// 同构),扫描继续。文件不存在返回 error,由调用方决定是否忽略。
func parseCodexConfig(path string, scope Scope) ([]Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg codexConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		a := Asset{Type: AssetSettings, Scope: scope, SourcePath: path, Name: "config", ParseError: err.Error()}
		fillHash(&a)
		return []Asset{a}, nil
	}
	var out []Asset

	// settings 主体:Content=原文(UI 展示);Fields 保留结构化字段 + raw 全文本载体
	// (baseline 规则按 field: raw 全文本匹配 sandbox_mode/approval_policy)。
	base := Asset{Type: AssetSettings, Scope: scope, SourcePath: path, Name: "config", Content: string(data)}
	base.Fields = map[string]any{"raw": string(data)}
	if cfg.Model != "" {
		base.Fields["model"] = cfg.Model
	}
	if cfg.ApprovalPolicy != "" {
		base.Fields["approval_policy"] = cfg.ApprovalPolicy
	}
	if cfg.SandboxMode != "" {
		base.Fields["sandbox_mode"] = cfg.SandboxMode
	}
		// [hooks.state] 非空时建模到 Fields["hooks_state"]。
		// 每个 entry 是 {enabled: bool, trusted_hash: string} 而非简单 string。
		// baseline 检测器可据此匹配未受信 hook;空时不写键,避免 Fields 出现 nil 值。
		if len(cfg.Hooks.State) > 0 {
			hs := make(map[string]any, len(cfg.Hooks.State))
			for k, v := range cfg.Hooks.State {
				hs[k] = map[string]any{
					"enabled":      v.Enabled,
					"trusted_hash": v.TrustedHash,
				}
			}
			base.Fields["hooks_state"] = hs
		}
	fillHash(&base)
	out = append(out, base)

	// mcp_server 逐条:transport 由 command/url 推断(与 mcpAssets 一致)。
	// Content 设为原文(与 settings 同为 config.toml),UI 展示原文件文本。
	for name, e := range cfg.MCPServers {
		transport := "stdio"
		if e.URL != "" {
			transport = "http"
		}
		a := Asset{Type: AssetMCPServer, Scope: scope, SourcePath: path, Name: name, Content: string(data)}
		a.Fields = map[string]any{
			"name":      name,
			"transport": transport,
			"command":   e.Command,
			"args":      e.Args,
			"url":       e.URL,
			"env":       e.Env,
		}
		fillHash(&a)
		out = append(out, a)
	}

	// profile 逐条(轻量)。Content 设为原文(与 settings 同为 config.toml),
	// UI 展示原文件文本。
	for name, pr := range cfg.Profiles {
		a := Asset{Type: AssetSettings, Scope: scope, SourcePath: path, Name: "profile:" + name, Content: string(data)}
		a.Fields = map[string]any{
			"profile":         name,
			"model":           pr.Model,
			"approval_policy": pr.ApprovalPolicy,
			"sandbox_mode":    pr.SandboxMode,
		}
		fillHash(&a)
		out = append(out, a)
	}
	return out, nil
}
