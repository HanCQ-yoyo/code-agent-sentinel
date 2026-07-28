# Code Agent Sentinel

> 把 Claude Code 的配置资产(settings、hooks、MCP servers、skills、commands、agents、plugins、CLAUDE.md/memory、keybindings、scripts)当作安全管控面来发现、解析、检测、编辑与监控的**本地单二进制安全看板**,并给出可解释的健康分。

[English](README.md) | 中文

## 核心能力

- **资产发现与解析**:扫描 `~/.claude/` 与项目 `.claude/`,覆盖 settings、permissions、hooks、MCP servers、skills、commands、agents、plugins、CLAUDE.md/memory、keybindings、scripts、**credential**(凭据文件 `auth.json`/`.env`/`*.pem` 等,不暴露内容)等 12 类资产。支持多种 code agent:**Claude Code**(`~/.claude/`)与 **OpenAI Codex CLI**(`~/.codex/config.toml`、`AGENTS.md`、`prompts/`、`hooks.json`)。`sentinel setup` 自动探测已安装 agent;看板支持多 agent 聚合、各自独立扫描。
- **发现补齐与跨资产组合规则**:Claude L1-L5——`managed-mcp.json`(企业模式提示)、全局 `.mcp.json`、项目 `hooks/` 目录、项目 `keybindings.json`、`skip_dangerous_mode_permission_prompt` 字段;Codex C2/C3——项目级 `.codex/config.toml` + `[hooks.state]` 建模;Codex C4/L6——`auth.json` 凭据 + 项目根敏感文件。**6 条跨资产组合规则**(skip-perm+Bash(*) / Codex danger+never / 凭据外发等)经统一规则引擎新增 `ComboRule` 第二遍求值。Codex 项目级发现改读 sentinel 的 `known_projects` 清单(独立于 `~/.claude.json`)。
- **安全检测**:统一规则引擎(256 条内置规则 + 6 条跨资产组合规则)+ 提示注入扫描(含反混淆)+ 密钥扫描(gitleaks)+ 依赖漏洞(govulncheck / npm-audit)。子进程缺失时优雅降级。
- **统一处置生命周期**:塌缩旧 `baseline.json` + `suppressions.yaml` 为单一 `finding_states.yaml` overlay(`findingstate` 包:Status / Priority / Note / Category / ContributingRuleIDs)。`sentinel baseline --create` 批量接受全部当前未处置 finding;`--prune` 打印清理报告。旧文件自动迁移(重命名 `.legacy`,不删除)。已接受 finding 不再拉低健康分。
- **健康分**:`Score = 100 × (1 − Σ(R(asset)·w(asset)) / (Rmax · Σ w(asset)))`,Rmax=10,0–100 五档,可解释 / 单调 / 可还原。
- **配置编辑**:原子写入 + 自动备份与迁移(`internal/editor`);configengine 保持只读。
- **定时扫描**:进程内 scheduler(`scan_interval` / `scan_enabled`)持续刷新历史;`sentinel scan` 一次性扫描不启 server。
- **自定义 `.claude` 目录**:`claude_dir` + `discovery.disabled_asset_types` 指向自定义配置根、跳过不关心的资产类型。
- **双语 UI**:界面 `zh` / `en` 切换(react-i18next,`language` 配置默认值);后端文案保持中文。
- **命中位置高亮**:规则 finding 携带 `Location{Line,StartCol,EndCol}`,在 Monaco 查看器中高亮。
- **资产能力看板**:结构化展示 allowed-tools / hook 事件 / mcp 命令 / memory 大纲,替代旧的单行 description。
- **FP 减负**:否定上下文抑制("禁止/不允许"前缀命中不再触发)+ 资产内去重(同位置多规则命中塌缩为单条 finding,`ContributingRuleIDs` 记录全部触发规则)+ 双视图 FindingTable(按 finding / 按资产聚合)。
- **检测任务视图**:History 页 per-agent 视图 + 检测范围/目标列(`ScanSummary.ScopePath`:`global` / `project:<path>` / `asset:<id>`)。
- **运行时风险指令拦截(Claude + Codex)**:`sentinel guard` 作为 Claude Code 与 OpenAI Codex CLI 的 `PreToolUse` Bash hook 运行,对每条 shell 命令跑 span 感知管线(解析 → 递归短路 → quick-reject → normalize 反混淆 → heredoc 提取 → **链式拆分 + span 分类** → 规则引擎评估 → 决策+记录),实时 deny 破坏性命令(`rm -rf /`、`git reset --hard`、`git commit -m "x" && rm -rf /` 链式、ANSI-C 混淆、`bash -c "..."` 内联脚本等)。**span 分类器**(引号/注释/命令替换状态机)把命令文本切成 executed/data/comment 三类 span,破坏性正则只在 executed 区匹配,抑制数据区字面量误报(`echo "rm -rf /"` 不再误 deny)。**链式拆分器**按 `&&`/`;`/`||`/`|` 拆命令为独立片段,每片段独立评估(闭合 Stage R2 遗留的链式绕过缺口)。**置信度打分**(high/low/unknown,按命中落 span 位置)+ **安全模式**(`strict` 默认 = 不确定时 deny / `lenient` = 不确定时 ask;高置信度命中两模式都 deny)。**放行清单 allowlist**(独立 `allowlist.yaml`,精确双匹配 normalize 前后,不支持通配)命中的命令即便命中规则也放行。fail-open 铁律:hook 永远 `exit 0`,deny 仅靠 stdout JSON 表达。**Codex 协议适配**:`turn_id` 字段自动消歧 Claude/Codex;Codex 发最小三字段 deny payload(防 strict parser 拒扩展字段);低置信度 ask 退化为 deny。`sentinel setup` 自动把 hook 注册进 `~/.claude/settings.json` 与 `~/.codex/hooks.json`;决策记录(含 `confidence` + `matched_span`)落盘 `~/.claude-sentinel/intercept/` 并在 `/intercept` 页只读展示。经 `guard` 配置段(`enabled`/`policy`/`deadline_ms`/`max_command_bytes`/`mode`/`allowlist_enabled`,`PUT /api/guard/config` 热生效)调控;设置页暴露 GuardConfig 编辑面板 + Allowlist 编辑面板。
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
| `known_projects` | list | `{path, name}` 条目——sentinel 独立已知项目清单;`setup` 从 `~/.claude.json` projects 自动导入初始值。用于 Codex 项目级发现(及 Claude)。空 → Claude 回退 `~/.claude.json` projects。 |
| `dir_tags` | map | 按路径覆盖标签。 |
| `favorites` | []string | 收藏的资产 ID(后端持久化)。 |
| `backup_dir` | string | 备份根目录;空 = `~/.claude-sentinel/backups`。 |
| `max_backups` | int | `0` = 默认 20。 |
| `sentinel_rules_dir` | string | 全局自定义规则目录;空 = `~/.claude-sentinel/rules`。 |
| `finding_states_path` | string | 处置 overlay 文件;空 = `~/.claude-sentinel/finding_states.yaml`。旧 `baseline.json` / `suppressions.yaml` 首次启动自动迁移(重命名 `.legacy`)。 |
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
| `sentinel guard` | 运行时拦截 hook(由 Claude Code `PreToolUse` 调用)。读 stdin JSON,评估 Bash 命令,向 stdout 写 deny/allow 决策。永远 `exit 0`(fail-open)。Flags:`--config`、`--deadline`、`--debug`。通常由 `sentinel setup` 自动注册。 |
| `sentinel uninstall` | 清理 `~/.claude-sentinel/`(历史、备份、finding_states、规则)。**不**碰 `~/.claude` 与二进制。`--yes` 跳过确认;`--keep-config` 保留 `config.yaml`。 |
| `sentinel baseline` | `--create` 批量接受当前全部未处置 finding 写入 `finding_states.yaml`;`--prune` 打印不复现指纹的清理报告。 |
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
