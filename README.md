# Code Agent Sentinel

> A local single-binary security console for Claude Code configuration assets — discover, scan, edit, and monitor the `~/.claude/` surface with an explainable health score.

## Features

- **Asset discovery & parsing**: reads `~/.claude/` and project `.claude/` — settings, permissions, hooks, MCP servers, skills, commands, agents, plugins, CLAUDE.md/memory, keybindings, scripts.
- **Security detection**: unified rule engine (63 built-in rules) + prompt-injection scanning with deobfuscation + secret scanning (gitleaks) + dependency audit (govulncheck / npm-audit). Missing scanner binaries degrade gracefully.
- **Suppressions & baseline**: silence known findings via `suppressions.yaml`; snapshot accepted fingerprints in `baseline.json` (create / prune from CLI or API).
- **Health score**: `Score = 100 × (1 − Σ(R(asset)·w(asset)) / (Rmax · Σ w(asset)))`, Rmax=10, 0–100, 5-tier — explainable, monotone, restorable.
- **Config editing**: atomic writes with automatic backup + migration (`internal/editor`); configengine stays read-only.
- **Scheduled scanning**: in-process scheduler (`scan_interval` / `scan_enabled`) keeps history fresh; `sentinel scan` does one-shot discover→scan→write-history without the server.
- **Custom `.claude` directory**: `claude_dir` + `discovery.disabled_asset_types` let you point at an alternate config root and skip asset types you don't care about.
- **Bilingual UI**: in-app `zh` / `en` switch (react-i18next) with `language` config default; backend strings remain Chinese.
- **Finding-location highlighting**: rule findings carry `Location{Line,StartCol,EndCol}` surfaced in the Monaco viewer.
- **Project pinning**: pin frequently-used projects to the top of the Assets page with a color tag (`pinned_projects`).
- **Dashboard**: health-score card, risk summary, detector status, asset inventory, history trends.

## Install

Prebuilt binaries are released as single archives (frontend embedded). For a local build:

```bash
git clone <repo> && cd code-agent-sentinel
make build          # builds web (npm run build) + Go binary -> bin/sentinel
```

Requires Go 1.25 and Node.js (for `make web`). The resulting `bin/sentinel` is fully self-contained.

## Quick Start

```bash
./bin/sentinel                  # 127.0.0.1 + random port, auto-opens browser
# Token is printed to stdout and passed via URL fragment (#token=...).

# One-shot scan without starting the server:
sentinel scan

# Remote dev box (service stays loopback-only; tunnel the port):
ssh -L <port>:127.0.0.1:<port> <devhost>
```

## Configuration

`~/.claude-sentinel/config.yaml` — deliberately outside `~/.claude/` to avoid self-scan. Empty fields fall back to defaults via `Resolve*` methods.

| Field | Type | Description |
| --- | --- | --- |
| `bind` | string | Bind address. Default `127.0.0.1`; non-loopback requires `allowed_cidrs` (or `--i-know-its-risky`). |
| `port` | int | `0` = random ephemeral port. |
| `allowed_cidrs` | []string | IP allowlist; mandatory for non-loopback binds. |
| `basic_auth` | object | `user` + bcrypt `password_hash`. Token auth remains primary. |
| `home_dir` | string | Overrides `$HOME` for discovery (debug). |
| `claude_dir` | string | Absolute path to `.claude` root; empty = `<home>/.claude`. |
| `discovery.disabled_asset_types` | []string | Skip asset types (e.g. `mcp`, `scripts`) during discovery. |
| `scan_interval` | duration string | e.g. `30m`, `1h`; empty/invalid = off. |
| `scan_enabled` | bool | Master switch for the in-process scheduler. |
| `language` | string | `zh` / `en`; empty = browser-detect then fall back to `zh`. |
| `pinned_projects` | list | `{path, color}` entries pinned on the Assets page. |
| `dir_tags` | map | Per-path label overrides. |
| `favorites` | []string | Pinned asset IDs (persisted server-side). |
| `backup_dir` | string | Backup root; empty = `~/.claude-sentinel/backups`. |
| `max_backups` | int | `0` = default 20. |
| `sentinel_rules_dir` | string | Global custom rules dir; empty = `~/.claude-sentinel/rules`. |
| `suppress_path` | string | Suppressions file; empty = `~/.claude-sentinel/suppressions.yaml`. |
| `baseline_path` | string | Baseline file; empty = `~/.claude-sentinel/baseline.json`. |
| `suppression_discount` | float | Residual risk factor for suppressed findings; `0`/negative = 0.3. |
| `detectors` | object | Per-detector `enabled` toggles + binary paths (rules / secret / dep). |

