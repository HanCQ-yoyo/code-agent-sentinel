# 发布记录(Code Agent Sentinel)

> 本文件记录 Code Agent Sentinel 每个大版本迭代的**系统功能升级**与**问题修复**。
>
> 版本以**功能里程碑**命名(不引入语义版本号 / git tag),条目按合入时间倒序排列(最新在上)。
>
> **维护约定**:每次将开发分支合并到 `main` 前,必须同步更新三处文档——
> [`README.md`](README.md)(英文)、[`README.zh-CN.md`](README.zh-CN.md)(中文)、本 `RELEASE.md`。
> README 两份反映能力 / 配置 / 命令的最新状态;RELEASE.md 追加一条版本条目(升级 + 修复)。
>
> 合入 SHA 取自 `git log main`,历史几次合并为 fast-forward(无 merge commit),则以里程碑末次提交 SHA 标注。

---

## 运行时风险指令拦截(Stage R2:Claude-only 最小版)

- **合入日期**:2026-07-28(分支 `feat/dcg-stage-r2-runtime-intercept`,merge `824ef55` 到 main)

### 升级

- **`sentinel guard` 运行时拦截 hook**:作为 Claude Code `PreToolUse` Bash hook 运行,对单条 shell 命令跑 7 步管线(解析 → 递归短路 → quick-reject → normalize 反混淆 → heredoc 内联脚本提取 → pack 评估 → 决策输出+记录),实时 deny 破坏性命令(`rm -rf /`、`git reset --hard`、`sudo rm`、`$'\x72\x6d'` ANSI-C 编码、`bash -c "rm -rf /"` 内联脚本等)。fail-open 铁律:hook 永远 `exit 0`,deny 仅靠 stdout JSON 表达;解析失败 / 超时 / panic → allow 或 ask(超时)。
- **反混淆状态机**(手写,参考 dcg `normalize.rs`,不引 shell parser):剥 sudo/env/command/exec/nohup/time/反斜杠 wrapper(迭代 ≤32)+ ANSI-C `$'\xNN'` 解码(仅 executable position,不动数据区)+ 去引号 + 路径展开(`/usr/bin/git`→`git`)。`command -v`/`-V` 查询模式不剥。
- **quick-reject 关键词快速放行**:每域手工声明在规则 `metadata.keywords`(`destructive_commands.yaml` 5 域 189 条规则),命中关键词进入精检,未命中且无混淆字符放行;空关键词列表保守不 reject(防漏放行);混淆字符(`\ ' "`)回退 normalize 重判。
- **heredoc Tier1/2 提取 + 手写分段递归**:17 个 trigger 正则 + `<<` 引号感知扫描(零假阴性);提取 interpreter `-c`/here-string/`$()` 内层命令,手写分段(`$()`/`;`/`&&`/`||`/`|`)递归(深度上限 8,砍 AST)。
- **规则库单一来源**:guard 合成 `configengine.Asset{Type:AssetCommand}` 走与静态 `RulesDetector` 同构的 `DispatchCommand → Eval` 路径,复用 `ruleengine.LoadBuiltin()`(`//go:embed`),绝不复制规则。
- **`sentinel setup` / `uninstall` 装/卸 hook**:`setup` 自动把 `sentinel guard` 注册到 `~/.claude/settings.json` 的 `hooks.PreToolUse`(matcher=`Bash`,sentinel 置首,幂等 basename 精确匹配);`uninstall` 反向移除(幂等)。
- **GuardConfig 配置段**(`internal/config/guard.go`,与 Detectors 平级):`enabled`/`policy`/`deadline_ms`/`max_command_bytes`,持 `sync.RWMutex`,`PUT /api/guard/config` 原地 `ApplyFrom` + 写盘热生效;hook 子进程每次 `config.Load` 读盘。
- **拦截记录存储**(`internal/intercept` 包,镜像 history):`InterceptRecord` JSON 文件(`~/.claude-sentinel/intercept/<id>.json`,原子写),`AgentProtocol="claude"` 命名空间(不复用 history/scheduler 的 AgentID)。
- **API**:`GET/PUT /api/guard/config`(全键校验防部分体静默禁用)、`GET /api/intercept`、`GET /api/intercept/:id`、`DELETE /api/intercept/:id`。
- **前端 Intercept 只读页**(`/intercept`):列表(时间 / 决策 / 命令 / 规则 / 严重度 / 耗时)+ 详情抽屉 + 按决策筛选 + 删除;复用 zustand store slice + i18n 中英字典。

