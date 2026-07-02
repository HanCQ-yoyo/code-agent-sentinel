# Code Agent Sentinel — P1 设计规格

**日期:** 2026-07-02
**状态:** 方向已确认;待编写实现计划
**范围:** 仅 Phase 1(P1)——延后的内容见下文「分阶段」

## 问题与定位

Vibe coding 类工具(以 Claude Code 为先)的配置分散在 `~/.claude/`(以及各项目的 `.claude/`)下的文件和目录中。这些资产——`settings.json` 权限、hooks、MCP server 的 tool 描述、skills、commands、agents、plugins、CLAUDE.md、scripts——直接决定了 agent 能做什么、什么内容会进入它的 prompt。一条危险权限、一段被投毒的 MCP tool 描述,或一个外发数据的 hook,都可能让开发者的机器和代码库沦陷。目前没有一个本地的、单页面的视图,把这些资产当作**安全管控面**来统一看待和跨资产检测风险。

**定位:** Claude Code CLI 的配置 / 插件 / 数据是**要治理的安全资产**。核心能力是**安全检测**;配置浏览既是入口,也是研发提效手段。本项目同时也是一个明确的学习载体——学习 Claude Code 的设计思想,以及 AI 安全产品的产品化。选 Claude Code 是因为它是作者日常用得最多的 harness;AI 安全是长期方向;在安全核心之外附带少量好用的研发提效功能。

**与参考项目的差异:**
- *Cross-Code Organizer (CCO)* 和 *Claude Code Studio* 都已经把「配置管理 + 安全扫描」做得很完整。Sentinel **不**追求对齐它们的广度。它以**安全态势作为一等公民、可量化、可解释的概念**(健康分 + 加权扣分 + 基线治理)为核心,配置管理只做到安全可见性所需的最小集外加少量提效功能,技术栈用 Go 单二进制而非 Node。
- UI **刻意独立**于两个参考项目(SOC 风格、高信息密度、语义色编码——见「前端」)。

## 目标(P1)

一个本地、单二进制工具 `sentinel`:发现并解析 Claude Code 的配置资产(全局 + 一个选中的项目),对这些资产跑静态安全检测(基线 + 提示注入 + 密钥/依赖),计算一个可解释的健康分,并呈现一个只读的安全态势看板。P1 **只读**——不做配置编辑。

## 非目标(P1)

- 配置编辑 / 写入(P2)
- 备份与迁移(P2)
- 内嵌 agent(更新 CLAUDE.md 记忆 / 问答)(P3)
- 会话历史管理(P3)
- 动态检测:实时连接 MCP、hash 基线、rug-pull / 变更监控(P4)
- 团队 / 基线规则外部化(P4)
- 多 harness 支持(Codex、Cursor……)——目前只做 Claude Code
- LLM-judge 检测(调用 LLM 判断恶意性)——P1 只做静态

## 分阶段

| 阶段 | 范围 | 状态 |
|---|---|---|
| **P1(本规格)** | 地基(配置发现 + 资产模型)+ 安全检测 v1(基线 + 静态注入 + 密钥/依赖)+ 只读看板骨架 + 只读配置浏览 | ← 现在 |
| P2 | 外科手术式配置编辑(diff 预览 + 自动备份)+ 备份/迁移 | 后续 |
| P3 | 内嵌 agent(CLAUDE.md 记忆更新 + 问答)+ 会话历史 | 后续 |
| P4 | 动态安全:实时取 MCP tool 定义 + hash 基线 + rug-pull/变更监控 + 团队基线规则外部化 | 后续 |

每个阶段各自走 spec → plan → 实现 的循环。

## 技术栈

- **后端:** Go 单二进制。Gin(HTTP API)+ cobra(CLI)+ `tidwall/sjson`/`gjson`(外科手术式 JSON 读写)+ `embed`(内嵌前端产物)。(fsnotify 是 P4 变更监控的依赖,P1 不用。)
- **前端:** React + Vite + TypeScript + Tailwind CSS + shadcn/ui + zustand。单独构建,通过 `embed` 打进二进制。
- **外部扫描器(子进程):** gitleaks(密钥)、govulncheck(Go 依赖)、npm-audit(skill/command/plugin 里的 JS 依赖)。semgrep 可选,用于对 scripts 做多语言 SAST。
- **分发:** 单二进制;`sentinel` 启动本地服务并打开浏览器。(对比参考项目的 `npx` 模式——单二进制对安全工具是优势。)

**不需要 Python:** 这里的检测是模式/规则/基线/静态分析驱动,不是 ML 推理。OSS 扫描器以子进程方式运行,与其自身语言无关。