Example:

```yaml
bind: 127.0.0.1
port: 0
claude_dir: /home/me/.claude
scan_interval: 30m
scan_enabled: true
language: en
pinned_projects:
  - path: /work/myapp
    color: "#1677ff"
discovery:
  disabled_asset_types: [scripts]
```

## CLI

All subcommands accept `--config` to override the config path. `--home` overrides `$HOME` for debug/test.

| Command | Purpose |
| --- | --- |
| `sentinel` | Start the local SOC dashboard server (default). Flags: `--config`, `--bind`, `--port`, `--no-browser`, `--i-know-its-risky`, `--home`, `--token`, `--claude-dir`. |
| `sentinel scan` | One-shot scan (discover → scan → write history), no server. `--detectors=rules,secret` restricts which detectors run. |
| `sentinel uninstall` | Delete `~/.claude-sentinel/` (history, backups, baseline, suppressions, rules). Does **not** touch `~/.claude` or the binary. `--yes` skips confirmation; `--keep-config` retains `config.yaml`. |
| `sentinel baseline` | `--create` merges current findings into `baseline.json`; `--prune` removes non-reproducing fingerprints. |
| `sentinel rules` | `list` prints id/severity/source/valid; `validate [file]` checks rule files (no arg = builtin + global). |

## Security Model

- **Loopback by default**: `bind` defaults to `127.0.0.1`. Non-loopback binds require a non-empty `allowed_cidrs`, otherwise startup is refused (override with `--i-know-its-risky`).
- **Token via URL fragment**: the random token is delivered through `#token=` — it never reaches server logs or `Referer` headers — and is verified on every API request.
- **Host header + strict CORS**: guards against DNS rebinding.
- **No auto-open on non-loopback**: opening a browser would leak the token through `xdg-open` argv on multi-user hosts.
- **Graceful degradation**: missing `gitleaks` / `govulncheck` / `npm` binaries mark the detector `unavailable` with a reason; the overall scan continues.
- **Scoped uninstall**: `sentinel uninstall` only deletes `~/.claude-sentinel/`; your Claude Code config and the binary are untouched.

## Development

```bash
make build          # web + Go binary -> bin/sentinel
make test           # go test ./...
make run            # build + run
make web            # build frontend only (vite build -> internal/api/web_dist)
make web-install    # cd web && npm install
make clean          # remove bin/, web/dist, web_dist
make release        # cross-platform archives for linux/darwin/windows
make build-cross GOOS=darwin GOARCH=arm64     # single platform
make build-cross-fast GOOS=linux GOARCH=arm64 # skip frontend rebuild
```

Frontend e2e: `cd web && npm run test:e2e` (Playwright).

Tech stack: **Go 1.25** (Gin + cobra + embed) + **React 18 / Vite / TypeScript / antd v5 / zustand / monaco-editor / react-i18next**. Single binary distribution — the React build is embedded into the Go binary.

## Chinese (中文)

> code-agent-sentinel(二进制名 `sentinel`)是把 Claude Code 的配置资产(settings、hooks、MCP servers、skills、commands、agents、plugins、CLAUDE.md/memory、keybindings、scripts)当作安全管控面来发现、解析、检测、编辑与监控的**本地单二进制安全看板**。

### 核心能力