### 修复

- 无(新功能分支,无既有缺陷修复)。

---

## 治理基础(资产能力看板 + FP 减负 + 统一处置生命周期 + 细粒度筛选 + 检测任务完善)

- **合入日期**:2026-07-27(分支 `feat/governance-foundation`,待合并后回填 main SHA)

### 升级

- **资产能力看板**:`CapabilityPanel` 结构化展示 allowed-tools / hook 事件 / mcp 命令 / memory 大纲,替代"仅一行 description"的旧视图;资产发现页加定位文案。
- **FP 减负**(三机制叠加):
  - **否定上下文抑制**:`ruleengine.IsNegatedByContext` 识别"禁止/不允许"前缀,否定语义命中不再当 finding。
  - **资产内去重**:emit 流水线按位置聚合同位置多规则命中,`ContributingRuleIDs` 记录所有触发规则,单一 finding 承载多规则上下文。
  - **聚合视图**:FindingTable 双视图(按 finding / 按资产聚合),资产维度集中查看风险。
- **统一处置生命周期**:塌缩旧 `baseline.json` + `suppressions.yaml` 为单一 `finding_states.yaml` overlay(`findingstate` 包:Status / Priority / Note / Category / ContributingRuleIDs);`applyFindingState` 在扫描期按 fingerprint 注入处置状态;`sentinel baseline --create` 改为 `BulkAccept`(批量接受全部当前未处置),`--prune` 改为打印 `PruneReport`。
- **细粒度筛选**:FindingTable 按 Category / Status / Priority / asset-type 多维度筛选;Category 派生(`ruleengine.CategoryOf`)贯穿检测器 → API → UI。
- **检测任务完善**:History 页 per-agent 视图(每 agent 独立最近扫描列表)+ 检测范围/目标列(`ScanSummary.ScopePath`,`global` / `project:<path>` / `asset:<id>`)。

### 修复

- **旧 baseline/suppressions 自动迁移**:首次启动时将 `~/.claude-sentinel/baseline.json` 与 `suppressions.yaml` 合并为 `finding_states.yaml`,旧文件重命名为 `.legacy`(不删,留作回滚);迁移幂等(已有 finding_states.yaml 时跳过)。
- **健康分 Status 单调**:`Status=accepted` 的 finding 不再参与 R(asset) 求和(修旧 baseline accepted finding 仍拉低分数的健康分失真)。
- **emit 流水线 fingerprint 稳定性**:同位置多规则去重时复用首条 finding 的 fingerprint(避免重复 hash 计算,确保跨扫描可还原)。

---

## Coding Agent 配置对齐(双 agent 资产发现 + credential/combo/managed-mcp 规则)

- **合入日期**:2026-07-27(分支 `feat/coding-agent-config-alignment`,待合并后回填 main SHA)
- **合入 SHA**:`472837e`(merge commit)

### 升级

- **Codex 配置根分流**:`Engine` 按 agent spec 选择 `ClaudeDir`/`CodexDir`,修复 Codex 误用 `~/.claude.json` 根因(C1);Codex 项目级发现改读 sentinel 独立 `known_projects`,不再借 `~/.claude.json`(C1)。
- **known_projects 独立清单**:`config.KnownProjects`/`KnownProject`/`ResolveKnownProjects`;`setup` 从 `~/.claude.json` projects 导入初始值(纯函数 `importKnownProjects`,文件缺失/JSON 损坏安全降级),用户可手改 `config.yaml` 增删。
- **Claude 发现补齐(L1-L5)**:`managed-mcp.json`(企业模式提示)、全局 `.mcp.json`、项目 `hooks/` 目录、项目 `keybindings.json`、`settings.json` 的 `skip_dangerous_mode_permission_prompt` 字段。
- **credential 资产类型(L6)**:`~/.aws/credentials` / `.env` / `.netrc` 等凭据文件建模为只读元数据资产(Content 不暴露明文);Codex `auth.json`(C4)+ 项目级敏感文件。
- **Codex 项目级 .codex/config.toml(C2/C3)**:项目级 Codex 配置发现 + `[hooks.state]` 建模。
- **结构化字段规则迁移**:settings/skill/agent 等结构化字段改走 `field:` op,精简匹配树(从 content 正则迁至字段精确匹配)。
- **MCP 明文/managed-mcp 规则**:全局 `.mcp.json` `http://` URL 检测 + `managed-mcp.json` 企业模式提示。
- **跨资产组合规则(6 条)**:`ComboRule` 类型 + 加载 + 校验;`RulesDetector` 第二遍跨资产组合求值。覆盖 skip-perm-combo / bash-wildcard + WebFetch(*) / 凭据外发 / Codex danger+never 等组合 critical finding。