## 架构

单个 Go 二进制 `sentinel`。启动:按配置选 bind 地址 + 端口、生成 per-session token、起 Gin API、打开浏览器到 `http://<bind>:<port>/#token=<token>`。

分层,依赖只向下不向上(镜像 Studio 的 core/server/web 分层,额外加一层 security):

```
cmd/sentinel          ← cobra:启动 / 端口 / token / 开浏览器(后续可加 `scan --json` 无头模式)
internal/api          ← Gin HTTP+JSON,服务内嵌 SPA,token + Host 校验 + bind 策略
internal/security     ← Detector 接口 + 注册表,4 个检测器,Scan 编排器,健康分
internal/configengine ← 纯逻辑:发现 + 解析 + 资产模型 + 只读查询(无副作用,可用 fixture 测)
internal/web          ← embed.FS,持有构建好的前端产物
web/                  ← React/Vite/TS 源码(单独构建,产物内嵌)
```

`configengine ← security ← api`。`configengine` 保持纯净、可复用(P2 写编辑、P4 动态检测都建在它之上)。

## 资产模型(configengine)

**发现范围(P1):**
- **全局(用户级):** `~/.claude/settings.json`、`~/.claude.json`(MCP server + 各项目状态;机器管理,只读)、`~/.claude/CLAUDE.md` + memory、`agents/`、`skills/`、`commands/`、`plugins/`(+ marketplace)、hooks(在 settings 内)、`keybindings.json`。
- **项目级(通过项目切换器选中):** `<proj>/.claude/settings.json`、`settings.local.json`、`.mcp.json`、`CLAUDE.md`、`agents/`、`skills/`、`commands/`。
- **托管 / 企业策略文件:** 存在时只读展示(它优先级最高;只看不改)。

**资产类型(安全管控对象):** Settings、Permissions(allow/deny/ask——因安全关键而单独列出)、Hooks(event→command)、MCP servers(name/scope/transport/command/url/env)、Skills、Commands、Agents、Plugins、CLAUDE.md/memory、Keybindings、被 hooks/commands 引用的 Scripts。

每个资产带:`id / type / scope(global·project·managed) / source_path / name / 解析字段 / mtime / hash`。

**优先级处理(P1):** 发现 + 解析 + scope 标注 + **重复检测**(同一 MCP 装两处)。完整的 effective-resolution(shadowed/conflict/ancestor + 点击跳转)延后到 P1.5/P2——安全不需要、复杂度高。

**configengine 在 P1 只读。** 所有写编辑在 P2。

## 安全检测引擎(security)

**统一抽象——`Detector` 接口:**
```go
type Detector interface {
    ID() string                  // 如 "baseline.permissions"
    Covers() []AssetType         // 扫描哪些资产
    Scan(ctx, []Asset) (*ScanResult, error)
}
```
检测器向 `Registry` 注册;`Scan` 编排器对一批资产跑所有匹配的检测器,把结果聚合成 `Finding`(`severity / asset / rule_id / evidence / remediation`)。加检测 = 实现接口 + 注册;无需改 API/UI。这是核心可扩展点,也是产品化学习的重点。

**P1 四个检测器:**

| 检测器 | 实现方式 | 数据驱动? |
|---|---|---|
| **1. 配置基线** `baseline.*` | Go 原生;读 settings.json + hooks + env | 是——规则集 YAML(危险标志黑名单、过宽权限模式、危险 env 名) |
| **2. 内容 / 提示注入** `content.injection` | Go 原生;把 MCP tool 描述 / skills / commands / agents / CLAUDE.md / scripts 当文本扫 | 是——注入模式规则集(隐藏指令、混淆:zero-width / base64 / leetspeak / HTML 注释——源自 CCO 的 9 类反混淆思路) |
| **3. 密钥扫描** `secret.*` | 子进程:gitleaks | — |
| **4. 依赖漏洞** `dep.*` | 子进程:govulncheck(Go)/ npm-audit(skill/command/plugin 里的 JS) | — |

**规则集(检测器 1、2)** 是 `internal/security/rules/*.yaml` 下的内嵌 YAML,通过 `embed` 打进二进制。每条规则带 `id / severity / description / pattern / deobfuscation[] / remediation`。规则在 **P1 不可由用户编辑**;外部化是 P4。

**子进程检测器适配(3、4):** 实现同一个 `Detector` 接口;shell out、解析扫描器 JSON 输出、归一化成 `Finding`。扫描器缺失时优雅降级——检测器把自己标记为 `unavailable` 并附原因,扫描继续。

