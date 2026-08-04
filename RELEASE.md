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

## 拦截规则表收敛到 destructive_commands.yaml

- **合入日期**:2026-07-31(直接在 main 修复)
- **合入 SHA**:`31397f0`(main)

### 修复

- **拦截域只加载破坏性命令规则**:此前 `syncBuiltinRules` 给 intercept 域同步了与 detect 域相同的**全部** builtin 规则(baseline/injection/skill/destructive),导致运行时拦截表混入检测规则;`code-agent-sentinel guard` 的 fail-open 回退路径(`LoadBuiltin`)同样加载全部。修复:新增 `ruleengine.LoadInterceptBuiltin()` 只读 `rules/destructive_commands.yaml`(190 条 `destructive.*` 规则),`syncBuiltinRules` 给 intercept 域改用它,guard 回退路径同步改用。detect 域不受影响(仍加载全部 builtin 做静态扫描);用户自定义拦截规则(`/api/intercept-rules`)不受影响(`SyncBuiltin` 只清 source=builtin 的 stale 行,不碰 custom)。已部署 db 下次启动由 `deleteStaleBuiltin` 自动收敛(257 → 190,一次性数据影响,研发阶段接受)。

### 已知限制

- 纯后端规则加载路径修复,不改 API 契约、不改 db schema、不改前端。

---

## UI 设计一致性修复(token 收敛 / 详情表单 / 按钮规范 / 字号统一)

- **合入日期**:2026-07-31(分支 `feat/ui-consistency-fix`)
- **合入 SHA**:`19fec4d`(main,fast-forward,10 commits,9 任务 SDD + 整支 review 1 Important 已修)

### 升级

- **token 体系收口**:补齐缺失的 `--scope-*`(global 青 / project 绿 / managed 黄 / plugin 紫)与 `--warn-bg`(命中行琥珀底)token;`.hit-line` 琥珀色去重改 token;收敛 10 个旧别名 token(`--bg` / `--text` / `--accent` 等)到新 `--color-*` 并删 `index.css` 别名块,后续维护只认 `--color-*` / `--fs-*`。
- **配色统一到 OKLCH**:Badge 全量迁 `--color-*`,scope 色块浅深主题生效;antd 命名色 `<Tag color="blue/red/...">` 全部改为 inline style 引 token(`STATUS_COLOR` / `PRIORITY_COLOR` 存 token 字符串),消除与 Badge 的双轨配色;FindingDrawer 的 `TAG_HEX` 裸 hex + `#fff` 改 token。整支 review 补迁 3 处遗漏(MatchNodeRow 的 AND/OR/NOT 标签、RescanModal 扫描启用态、SettingsAllowlist 白名单启停态)。
- **详情表单两列布局**:FindingDrawer 与 RuleDrawer view 态 `<Descriptions>` 由单列改 `column={2}`,短字段左右排、长内容字段(evidence / metadata / match 树等)`span={2}` 整行上下排,消除窄抽屉单列长表堆叠。
- **按钮规范归一**:行内操作(表格 delete / edit / fork)统一 `type="text"` + 图标 + `aria-label`(仅图标);导航按钮加 `Arrow*Outlined`;RuleDrawer dashed `+` 号去冗余字面文本改 i18n;规范记入 `design.md` CTA voice。
- **消除单按钮独占行**:SettingsSchedules「添加定时任务」并入总开关 Card `extra`;Findings 的 AgentMultiSelect 并入 `.filter-toolbar`(空态无条件渲染,保留切 agent 能力);Settings 保存配置并入检测器配置卡。
- **字号统一**:79 处裸 `fontSize: 10/11/12/12.5/13/14/18` 迁移到 `--fs-*` token,同语义字号归一(RawFilePanel 标题 18 → 20 对齐 AssetDetailPanel;次级文字 12;微标签 11);antdTheme 全局 token 与 MonacoViewer API 数值豁免。

### 修复

