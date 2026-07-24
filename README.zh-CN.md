# Code Agent Sentinel

> 把 Claude Code 的配置资产(settings、hooks、MCP servers、skills、commands、agents、plugins、CLAUDE.md/memory、keybindings、scripts)当作安全管控面来发现、解析、检测、编辑与监控的**本地单二进制安全看板**,并给出可解释的健康分。

[English](README.md) | 中文

## 核心能力

- **资产发现与解析**:扫描 `~/.claude/` 与项目 `.claude/`,覆盖 settings、permissions、hooks、MCP servers、skills、commands、agents、plugins、CLAUDE.md/memory、keybindings、scripts 等 11 类资产。支持多种 code agent:**Claude Code**(`~/.claude/`)与 **OpenAI Codex CLI**(`~/.codex/config.toml`、`AGENTS.md`、`prompts/`、`hooks.json`)。`sentinel setup` 自动探测已安装 agent;看板支持多 agent 聚合、各自独立扫描。
- **安全检测**:统一规则引擎(63 条内置规则)+ 提示注入扫描(含反混淆)+ 密钥扫描(gitleaks)+ 依赖漏洞(govulncheck / npm-audit)。子进程缺失时优雅降级。
- **抑制与 baseline**:`suppressions.yaml` 静默已知 finding;`baseline.json` 快照已接受指纹(CLI 或 API create / prune)。
- **健康分**:`Score = 100 × (1 − Σ(R(asset)·w(asset)) / (Rmax · Σ w(asset)))`,Rmax=10,0–100 五档,可解释 / 单调 / 可还原。
- **配置编辑**:原子写入 + 自动备份与迁移(`internal/editor`);configengine 保持只读。
- **定时扫描**:进程内 scheduler(`scan_interval` / `scan_enabled`)持续刷新历史;`sentinel scan` 一次性扫描不启 server。
- **自定义 `.claude` 目录**:`claude_dir` + `discovery.disabled_asset_types` 指向自定义配置根、跳过不关心的资产类型。
- **双语 UI**:界面 `zh` / `en` 切换(react-i18next,`language` 配置默认值);后端文案保持中文。
- **命中位置高亮**:规则 finding 携带 `Location{Line,StartCol,EndCol}`,在 Monaco 查看器中高亮。
- **项目置顶**:`pinned_projects` 把常用项目置顶 Assets 页并配色。
- **Dashboard**:健康分卡、风险摘要、检测器状态、资产盘点、历史趋势。

## 安装

预编译二进制以单归档发布(前端已内嵌)。本地构建:

```bash
git clone <repo> && cd code-agent-sentinel
make build          # 构建 web(npm run build)+ Go 二进制 -> bin/sentinel
```

需要 Go 1.25 与 Node.js(用于 `make web`)。生成的 `bin/sentinel` 完全自包含。

## 快速开始

```bash
./bin/sentinel                  # 127.0.0.1 + 随机端口,自动打开浏览器
# Token 打印到 stdout,经 URL fragment(#token=...)传递。

# 不启 server 的一次性扫描:
sentinel scan

# 远程开发机(服务仍仅绑 loopback,端口通过隧道转发):
ssh -L <port>:127.0.0.1:<port> <devhost>
```

## 配置文件