### 修复

- Codex 项目级发现误借 `~/.claude.json`(C1):改读 `Engine.KnownProjects`(从 sentinel config 桥接)。
- Codex 配置根误用 `~/.claude.json`(根因):`Engine` 按 agent spec 分流 `ClaudeDir`/`CodexDir`。

---

## Codex CLI 支持

- **合入日期**:2026-07-24
- **合入 SHA**:`2438ee1`(main,fast-forward,7 commits)

### 升级

- 新增 OpenAI Codex CLI 配置资产支持(config.toml / AGENTS.md / prompts / hooks.json)
- setup 自动探测 ~/.codex/config.toml,可勾选启用
- 多 agent 聚合:claude-code 与 codex 并存,各自独立扫描
- 新增 2 条 Codex baseline 规则(danger-full-access / approval-never)

---

## DCG 规则引擎搬运(危险命令语义解析 + 252 条规则)

- **合入日期**:2026-07-23 ~ 2026-07-24
- **合入 SHA**:`98de9d5` → `48b57b7`(main,fast-forward,33 commits)

### 升级

- **统一规则引擎重构**(`internal/security/ruleengine`):规则 schema 类型化(11 个 op 枚举)+ 加载校验器(schema + 正则编译 + match 树)+ match 树求值器(11 op + and/or/not + content 保留字段 + deobfuscation)+ 指纹算法(锚定规则意图,确定性)。
- **反混淆增强**:base64 多块越界修复、wrapper-strip 与 ANSI-C 解码、`regexp2` 分流编译层(支持 lookahead/lookbehind/反向引用,RE2 不兼容特性拒绝并附测试)。
- **命中位置**:`Eval` 返回 `Location`(仅 content 字段叶子算位置),OR 失败兄弟路径不污染 Locations(消除过度高亮)。
- **DCG 危险命令规则搬运**(Go 原生重写,源自 references/dcg,252 条):
  - **filesystem 域**(26 dest + 32 safe→post_exclude)、**git 域**(12 dest + 6 safe)、**database 域**(mysql/mongodb/postgresql/redis/sqlite/snowflake/supabase/mariadb,112 条)、**containers 域**(docker/podman/compose,21 条)、**package_managers 域**(18 条)。
  - **语义解析器**(Go 重写,非纯正则):snowflake SQL lexer(5 状态机排除注释/字符串)、filesystem `rm`(flag 扫描 + interactive 判定 + 管道 stdin)、core.git 语义解析器(子命令识别 + 数据区降级)。语义 finding 按 `dcg_rule_id` 精确匹配承运规则。
- **post_exclude 遍历**:遍历全部匹配而非仅最左匹配;filesystem 补 `..` 路径遍历防护 + 去 `$TMPDIR` 过度包含。

### 修复

- `C1 漏报`:语义 Safe 改按行 span-scoping;snowflake `UPDATE <table> SET` 与 `TRUNCATE <table>` 形式补全。
- `C2 severity 失真`:snowflake 返回具体 `dcg_rule_id`;语义 finding 按 `dcg_rule_id` 精确匹配承运规则(修 severity 健康分失真)。
- `rm --interactive=never` 误判 Safe(=force 非 interactive)修正;git 语义解析器剥离 `-c` 配置覆盖 flag(修 `reset --hard` 漏报)。
- `asset_type=hook` 路由缺口:destructive 规则覆盖 command 类资产。

---

## Multi-Agent 运行时扫描守护进程(四支柱)

- **合入日期**:2026-07-23
- **合入 SHA**:`98de9d5`(main,fast-forward,分支 `feat/multi-agent-runtime-scan-daemon`,33 commits)

### 升级