- **Findings 空态切 agent 锁死**:Task 7 初版把 AgentMultiSelect 并入 FindingTable `toolbarPrefix`,因 FindingTable 仅在 `findings.length > 0` 时渲染,空态筛选器消失致选了零 finding 的 agent 后无法切回。修:回退 toolbarPrefix,Findings 无条件渲染 `.filter-toolbar` 内的 AgentMultiSelect。

### 已知限制

- 纯前端 token / 样式 / 布局改动,不碰 Go 后端,不改 API 契约与业务逻辑。
- `design.md` 是本地 gitignored 文件,token 漂移回写与 CTA voice 规范仅本地留存,不进版本库。

---

## 产品打磨(8 区域 UI/UX + 后端读路径修复)

- **合入日期**:2026-07-30(分支 `feat/product-polish`)
- **合入 SHA**:`7bfae60`(main,merge commit,25 commits,23 任务 SDD + 整支 review 0 Critical/0 Important,可修 Minor 已修)

### 升级

- **术语统一**:UI 全量「检测→扫描」改名(保留「检测器」名词——指 detector 实体)。涉及导航、tab、按钮、列表表头、空态等,与既有「扫描任务」「扫描配置」命名一致。
- **安全检查按钮配色**:TopBar 与资产详情的安全检查按钮在暗色主题改 default 描边(非实色 primary),与页面其他操作按钮视觉层级对齐。
- **Dashboard 打磨**:检测器关停态圆点改 token 灰 + 描边(暗色可见);风险趋势图 hover 圆点显浮层(时间 + 分数),命中区放大。
- **风险管理列表重排**:8 列布局(名称 / 资产 / 级别 / Agent / 风险类型 / 处置状态 / 扫描时间 / 操作);资产列加资产类型 Tag + 完整路径 tooltip;处置状态 / 优先级配色统一;优先级筛选加宽;平铺聚合移左,聚合行内可点直达。
- **风险详情处置弹框化**:风险详情的处置从抽屉内联改弹框(Modal),加「立即处置」按钮态 + 状态 / 优先级配色;列表操作列可触发处置弹框。修处置后列表不更新的 bug——`/api/findings` 读路径 apply finding-state 并附 `started_at` / `source_path`(见修复节)。
- **检测任务视图打磨**:列表删除按钮加文字(暗色可见);详情页仅显当前 agent 健康分卡 + 顶部兄弟快捷入口上移。
- **拦截记录页打磨**:页面改名「拦截记录」+ 字号对齐其他页;列表去耗时列、加处置列(处置弹框 + 加拦截名单 / 放行名单,各带确认);详情抽屉加宽 + 表单化 + 时间上移 + 规则背景分组。
- **设置页打磨**:规则配置 tab 改名「扫描配置」;拦截配置 + 放行清单合并为一个 tab + 子 tab(拦截规则 / 白名单);拦截总开关 + 确认流程 + 高级弹框(安全模式 / 白名单开关 / 评估预算 / 命令长度上限,即现有 GuardConfig 字段的 UI 暴露);白名单子 tab 顶部展示启停状态。

### 修复

- **处置后风险列表不更新**:`/api/findings` 原读路径不 apply finding-state,处置一条 finding 后列表不立即反映新状态(需重扫)。修:`handlers_health.go` 在读路径调用 `ApplyFindingStateBatch` 重算 `suppressed`,并附 `started_at` / `source_path`(由 `Finding` 新增 `StartedAt` / `SourcePath` 字段携带)。Task 13 补 `/api/findings` 聚合路径(`?agent=all`)的 apply state 断言,闭合单路径覆盖缺口。
- **DispositionModal 防空指纹 finding**:列表 / 抽屉两处处置弹框入口对 finding 加 `fingerprint` 守卫(对齐 Drawer 路径),防空指纹 finding 写状态。
- **SettingsGuard 死代码**:拦截配置子 tab 的 `RuleDrawer` 原挂在 Tabs 内,antd Tabs 卸载非活动 pane 致抽屉不挂载;提升到页面级后 `SettingsGuard` 残留死代码,删除(字段已搬进设置页高级弹框)。

