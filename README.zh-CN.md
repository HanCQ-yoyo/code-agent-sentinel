# Code Agent Sentinel

> 一款本地单二进制 AI 编程助手安全控制台 ——
> 发现、检测、拦截、监控你的 Claude Code 配置攻击面。

[English](README.md) | 中文

## 为什么需要 Sentinel？

AI 编程助手（如 Claude Code 和 Codex CLI）会积累大量配置攻击面：MCP 服务器、hooks、skills、
凭据文件、自定义命令 —— 分布在 12 种以上的资产类型中。人工审查不具可扩展性。

Sentinel 以一个二进制提供 SOC 风格安全看板：
**资产发现 → 静态检测 → 运行时拦截 → 可解释的健康分。**

## 核心能力

### 多 Agent 资产发现

从 Claude Code（`~/.claude/`）和 OpenAI Codex CLI（`~/.codex/`）发现并解析资产。
覆盖 settings、permissions、hooks、MCP servers、skills、commands、agents、plugins、
CLAUDE.md/memory、keybindings、scripts 及凭据文件 —— 两种 agent 共 12 种资产类型。

`code-agent-sentinel setup` 自动探测已安装的 agent。看板以多 agent 聚合视图展示，
并支持独立按 agent 扫描。

### 安全检测引擎

- **统一规则引擎**:256 条内置规则 + 6 条跨资产组合规则
- **提示注入扫描** 带反混淆
- **密钥扫描**（gitleaks）+ **依赖审计**（govulncheck / npm-audit）
- 规则存储在 sqlite 中，热生效 —— 通过设置界面创建、编辑、fork、启用或禁用规则，
  无需重启。扫描器二进制缺失时优雅降级。

### 运行时命令拦截

一条同时对 Claude Code 和 Codex CLI 生效的 `PreToolUse` Bash hook，
通过 span 感知管线评估每条 shell 命令：

解析 → 短路判断 → 反混淆 → 链式拆分（`&&`/`;`/`||`/`|`）→
span 分类器（引号/注释/命令替换）→ 规则引擎 → 决策

- 拦截破坏性命令（`rm -rf /`、`git reset --hard`、链式绕过、ANSI-C 混淆）
- 避免数据字面量误报（`echo "rm -rf /"` 安全放行）
- 置信度打分（高/低/未知）+ 严格/宽松模式 + 放行清单
- **fail-open**：hook 始终 exit 0；拒绝仅通过 stdout JSON 表达
- `/intercept` 页面展示完整审计日志（含置信度 + 命中 span 元数据）

### 风险看板与处置生命周期

- **健康分**：0–100 五档，基于可解释公式 —— 单调、可还原。修复一个 finding，分数只升不降。
- **统一处置生命周期**：open → in_progress → resolved / false_positive / accepted，
  支持批量接受和清理
- **能力看板**：按资产结构化展示 allowed-tools、hook 事件、MCP 命令、memory 大纲
- **误报减负**：否定上下文抑制、资产内去重、双视图 FindingTable（按 finding / 按资产聚合）
- **扫描任务视图**：按 agent 的历史记录，含扫描范围和目标列

### 配置管理

所有运行时设置通过 Web 界面管理并持久化到 SQLite，无需手动编辑 YAML：

- 各检测器启用/禁用 + 二进制路径
- 各 agent 扫描计划，进程内调度器
- Guard 配置（策略、超时、模式）—— 热生效
- 项目置顶 + 彩色标签

## 快速开始

```bash
./bin/code-agent-sentinel                  # 127.0.0.1 + 随机端口，自动打开浏览器
# Token 打印到 stdout，经 URL fragment (#token=...) 传递。

# 不启动 server 的一次性扫描：
code-agent-sentinel scan

# 远程开发机（服务仅绑 loopback，通过隧道转发端口）：
ssh -L <port>:127.0.0.1:<port> <devhost>
```

## 配置文件

`~/.code-agent-sentinel/config.yaml` —— 刻意放在 `~/.claude/` 之外，避免自扫。