- **支柱 1 全页面 multi-agent**:`engineForQuery(c)` 路由 9 只读 handler 走选中 agent Engine;`latestScan(agentID)` + `Store.LatestForAgent`;前端各页 `?agent=`;Dashboard 多圆圈 + 多线趋势 + `AgentMultiSelect` 筛选器;Assets 两级 tab(agent L1 + global/projects L2);Findings/History 加 Agent 列。
- **支柱 2 运行时扫描开关**:`Manager.Paused` atomic 闸门 + `applyScanToggle` 传播 `scan_enabled`→`SetPaused`(删 dead `Server.Scheduler` 字段);前端 Settings 总开关 + `RescanModal` 多选 agent。
- **支柱 3 重扫描**:`ScanScope{Type,Path}` + `RunScan(scope)`;`ScanRecord.Scope`;`LatestForAgent` 优先 global scope(防 asset-scope 重扫污染 dashboard);`partialRescan` 改走 Runner 抽象;`POST /api/scan?scope=&path=`;`RescanModal`(选范围/agent/检测器)+ 页面级入口(`store openRescan/closeRescan` + `initialScope` 预填)。
- **支柱 4 守护进程**:`config.Token`(三级优先级 `--token`>cfg.Token>genToken);`serveHTTP` signal/graceful shutdown;`internal/service` 三平台单元生成器(stdlib 叶子);`sentinel service install/uninstall/status`;`--daemon` 后台启动 flag + 跨平台 self-fork;`--log-path` 日志路径 flag + config.LogPath。
- **资产级安全检查**:资产详情页安全检查 Modal(只配检测器,`getContainer={false}` 修 z-index);`AssetDetailPanel` 三调用点透传 agentID;agent 展示改用 Claude Code 品牌 logo(`AgentIcon`)。
- **安全检测文案双语**(i18n)。

### 修复

- **Windows `sc.exe create` 引号**:给含空格 `ExePath` 加引号(修 Program Files 静默失败)。
- **`partialRescan` 跨 agent prior**:dedup 按 agent 取 prior(深层根因:`RunScan.saveHistory` 须归一化 agentID 空→首 agent,否则 `latestScan(agentID)` 返 nil)。
- **Findings 切 agent 陈旧**:`App.fetchLatestScan` 加 `selectedAgent` dep 修 Findings 切 agent 陈旧;回退 agent 扫描开关 + Assets L1 始终显示。
- **重扫后刷新**:`runScan` 重扫后刷新 Findings/LatestScan(修 Findings 陈旧 + Assets 风险徽章消失)。
- **structured 资产 Content 泄漏**:`structured` 资产带 Content 展示原文件(修 ContentArea 泄漏)。
- **`AssetDrawer` 透传 agentID** + 清理 `scanOff` 死键 + e2e 注释。

---

## P3 安全检测增强(统一规则引擎 + 抑制/baseline + 发现层 + 检测器配置 + Dashboard 重设计)

- **合入日期**:2026-07-13 ~ 2026-07-14
- **合入 SHA**:`3c6ea2b` → `107a814`(main,fast-forward)

### 升级

- **统一规则引擎**(63 条内置规则)+ **提示注入扫描**(含反混淆)+ **密钥扫描**(gitleaks)+ **依赖漏洞**(govulncheck / npm-audit)。子进程缺失时优雅降级(`unavailable`)。
- **抑制与 baseline**:`suppressions.yaml` 静默已知 finding;`baseline.json` 快照已接受指纹(`sentinel baseline --create/--prune` 或 API)。
- **发现层补齐**:项目根 `CLAUDE.md` 与 `CLAUDE.local.md` 发现;skill frontmatter `allowed-tools` 字段解析。
- **检测器运行期配置**:`Enabled()` 接口与三态(已禁用/不可用/可用);`GET/PUT /api/detectors/config` 端点;设置页检测器配置 UI(启用开关 + 二进制路径,三态着色);规则详情补齐 9 字段(资产类型/修复/路径/元数据/来源等)。
- **Dashboard 重设计**:分层看板(资产统计/检测器对齐/Top 风险/趋势图)+ 共享 `DetectorPanel`;收藏后端持久化;文件树 md/json 预览。
- **i18n 默认英文** + 切换后刷新保留(修刷新不回退 bug,根因 `i18n.init` 显式 lng 跳过 detection);规则/检测器名双语走前端字典 `lib/i18n-names.ts`。
- **CLI `sentinel setup`**:交互式配置 code agent。