### 已知限制

- 术语改名仅 UI 层;后端包名 / 字段 / 日志仍用「检测」(detector / detect 等 Go 标识符不变),不影响 API 契约。
- 既有 `e2e.spec.ts` 的「项目 tab 右键置顶」测试在 main 上即 flaky(L1 agent tab 占 `.nth(0)` 致 `.nth(1)` 错位到全局 tab),本分支改按项目名文本定位项目 tab 缓解,非本分支引入。

---

## 规则结构化表单 + match 树编辑器

- **合入日期**:2026-07-29(分支 `feat/rule-form-match-tree`)
- **合入 SHA**:`3842580`(main,merge commit,11 commits,10 任务 SDD 全 review clean + 最终整支 review Ready to merge 0 Critical/0 Important)

### 升级

- **RuleDrawer edit/create 改全结构化表单(移除 Monaco YAML 编辑器)**:edit/create 模式从 Monaco YAML 文本编辑器替换为结构化表单——基础区(id / severity 带色点 / asset_type 12 项 / dotall 开关)+ match 树编辑器 + 高级折叠区(description / remediation / deobfuscation 6 技术 / post_exclude / paths include+exclude / metadata 动态 key-value)。`useDebouncedEffect` 数据源从 YAML 字符串改 draft 对象(防抖 500ms → POST /validate),`onSave` 序列化 draft 为 RuleDTO map。零后端 / 零 store / 零 RuleDTO 类型改动,复用 validateRuleDraft/saveRule/forkRule/useDebouncedEffect/SevBadge/ruleName。
- **match 树编辑器(递归 + 任意深度嵌套)**:新增纯函数模块 `web/src/lib/match-tree.ts`(树对象 ↔ RuleDTO.match map 双向转换 + 节点变换,零依赖独立单测)+ `asset-fields.ts`(asset_type→field 建议表)。组件 `MatchTreeEditor.tsx`(递归渲染整棵树)+ `MatchNodeRow.tsx`(单节点行:AND/OR/NOT 分组标题 + field AutoComplete + op 4 组下拉 + value 动态控件)。支持 and/or/not 节点 + field/op/value 叶子,节点可转换/删除/上下移,任意深度嵌套(逐层冒泡)。op 11 个分 4 类:value 控件随 op 契约动态切换(无 value / Tags 数组 / Input 标量,正则带 (?s) 提示)。对齐 `internal/security/ruleengine/schema.go` + `validate.go` 数据契约。
- **view 模式 match 摘要改树形只读渲染**:从 `JSON.stringify` 改为 `MatchTreeEditor readOnly`,内置规则的 and/or/not 结构以树形展示而非裸 JSON。
- **降级保护(无损)**:含特殊 op(repeat_check/homoglyph_check,非用户 op)或异形 match(混键等)的规则 → match 区降级为只读 JSON 块 + 警告,`matchMapToTree` 返回 null 为唯一真源;保存时原 matchMap 原样回写,不丢数据。
- **新增 vitest 纯函数单测 + Playwright e2e**:引入 vitest(`match-tree.test.ts` 往返/变换/降级,25 用例);`rules-form.spec.ts` e2e 覆盖 create 单叶子保存 / view 内置规则树渲染 / builtin view 只读核心路径。

### 修复

- **e2e create 测试不可重入**:POST /api/detect-rules 对已存在 id 返回 409(`handlers_rules.go:postRule`,create 重复 id 非幂等),create 测试写的 `custom.e2e-single-leaf` 持久化在 SQLite → 上次运行残留 → 本次保存 POST 409 → `waitForResponse(200)` timeout。修:`beforeEach` 先 DELETE 清残留规则,保证每次 create 从干净态出发(连续 3 次重跑验证确定性)。

### 已知限制