- **资产发现与解析**:扫描 `~/.claude/` 与项目 `.claude/`,覆盖 11 类资产。
- **安全检测**:统一规则引擎(63 条内置规则)+ 提示注入扫描(含反混淆)+ 密钥扫描(gitleaks)+ 依赖漏洞(govulncheck / npm-audit)。子进程缺失时优雅降级。
- **抑制与 baseline**:`suppressions.yaml` 静默已知 finding;`baseline.json` 快照已接受指纹(CLI 或 API create / prune)。
- **健康分**:`Score = 100 × (1 − Σ(R(asset)·w(asset)) / (Rmax · Σ w(asset)))`,Rmax=10,0–100 五档,可解释 / 单调 / 可还原。
- **配置编辑**:原子写入 + 自动备份与迁移(`internal/editor`);configengine 保持只读。
- **定时扫描**:进程内 scheduler(`scan_interval` / `scan_enabled`)持续刷新历史;`sentinel scan` 一次性扫描不启 server。
- **自定义 `.claude` 目录**:`claude_dir` + `discovery.disabled_asset_types` 指向自定义配置根、跳过不关心的资产类型。
- **双语 UI**:界面 `zh` / `en` 切换(react-i18next,`language` 配置默认值);后端文案保持中文。
- **命中位置高亮**:规则 finding 携带 `Location{Line,StartCol,EndCol}`,在 Monaco 查看器中高亮。
- **项目前置**:`pinned_projects` 把常用项目置顶 Assets 页并配色。
- **Dashboard**:健康分卡、风险摘要、检测器状态、资产盘点、历史趋势。

### 配置文件

`~/.claude-sentinel/config.yaml`(在 `~/.claude/` 之外,避免自扫)。关键字段:

| 字段 | 说明 |
| --- | --- |
| `bind` / `port` / `allowed_cidrs` | 绑定地址 / 端口(0=随机)/ 非 loopback 必填的 IP 白名单。 |
| `claude_dir` | `.claude` 目录绝对路径;空 = `<home>/.claude`。 |
| `discovery.disabled_asset_types` | 发现时跳过的资产类型。 |
| `scan_interval` / `scan_enabled` | 定时扫描间隔(如 `30m`)与总开关。 |
| `language` | `zh` / `en`;空 = 浏览器探测后回退 `zh`。 |
| `pinned_projects` | `{path, color}` 列表,Assets 页置顶。 |
| `basic_auth` | `user` + bcrypt `password_hash`;认证以 token 为准。 |
| `backup_dir` / `max_backups` | 备份目录与上限(默认 20)。 |
| `sentinel_rules_dir` / `suppress_path` / `baseline_path` | 自定义规则目录 / 抑制文件 / baseline 文件。 |
| `suppression_discount` | 抑制 finding 残值扣分因子;默认 0.3。 |
| `detectors` | 检测器运行期配置(启用开关 + 二进制路径)。 |

### 命令行

| 命令 | 用途 |
| --- | --- |
| `sentinel` | 启动本地 SOC 看板(默认)。Flags: `--config` / `--bind` / `--port` / `--no-browser` / `--i-know-its-risky` / `--home` / `--token` / `--claude-dir`。 |
| `sentinel scan` | 一次性扫描(发现→扫描→写历史),不启 server;`--detectors` 限定检测器。 |
| `sentinel uninstall` | 清理 `~/.claude-sentinel/`(不删 `~/.claude` 与二进制);`--yes` 跳过确认,`--keep-config` 保留 `config.yaml`。 |
| `sentinel baseline` | `--create` 合并指纹到 baseline;`--prune` 清理不复现指纹。 |
| `sentinel rules` | `list` 列规则;`validate [file]` 校验规则文件。 |

**安全模型**:默认仅绑 `127.0.0.1`;非 loopback 必须配 `allowed_cidrs`(或 `--i-know-its-risky`)。token 经 URL fragment 传递(不进 server log / Referer),每个 API 请求校验;严格 CORS + Host 头校验防 DNS rebinding。扫描器缺失时优雅降级。`sentinel uninstall` 仅删 `~/.claude-sentinel/`,不动 `~/.claude` 与二进制。

**开发**:`make build` / `make test` / `make run` / `make web` / `make release`。技术栈:Go 1.25(Gin + cobra + embed)+ React 18 / Vite / TypeScript / antd v5 / zustand / monaco-editor / react-i18next,单二进制分发(前端 embed 进二进制)。