`~/.claude-sentinel/config.yaml`(在 `~/.claude/` 之外,避免自扫)。空字段经 `Resolve*` 方法回退默认值。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `bind` | string | 绑定地址。默认 `127.0.0.1`;非 loopback 需 `allowed_cidrs`(或 `--i-know-its-risky`)。 |
| `port` | int | `0` = 随机临时端口。 |
| `allowed_cidrs` | []string | IP 白名单;非 loopback 绑定必填。 |
| `basic_auth` | object | `user` + bcrypt `password_hash`;认证以 token 为准。 |
| `home_dir` | string | 覆盖发现用的 `$HOME`(调试)。 |
| `claude_dir` | string | `.claude` 根目录绝对路径;空 = `<home>/.claude`。 |
| `discovery.disabled_asset_types` | []string | 发现时跳过的资产类型(如 `mcp`、`scripts`)。 |
| `scan_interval` | duration string | 如 `30m`、`1h`;空/无效 = 关。 |
| `scan_enabled` | bool | 进程内 scheduler 总开关。 |
| `language` | string | `zh` / `en`;空 = 浏览器探测后回退 `zh`。 |
| `pinned_projects` | list | `{path, color}` 条目,Assets 页置顶。 |
| `dir_tags` | map | 按路径覆盖标签。 |
| `favorites` | []string | 收藏的资产 ID(后端持久化)。 |
| `backup_dir` | string | 备份根目录;空 = `~/.claude-sentinel/backups`。 |
| `max_backups` | int | `0` = 默认 20。 |
| `sentinel_rules_dir` | string | 全局自定义规则目录;空 = `~/.claude-sentinel/rules`。 |
| `suppress_path` | string | 抑制文件;空 = `~/.claude-sentinel/suppressions.yaml`。 |
| `baseline_path` | string | baseline 文件;空 = `~/.claude-sentinel/baseline.json`。 |
| `suppression_discount` | float | 抑制 finding 的残值扣分因子;`0`/负 = 0.3。 |
| `detectors` | object | 各检测器 `enabled` 开关 + 二进制路径(rules / secret / dep)。 |

示例:

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

## 命令行

所有子命令均可用 `--config` 覆盖配置路径。`--home` 覆盖发现用的 `$HOME`(调试/测试)。

| 命令 | 用途 |
| --- | --- |
| `sentinel` | 启动本地 SOC 看板 server(默认)。Flags:`--config`、`--bind`、`--port`、`--no-browser`、`--i-know-its-risky`、`--home`、`--token`、`--claude-dir`。 |
| `sentinel scan` | 一次性扫描(发现 → 扫描 → 写历史),不启 server;`--detectors=rules,secret` 限定运行的检测器。 |
| `sentinel uninstall` | 清理 `~/.claude-sentinel/`(历史、备份、baseline、抑制、规则)。**不**碰 `~/.claude` 与二进制。`--yes` 跳过确认;`--keep-config` 保留 `config.yaml`。 |
| `sentinel baseline` | `--create` 合并当前 finding 到 `baseline.json`;`--prune` 清理不复现的指纹。 |
| `sentinel rules` | `list` 打印 id/severity/source/valid;`validate [file]` 校验规则文件(无参 = 内置 + 全局)。 |

## 安全模型

- **默认仅 loopback**:`bind` 默认 `127.0.0.1`。非 loopback 绑定须有非空 `allowed_cidrs`,否则拒绝启动(用 `--i-know-its-risky` 覆盖)。
- **token 经 URL fragment 传递**:随机 token 通过 `#token=` 下发——不进 server 日志、不进 `Referer`——每个 API 请求校验。
- **Host 头 + 严格 CORS**:防 DNS rebinding。
- **非 loopback 不自动开浏览器**:多用户主机上开浏览器会经 `xdg-open` argv 泄露 token。
- **优雅降级**:缺失 `gitleaks` / `govulncheck` / `npm` 时检测器标记 `unavailable` 并附原因,整体扫描继续。
- **范围明确的卸载**:`sentinel uninstall` 仅删 `~/.claude-sentinel/`;Claude Code 配置与二进制不受影响。

## 开发

```bash
make build          # web + Go 二进制 -> bin/sentinel
make test           # go test ./...
make run            # build 后运行
make web            # 仅构建前端(vite build -> internal/api/web_dist)
make web-install    # cd web && npm install
make clean          # 删除 bin/、web/dist、web_dist
make release        # linux/darwin/windows 交叉编译归档
make build-cross GOOS=darwin GOARCH=arm64     # 单平台
make build-cross-fast GOOS=linux GOARCH=arm64 # 跳过前端重建
```

前端 e2e:`cd web && npm run test:e2e`(Playwright)。

技术栈:**Go 1.25**(Gin + cobra + embed)+ **React 18 / Vite / TypeScript / antd v5 / zustand / monaco-editor / react-i18next**。单二进制分发——React 构建产物 embed 进 Go 二进制。