### 修复

- 检测器配置 struct 补 `json` tag 修设置页白屏(既走 YAML 又经 API JSON 暴露的结构体必须有 json tag)。
- `final-review` 修复:Dashboard 扫描后刷新 + 部分检测器配置拒绝;规则详情字段同步。

---

## P2 安全写编辑闭环

- **合入日期**:2026-07-10
- **合入 SHA**:`e0b16b3`(main,merge commit,分支 `feat/p2-write-edit`)

### 升级

- **配置编辑**:原子写入 + 自动备份与迁移(新建 `internal/editor` 写层);**configengine 保持只读**(P2 的写编辑、P4 的动态检测都建在 configengine 之上,configengine 刻意保持纯净可复用)。

---

## 导航品牌 + 面包屑 + 历史风险指数 + 检测器规则合并视图

- **合入日期**:2026-07-09
- **合入 SHA**:`b262a5d`(main,merge commit,分支 `feat/nav-breadcrumb-history-rules-refinement`)

### 升级

- 导航品牌 + 面包屑;历史风险指数;检测器规则合并视图。

---

## 资产树收起 + 发现页列重排与详情抽屉

- **合入日期**:2026-07-09
- **合入 SHA**:`6ca2a5f`(main,merge commit,分支 `feat/findings-drawer-and-tree-collapse`)

### 升级

- 资产树收起;发现页列重排;发现详情抽屉。

---

## UI 重构阶段 A–D(antd v5 迁移 + Monaco + markdown 预览 + 三大特性增强)

- **合入日期**:2026-07-07 ~ 2026-07-09
- **合入 SHA**:`cbf0367` → `a43bd89` → `6b7b3c7` → `81ca637` → `b81d19d`(main,merge commits,分支 `feat/ui-antd-stage-a` ~ `feat/ui-antd-stage-d`)

### 升级

- **阶段 A**:antd v5 迁移基线。
- **阶段 B**:agent 抽象 + 文件树重构。
- **阶段 C**:Monaco 高亮 + markdown 预览 + 信息重排。
- **阶段 D**:打磨收尾 + 三大特性增强(命中位置高亮 / 项目置顶 / 自定义 `.claude` 目录)。
- **前端终审修复**(2026-07-03):e2e 认证化 / 错误渲染 / gitignore / clean。

---

## P1 只读安全管控面

- **合入日期**:2026-07-03
- **合入 SHA**:`4b8b05d`(main,merge commit)

### 升级

- **资产发现与解析**:扫描 `~/.claude/` 与项目 `.claude/`,覆盖 settings、permissions、hooks、MCP servers、skills、commands、agents、plugins、CLAUDE.md/memory、keybindings、scripts 等 11 类资产。
- **安全检测**(4 检测器,`Detector` 接口 + `Registry` + `Scan` 编排器):基线(Go 原生 + 内嵌 YAML 规则)、提示注入(Go 原生 + 反混淆)、密钥(子进程 gitleaks)、依赖(子进程 govulncheck / npm-audit)。
- **健康分**:`Score = 100 × (1 − Σ(R(asset)·w(asset)) / (Rmax · Σ w(asset)))`,Rmax=10,0–100 五档——可解释、单调、可还原。权重表是 `internal/security/health.go` 里的显式常量。
- **本地服务安全**:默认 bind `127.0.0.1`;非 loopback 必须有非空 `allowed_cidrs` 否则拒绝启动(除非 `--i-know-its-risky`);token 经 URL fragment `#token=` 传递(不进 server log / Referer);严格 CORS + Host 头校验防 DNS rebinding。
- **错误约定**:API 返回 `{error: {code, message, details?}}`;资产文件解析失败不致全盘失败——该资产标记 `parse_error` 作为 Finding 暴露,扫描继续。
- **sentinel 自己的配置**放 `~/.claude-sentinel/config.yaml`(在 `~/.claude/` 之外,避免自扫 / 递归)。
- **后端终审修复**(2026-07-03):XFF / token / CIDR / `--token` / govulncheck。

### 修复

- 前端终审修复:e2e 认证化 / 错误渲染 / gitignore / clean。