**基线规则来源:** P1 自研规则集,从 Claude Code settings 文档 + CCO 思路提炼。Claude Code 的配置语义是 harness 专属的(`skipDangerousModePermissionPrompt`、`Bash(*)`、危险 hook 命令),通用扫描器表达不了;自研规则集本身就是基线产品化学习的一部分。

**注入检测深度(P1):** 仅文本级静态(模式匹配 + 反混淆)。不做 LLM-judge。

**扫描触发(P1):** 手动(用户点"扫描")+ 启动时自动跑一次基线快扫。文件监听触发的增量扫描延后(避免 v1 状态管理复杂度)。

## 健康分

一个聚合指标,把安全态势压成一个可比较、可解释的数字。三条硬原则:**可解释**(每个扣分点都能追溯到具体 Finding)、**单调**(修掉一个 Finding 分数只升不降)、**可还原**(给原始 Findings 能复算出同一分数)。

**模型:**
- **资产基数 N** = 在范围内的资产数。每个资产有一个**基础风险权重** `w(asset)` = 其类型权重 `w_type`(MCP server 和 Hook 最高,因为它们直接执行或注入 prompt)。
- **每个 Finding** 有按严重度(Critical / High / Medium / Low;Critical 最大)的**扣分系数** `p_sev`。
- **单资产风险** `R(asset) = Σ(该资产上 Finding 的 p_sev)`,上限封顶 `Rmax`。
- **健康分** `= 100 × (1 − Σ_assets(R(asset)·w(asset)) / (Rmax · Σ_assets w(asset)))`。无 Finding 时为 100(分子为 0),每个资产都到 `Rmax` 时为 0(分子 = 分母)。映射到 5 档:Excellent / Good / Fair / At-Risk / Critical。
- **加权扣分明细** 展示:"MCP server 'xxx' · prompt-injection High · −6 分"。用户同时看到分数、为什么、改什么。

**权重(P1):** 内置默认权重表(对资产类型 / 严重度的合理初值),规则集 YAML 里可调,但 **P1 不在 UI 暴露编辑**(延后到 P4 团队基线)。权重是显式、可审计的常量——不是魔法数字。

**学习价值:** 这套"资产 × Finding × 严重度 → 可解释聚合分"是安全产品通用的态势量化范式(对标 CVSS 的可解释性思路,更轻)——一次完整的 AI 安全指标产品化练习。

## 看板(P1 骨架)

P1 看板**只读态势视图**,不做写操作。内容(布局 mock 留到 UI 实现阶段用可视化伴侣做):

- **健康分卡:** 大数字 Score + 档位 + 本会话内与上一次扫描的 delta(P1 只在内存保留当前和上一次扫描,不持久化历史)。
- **资产盘点:** 按类型计数,Global/Project 分组——插件数、skill 数、MCP server 数、hook 数等。
- **风险摘要:** Findings 按 severity 堆叠柱状图;按资产类型热力分布;Top N 高风险资产列表。
- **检测器状态:** 4 个检测器各自:enabled / available(子进程在不在)/ 上次扫描 finding 数 / 上次耗时。
- **最近扫描:** 时间线 + 一键"重新扫描"。

**健康分趋势延后到 P1.5:** P1 不持久化扫描历史(每次扫描重算);趋势需要状态存储,延后以避免 v1 状态复杂度。看板数据全部来自内存中的当次扫描结果。

## API 与本地服务安全(P1,只读)

Gin HTTP+JSON,服务内嵌 SPA。Per-session token。本地服务能读敏感数据(`~/.claude.json` 的 MCP 凭据、settings 里 env 的 token、CLAUDE.md 机密),所以未授权访问 = 凭据/机密泄露——访问控制是必需的,不是可选的。

**Bind 策略(混合):**
- 配置文件 `~/.claude-sentinel/config.yaml` 设 `bind`(默认 `127.0.0.1`)和 `port`(默认随机高端口)。配置文件放在 `~/.claude/` **之外**,避免 sentinel 扫描自己的配置 / 递归。
- **默认 loopback → 远程访问走 SSH 端口转发**(`ssh -L <port>:127.0.0.1:<port> dev`);服务保持零网络暴露,身份认证完全复用 SSH。启动时打印对应的访问方式(和隧道命令)。
- **当 `bind` 非 loopback 时:** 启动警告 + **强制非空 IP 白名单**(`allowed_cidrs`;为空则拒绝启动,除非 `--i-know-its-risky`)+ 可选 basic auth(密码 bcrypt 哈希,绝不存明文)。
- Token 始终必需,通过 **URL fragment**(`#token=`,不是 query param——不进 server log、不进 Referer 头)传递,每个 API 请求校验。
- 严格 CORS(拒绝跨域)+ `Host` 头校验(防 DNS rebinding)。
- 除 npm/marketplace 元数据外无远程网络调用。

