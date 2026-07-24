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