- 不暴露 repeat_check/homoglyph_check 特殊 op(内置规则用,走降级只读 JSON)。
- e2e 未覆盖 edit→AND 分组变换路径(brief 原列该覆盖但未给测试代码,本计划遵 brief 只交付 3 测试);edit 态变换经手动验收。
- 既有 `e2e.spec.ts` 的「项目 tab 右键置顶」测试预先存在失败(非本分支引入,与本特性无关)。

---

## 规则可配置化 + sqlite 存储迁移

- **合入日期**:2026-07-29(规则可配置化分支 `feat/rule-configurable-sqlite`)
- **合入 SHA**:`796bc3d`(main,fast-forward,25 commits,17 任务 SDD 全 review clean)

### 升级

- **规则存储迁 sqlite(WAL + 双域)**:`internal/security/ruleengine/storage.go` 用 sqlite(WAL 模式,文件 `~/.code-agent-sentinel/sentinel.db`,运行时强制 `0o600`)存检测/拦截两域规则。三表:`rules`(规则正文,`match_json`/`paths_json` 等以 JSON 文本列往返)/`overrides`(启停覆盖,JOIN 派生 `enabled`)/`combos`(跨资产组合规则)。`MatchNode` 导出 `Raw()`/`NewMatchNode()` 供持久层 map 往返,与 YAML 加载路径同构。文件路径与 db 路径规则等价性有专项测试(9 个 asset_type)。
- **启动迁移旧规则文件 + 双域 builtin 同步**:`main.go` 启动注入 db;`migrate.go` 把旧 `~/.code-agent-sentinel/rules/*.yaml` 旧文件规则迁入 db(对侧域标签修正 + 重命名语义);`SyncBuiltin` 把 embed 内置规则同步进两域 db(覆盖 builtin 行 + 报告孤儿 override 供审计)。
- **规则 CRUD + 启停 + fork + validate(检测/拦截对称)**:API 两域对称 16 端点(`/api/{detect|intercept}-rules` CRUD + `/:id/enabled` 启停 + `/:id/fork` builtin→custom + `/validate` 不落库校验)。`POST` 拒绝覆盖 builtin id(409,闭合"内置只读"绕过:UpsertRule ON CONFLICT 本可静默改 builtin 的 match/source)。`dtoToRule` 直接构造 `ruleengine.Rule` 经 `NewMatchNode(dto.Match)` + `Validate`(与 YAML 加载同路径,保等价)。
- **运行时热重载 + fail-open**:`RulesDetector` 持 db 引用,扫描时实时读 db(规则改动无需重启,修 Finding #5 旧文件缓存);`guard` 守卫读 db 拦截规则,db 故障 fail-open 回退 builtin(4 种故障子情况全覆盖:dbPath 空/Open 失败/List 失败/表空,corrupt db 经 Ping 失败不 panic)。combos 构造时预编译不热重载。
- **前端规则管理 + 域切换**:`RuleDTO` 统一类型(两域共用,`domain` 标识来源域);`store` 规则 actions(`fetchDetectRules`/`fetchInterceptRules`/`saveRule`/`toggleRule`/`forkRule`/`deleteRule`/`validateRuleDraft`);`RulesTable` 操作列(启停/来源筛选);`RuleDrawer` view/edit/create 三态 + builtin fork + 防抖实时校验(注:edit/create 的 Monaco YAML 编辑器在下一里程碑「规则结构化表单 + match 树编辑器」已替换为结构化表单);`Settings` 域切换(检测/拦截两域同表单)。e2e 覆盖启停/域切换核心路径。

### 修复

- 无(新能力分支,基于 main@857cdfe 增量;既有 Stage R3 问题已在 R3 版本条目记录)。

---

## 运行时拦截增强(Stage R3:span 感知 + I1 闭合 + Mode + allowlist + Codex 适配)

- **合入日期**:2026-07-29(Stage R3 运行时拦截增强分支 `feat/dcg-stage-r3-runtime-enhance`)
- **合入 SHA**:`8946ecf`(main,fast-forward,15 commits:13 任务 + Task1 panic fix + 不变量#4 Eval panic 兜底 fix)

### 升级

- **span 分类器(引号/注释/命令替换状态机)**:`ruleengine/span.go` 把命令文本切成 `executed`(实际执行区)/`data`(引号/heredoc 内容)/`comment` 三类 span,破坏性正则只在 executed 区匹配,抑制数据区字面量误报(`echo "rm -rf /"` 不再误 deny)。手写状态机(不引 shell parser),覆盖单/双引号、`$(...)` 命令替换、`${var}` 引用(已知限制:双引号 `$(...)` 嵌套不闭合按数据区处理,安全侧)。
- **链式命令拆分(I1 闭合)**:`ruleengine/split.go` 按 `&&`/`;`/`||`/`|` 链式操作符拆命令为独立片段,每片段独立评估。闭合 Stage R2 遗留的链式绕过缺口(`git commit -m "x" && rm -rf /` 现被 deny,R2 只看整条命令首段导致漏判)。替换 R2 粗版 `splitCommandSegments`,链式拆分器带引号/转义感知(不在引号内拆)。
- **置信度降级 + 用户可见安全模式(strict/lenient)**:`ruleengine/confidence.go` 按命中落 span 位置打分——`high`(命中落 executed 区)/ `low`(命中落 span 边界 → 降级)/ `unknown`。`GuardConfig.Mode`(`strict` 默认 / `lenient`)驱动降级:strict 模式不确定时 deny,lenient 模式不确定时 ask(询问用户)。高置信度命中两模式都 deny(安全不变量 #1)。
- **放行清单 allowlist(独立文件 + 精确双匹配)**:`allowlist.yaml` 独立文件存储(与 guard config 解耦),`AllowlistStore` 原子读写。`Matches` 精确命令匹配,normalize 前后双比对(原始命令 + 反混淆后命令),不支持通配(防 `rm -rf *` 通配放行漏洞)。命中的命令即便命中规则也放行;`allowlist_enabled` 开关控制管线是否做放行匹配。
- **静态层同治**:`RulesDetector` 静态检测器复用 span 分类 + 片段拆分(`split.go` + `span.go`),静态扫描命令类资产时同样按片段独立评估,与运行时 guard 走同一拆分/分类路径(I1 静态闭合)。
- **前端管控面前移**:`SettingsGuard` 编辑面板(Mode strict/lenient 单选 + 总开关 + 放行清单启用 + 评估预算 + 命令长度上限,`PUT /api/guard/config` 热生效)+ `SettingsAllowlist` 放行清单编辑面板(增删条目,`GET/POST/DELETE /api/guard/allowlist`)+ Intercept 页 confidence 列(Tag 着色 high=绿/low=橙/unknown=灰)+ matched_span 详情区(命中片段文本)。
- **Codex CLI 协议适配**:`code-agent-sentinel guard` 按 `turn_id` 字段自动消歧 Claude/Codex 协议(Codex payload 带 `turn_id`,Claude 不带)。Codex 发最小 deny payload(仅 `hookEventName`/`permissionDecision`/`permissionDecisionReason` 三字段,防 strict parser 拒扩展字段);低置信度 ask 退化为 deny(Codex 不发 ask,安全不变量 #5)。`code-agent-sentinel setup` 自动安装 `~/.codex/hooks.json` PreToolUse Bash hook(`InstallCodexHook`/`UninstallCodexHook`),与 Claude `~/.claude/settings.json` 并行。

### 修复

- 无(新功能分支,基于 Stage R2 增量;Stage R2 既有问题已在 R2 版本条目记录)。

---

## 运行时风险指令拦截(Stage R2:Claude-only 最小版)

- **合入日期**:2026-07-28(Stage R2 运行时拦截分支,merge `824ef55` 到 main)

### 升级

- **`code-agent-sentinel guard` 运行时拦截 hook**:作为 Claude Code `PreToolUse` Bash hook 运行,对单条 shell 命令跑 7 步管线(解析 → 递归短路 → quick-reject → normalize 反混淆 → heredoc 内联脚本提取 → pack 评估 → 决策输出+记录),实时 deny 破坏性命令(`rm -rf /`、`git reset --hard`、`sudo rm`、`$'\x72\x6d'` ANSI-C 编码、`bash -c "rm -rf /"` 内联脚本等)。fail-open 铁律:hook 永远 `exit 0`,deny 仅靠 stdout JSON 表达;解析失败 / 超时 / panic → allow 或 ask(超时)。
- **反混淆状态机**(手写,不引 shell parser):剥 sudo/env/command/exec/nohup/time/反斜杠 wrapper(迭代 ≤32)+ ANSI-C `$'\xNN'` 解码(仅 executable position,不动数据区)+ 去引号 + 路径展开(`/usr/bin/git`→`git`)。`command -v`/`-V` 查询模式不剥。
- **quick-reject 关键词快速放行**:每域手工声明在规则 `metadata.keywords`(`destructive_commands.yaml` 5 域 189 条规则),命中关键词进入精检,未命中且无混淆字符放行;空关键词列表保守不 reject(防漏放行);混淆字符(`\ ' "`)回退 normalize 重判。
- **heredoc Tier1/2 提取 + 手写分段递归**:17 个 trigger 正则 + `<<` 引号感知扫描(零假阴性);提取 interpreter `-c`/here-string/`$()` 内层命令,手写分段(`$()`/`;`/`&&`/`||`/`|`)递归(深度上限 8,砍 AST)。
- **规则库单一来源**:guard 合成 `configengine.Asset{Type:AssetCommand}` 走与静态 `RulesDetector` 同构的 `DispatchCommand → Eval` 路径,复用 `ruleengine.LoadBuiltin()`(`//go:embed`),绝不复制规则。
- **`code-agent-sentinel setup` / `uninstall` 装/卸 hook**:`setup` 自动把 `code-agent-sentinel guard` 注册到 `~/.claude/settings.json` 的 `hooks.PreToolUse`(matcher=`Bash`,sentinel 置首,幂等 basename 精确匹配);`uninstall` 反向移除(幂等)。
- **GuardConfig 配置段**(`internal/config/guard.go`,与 Detectors 平级):`enabled`/`policy`/`deadline_ms`/`max_command_bytes`,持 `sync.RWMutex`,`PUT /api/guard/config` 原地 `ApplyFrom` + 写盘热生效;hook 子进程每次 `config.Load` 读盘。
- **拦截记录存储**(`internal/intercept` 包,镜像 history):`InterceptRecord` JSON 文件(`~/.code-agent-sentinel/intercept/<id>.json`,原子写),`AgentProtocol="claude"` 命名空间(不复用 history/scheduler 的 AgentID)。
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
- **统一处置生命周期**:塌缩旧 `baseline.json` + `suppressions.yaml` 为单一 `finding_states.yaml` overlay(`findingstate` 包:Status / Priority / Note / Category / ContributingRuleIDs);`applyFindingState` 在扫描期按 fingerprint 注入处置状态;`code-agent-sentinel baseline --create` 改为 `BulkAccept`(批量接受全部当前未处置),`--prune` 改为打印 `PruneReport`。
- **细粒度筛选**:FindingTable 按 Category / Status / Priority / asset-type 多维度筛选;Category 派生(`ruleengine.CategoryOf`)贯穿检测器 → API → UI。
- **检测任务完善**:History 页 per-agent 视图(每 agent 独立最近扫描列表)+ 检测范围/目标列(`ScanSummary.ScopePath`,`global` / `project:<path>` / `asset:<id>`)。

### 修复

- **旧 baseline/suppressions 自动迁移**:首次启动时将 `~/.code-agent-sentinel/baseline.json` 与 `suppressions.yaml` 合并为 `finding_states.yaml`,旧文件重命名为 `.legacy`(不删,留作回滚);迁移幂等(已有 finding_states.yaml 时跳过)。
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

## 危险命令规则库(语义解析 + 252 条规则)

- **合入日期**:2026-07-23 ~ 2026-07-24
- **合入 SHA**:`98de9d5` → `48b57b7`(main,fast-forward,33 commits)

### 升级

- **统一规则引擎重构**(`internal/security/ruleengine`):规则 schema 类型化(11 个 op 枚举)+ 加载校验器(schema + 正则编译 + match 树)+ match 树求值器(11 op + and/or/not + content 保留字段 + deobfuscation)+ 指纹算法(锚定规则意图,确定性)。
- **反混淆增强**:base64 多块越界修复、wrapper-strip 与 ANSI-C 解码、`regexp2` 分流编译层(支持 lookahead/lookbehind/反向引用,RE2 不兼容特性拒绝并附测试)。
- **命中位置**:`Eval` 返回 `Location`(仅 content 字段叶子算位置),OR 失败兄弟路径不污染 Locations(消除过度高亮)。
- **危险命令规则库**(Go 原生重写,252 条):
  - **filesystem 域**(26 dest + 32 safe→post_exclude)、**git 域**(12 dest + 6 safe)、**database 域**(mysql/mongodb/postgresql/redis/sqlite/snowflake/supabase/mariadb,112 条)、**containers 域**(docker/podman/compose,21 条)、**package_managers 域**(18 条)。
  - **语义解析器**(Go 重写,非纯正则):snowflake SQL lexer(5 状态机排除注释/字符串)、filesystem `rm`(flag 扫描 + interactive 判定 + 管道 stdin)、core.git 语义解析器(子命令识别 + 数据区降级)。语义 finding 按 `rule_id` 精确匹配承运规则。
- **post_exclude 遍历**:遍历全部匹配而非仅最左匹配;filesystem 补 `..` 路径遍历防护 + 去 `$TMPDIR` 过度包含。

### 修复

- `C1 漏报`:语义 Safe 改按行 span-scoping;snowflake `UPDATE <table> SET` 与 `TRUNCATE <table>` 形式补全。
- `C2 severity 失真`:snowflake 返回具体 `rule_id`;语义 finding 按 `rule_id` 精确匹配承运规则(修 severity 健康分失真)。
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
- **支柱 4 守护进程**:`config.Token`(三级优先级 `--token`>cfg.Token>genToken);`serveHTTP` signal/graceful shutdown;`internal/service` 三平台单元生成器(stdlib 叶子);`code-agent-sentinel service install/uninstall/status`;`--daemon` 后台启动 flag + 跨平台 self-fork;`--log-path` 日志路径 flag + config.LogPath。
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
- **抑制与 baseline**:`suppressions.yaml` 静默已知 finding;`baseline.json` 快照已接受指纹(`code-agent-sentinel baseline --create/--prune` 或 API)。
- **发现层补齐**:项目根 `CLAUDE.md` 与 `CLAUDE.local.md` 发现;skill frontmatter `allowed-tools` 字段解析。
- **检测器运行期配置**:`Enabled()` 接口与三态(已禁用/不可用/可用);`GET/PUT /api/detectors/config` 端点;设置页检测器配置 UI(启用开关 + 二进制路径,三态着色);规则详情补齐 9 字段(资产类型/修复/路径/元数据/来源等)。
- **Dashboard 重设计**:分层看板(资产统计/检测器对齐/Top 风险/趋势图)+ 共享 `DetectorPanel`;收藏后端持久化;文件树 md/json 预览。
- **i18n 默认英文** + 切换后刷新保留(修刷新不回退 bug,根因 `i18n.init` 显式 lng 跳过 detection);规则/检测器名双语走前端字典 `lib/i18n-names.ts`。
- **CLI `code-agent-sentinel setup`**:交互式配置 code agent。

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
- **sentinel 自己的配置**放 `~/.code-agent-sentinel/config.yaml`(在 `~/.claude/` 之外,避免自扫 / 递归)。
- **后端终审修复**(2026-07-03):XFF / token / CIDR / `--token` / govulncheck。

### 修复

- 前端终审修复:e2e 认证化 / 错误渲染 / gitignore / clean。