> 语言、收藏、项目置顶、扫描计划、检测器配置、guard 设置等运行时选项通过 Web 界面管理，
> 并持久化到 SQLite。无需手动编辑 YAML。

只有启动引导字段需要放在 config.yaml 中：

| 字段 | 类型 | 说明 |
|---|---|---|
| `bind` | string | 绑定地址。默认 `127.0.0.1`；非 loopback 需 `allowed_cidrs`（或 `--i-know-its-risky`）。 |
| `port` | int | `0` = 随机临时端口。 |
| `allowed_cidrs` | []string | IP 白名单；非 loopback 绑定必填。 |
| `basic_auth` | object | `user` + bcrypt `password_hash`。 |
| `home_dir` | string | 覆盖 `$HOME` 用于发现（调试）。 |
| `claude_dir` | string | `.claude` 根目录绝对路径；空 = `<home>/.claude`。 |
| `discovery.disabled_asset_types` | []string | 发现时跳过的资产类型。 |
| `backup_dir` | string | 备份根目录；空 = `~/.code-agent-sentinel/backups`。 |
| `max_backups` | int | `0` = 默认 20。 |
| `log_path` | string | 日志文件路径；空 = stderr。 |
| `token` | string | 预设访问 token；空 = 启动时随机生成。 |
| `known_projects` | list | `{path, name}` 条目；`setup` 从 `~/.claude.json` 自动导入。 |
| `agents` | list | Agent 定义（`id`/`enabled`/`root_dir`/`claude_json`）；`setup` 填充。 |

示例：

```yaml
bind: 127.0.0.1
port: 0
language: en
claude_dir: /home/me/.claude
discovery:
  disabled_asset_types: [scripts]
```

## 命令行参考

所有子命令均可用 `--config` 覆盖配置路径。

| 命令 | 用途 |
|---|---|
| `code-agent-sentinel` | 启动本地 SOC 看板 server（默认）。 |
| `code-agent-sentinel scan` | 一次性扫描（发现 → 扫描 → 写历史），不启 server。 |
| `code-agent-sentinel guard` | 运行时拦截 hook（由 Claude Code `PreToolUse` 调用）。 |
| `code-agent-sentinel setup` | 交互式 agent 配置。 |
| `code-agent-sentinel uninstall` | 清理 `~/.code-agent-sentinel/`，不碰 `~/.claude`。 |
| `code-agent-sentinel baseline` | `--create` 批量接受 finding；`--prune` 清理失效条目。 |
| `code-agent-sentinel rules` | `list` 打印规则列表；`validate` 校验规则文件。 |
| `code-agent-sentinel service` | `install`/`uninstall`/`status` systemd 服务。 |

## 安全模型

- **默认仅 loopback**：`bind` 默认 `127.0.0.1`。非 loopback 绑定须有非空 `allowed_cidrs`（或 `--i-know-its-risky`）。
- **token 经 URL fragment 传递**：随机 token 通过 `#token=` 下发 —— 不进 server 日志、不进 `Referer` —— 每个 API 请求校验。
- **Host 头 + 严格 CORS**：防 DNS rebinding。
- **非 loopback 不自动开浏览器**：避免多用户主机上通过 `xdg-open` argv 泄露 token。
- **优雅降级**：缺失 `gitleaks`/`govulncheck`/`npm` 时检测器标记 `unavailable`，整体扫描继续。
- **范围明确的卸载**：`code-agent-sentinel uninstall` 仅删 `~/.code-agent-sentinel/`；Claude Code 配置与二进制不受影响。

## 开发

```bash
make build          # web + Go 二进制 -> bin/code-agent-sentinel
make test           # go test ./...
make run            # build 后运行
make web            # 仅构建前端
make web-install    # cd web && npm install
make clean          # 删除 bin/、web/dist
make release        # 交叉编译归档
```

前端 e2e：`cd web && npm run test:e2e`（Playwright）。

技术栈：**Go 1.25**（Gin + cobra + embed）+ **React 18 / Vite / TypeScript /
antd v5 / zustand / monaco-editor / react-i18next**。单二进制分发 ——
React 构建产物 embed 进 Go 二进制。