**P1 资源(除 POST /scan 外全只读):**

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/assets` | 资产盘点(支持 `?type=` `?scope=` 过滤) |
| GET | `/api/assets/:id` | 单资产详情(解析字段 + 原始内容预览) |
| POST | `/api/scan` | 触发扫描(可 `?detectors=baseline,secret` 选择)→ ScanResult |
| GET | `/api/scan/result` | 取最近一次内存中的扫描结果(P1 不持久化) |
| GET | `/api/findings` | Findings 列表(支持 `?severity=` `?asset=` 过滤) |
| GET | `/api/health` | 健康分 + 加权扣分明细 |
| GET | `/api/dashboard` | 聚合:资产计数 + 风险摘要 + 检测器状态(一次取全) |
| GET | `/api/detectors` | 检测器注册表 + 可用性状态 |
| GET | `/api/project` | 当前选中项目 + 可选项目列表 |
| POST | `/api/project` | 切换项目 |

**扫描并发:** `POST /api/scan` 同步返回(P1 资产规模小);context 超时控制整次扫描,避免某个子进程卡死。每个子进程检测器各自的超时归一化进同一 context。

**错误约定:** JSON `{error: {code, message, details?}}`。资产文件解析失败不致全盘失败——该资产标记 `parse_error` 作为 Finding 暴露,扫描继续。

## 前端(web/)

```
web/src/
  pages/        Dashboard / Assets / Findings / Settings
  components/   HealthScoreCard / AssetList / FindingTable / SeverityChart / DetectorStatus / ProjectSwitcher
  api/          API client(token 注入,感知 bind 的 base URL)
  store/        zustand(扫描结果缓存,资产树)
```
路由:Dashboard / Assets / Findings / Settings(只读:展示检测器、规则版本、扫描设置)。P1 无编辑页面(全只读)。

**UI 风格(刻意独立于两个参考项目):**
- shadcn/ui + Tailwind——组件 copy-paste 即归你所有、完全可主题、不自带视觉品牌。
- 风格取向:**SOC(安全运营中心)审美**——深色优先、高信息密度、风险用语义色编码(Critical=红 / High=橙 / Medium=琥珀 / Low=蓝绿)、等宽字体显示配置/规则文本、卡片化态势。
- 刻意不同于 CCO 的轻盈 web 风 和 Studio 的表单式设置风。

**布局 mock 延后:** 本规格只定*风格取向*(SOC 深色高密度 + 语义色);具体线框图/mock 在 UI 实现阶段用可视化伴侣产出,由你拍板。这样规格聚焦、不被未定的视觉细节拖住。

## 错误处理

- 现有文件里 JSON 损坏:展示文件并定位解析错误;提供 raw 预览;绝不穿过解析失败写入(P1 只读,所以仅展示)。
- 子进程调用失败:原样展示 stdout/stderr 和确切命令;标记检测器 unavailable。
- 扫描器二进制缺失:检测器报 `unavailable` + 原因;整体扫描成功但覆盖减少。
- Bind 策略违规(非 loopback + 白名单空):拒绝启动并给清晰提示 + `--i-know-its-risky` 覆盖提示。

## 测试

- **configengine:** 用临时目录里的 fixture home 做单测——测试重心(发现、解析、scope 标注、重复检测、损坏文件处理)。
- **security:** 每个检测器用 fixture 资产做单测(基线规则匹配、注入反混淆、分数单调性/可还原性);子进程检测器用 fixture 扫描器输出测(并有一条二进制缺失时跳过的集成路径)。
- **api:** 在引擎之上用 fixture 做集成测试(只读端点、扫描触发、错误约定)。
- **前端:** 少量 Playwright e2e 流——加载看板 → 触发扫描 → 看到 findings + 分数。

## 分发

- 单二进制;`sentinel` 无需安装即可运行。前端产物内嵌(用户机器无构建步骤)。
- 外部扫描器(gitleaks/govulncheck/npm-audit)是可选运行时依赖——启动时探测;缺失则优雅降级。

## 后续(明确不在 P1)

- P2 外科手术式配置编辑 + 备份/迁移。
- P3 内嵌 agent + 会话历史。
- P4 动态检测(实时 MCP、hash 基线、rug-pull 监控)+ 团队基线规则外部化 + 扫描历史持久化 / 健康分趋势 + 多 harness。
