# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

`code-agent-sentinel`(二进制名 `code-agent-sentinel`)是一个本地单二进制工具:把 Claude Code 的配置资产(settings、hooks、MCP servers、skills、commands、agents、plugins、CLAUDE.md/memory、scripts)当作**安全管控面**,发现并解析它们,跑静态安全检测(基线 / 提示注入 / 密钥 / 依赖),算出一个可解释的健康分,呈现只读 SOC 风格看板。P1 阶段**全只读**。

## 常用命令

```bash
make build        # go build -o bin/code-agent-sentinel ./cmd/sentinel
make test         # go test ./...
make run          # build 后运行 ./bin/code-agent-sentinel
make clean        # 删除 bin/ web/dist
make web-install  # cd web && npm install(前端尚未建立)
make web          # cd web && npm run build(前端尚未建立)

# 跑单个测试(本项目核心开发动作,见下「开发工作流」)
go test ./internal/configengine/ -run TestHashAndMTime -v
go test ./internal/security/ -run TestHealth -v
```

Go 工具链实际为 1.25(`go.mod` 声明 `go 1.22`)。目前尚无外部依赖(`go.sum` 不存在);`gopkg.in/yaml.v3` 在 Task 10 引入,Gin/cobra/gjson/sjson 在后续任务引入——新增依赖后务必 `go mod tidy`。

## 架构

单个 Go 二进制,分层,依赖**只向下不向上**:

```
cmd/sentinel          ← cobra CLI:启动 / 端口 / token / 开浏览器
internal/api          ← Gin HTTP+JSON,内嵌 SPA,token + Host 校验 + bind 策略
internal/security     ← Detector 接口 + Registry,4 个检测器,Scan 编排器,健康分
internal/configengine ← 纯逻辑:发现 + 解析 + 资产模型 + 只读查询(无副作用,fixture 可测)
internal/config       ← 配置加载与默认值
internal/web          ← embed.FS,持有构建好的前端产物
web/                  ← React/Vite/TS 源码(单独构建,产物经 embed 打进二进制)
```

依赖方向:`configengine ← security ← api`。`configengine` 刻意保持纯净、可复用——P2 的写编辑、P4 的动态检测都建在它之上。

**可扩展核心:`Detector` 接口**(`internal/security`)。检测器向 `Registry` 注册,`Scan` 编排器对一批资产跑所有匹配 `Covers()` 的检测器,结果聚合成 `Finding`。加检测 = 实现接口 + 注册,无需改 API/UI。P1 四个检测器:基线(Go 原生 + 内嵌 YAML 规则)、提示注入(Go 原生 + 反混淆)、密钥(子进程 gitleaks)、依赖(子进程 govulncheck / npm-audit)。

资产模型在 `internal/configengine/types.go`:`Asset` 带 `id/type/scope/global·project·managed/source_path/name/fields/content/mtime/hash/parse_error`。

## 关键约束与约定

- **configengine 必须无副作用**:所有路径注入(`Engine` 接受 `home` 参数),**绝不**直接读真实 `~/.claude`;用临时目录 fixture 测试(`fixtures_test.go` 的 `newFixture(t)`)。
- **健康分公式**:`Score = 100 × (1 − Σ(R(asset)·w(asset)) / (Rmax · Σ w(asset)))`,其中 `R(asset) = min(Σ p_sev, Rmax)`。无 Finding = 100,全满 = 0。三条硬原则:可解释、单调(修掉 Finding 分数只升不降)、可还原。权重表是 `internal/security/health.go` 里的显式常量,不是魔法数字。
- **本地服务安全**:`configengine` 能读到敏感凭据,所以访问控制是必需的。默认 bind `127.0.0.1`;非 loopback 必须有非空 `allowed_cidrs` 否则**拒绝启动**(除非 `--i-know-its-risky`)。token 经 URL fragment `#token=` 传递(不进 server log / Referer),每个 API 请求校验;严格 CORS + Host 头校验防 DNS rebinding。
- **错误约定**:API 返回 `{error: {code, message, details?}}`;资产文件解析失败不致全盘失败——该资产标记 `parse_error` 作为 Finding 暴露,扫描继续。
- **扫描器缺失优雅降级**:子进程检测器(gitleaks/govulncheck/npm-audit)缺失时标记 `unavailable` 并附原因,整体扫描继续。
- **sentinel 自己的配置**放 `~/.code-agent-sentinel/config.yaml`——在 `~/.claude/` **之外**,避免 sentinel 扫描自己的配置 / 递归。
- **所有 markdown 文件用中文**(本项目约定)。
- **不要解析图片/视频等多模态内容**:当前模型不支持多模态,遇到图片(`.png`/`.jpg`/`.gif` 等)、视频、二进制等无法以文本解读的资源时,**不要尝试解析其内容**;如确需了解,请向用户询问其内容或要求以文字描述提供,而非猜测图片/视频里的信息。使用 Playwright 驱动浏览器时同理——**不要调用截图(`browser_take_screenshot`)后再尝试解析图片**,截图会因多模态不支持而报错;需要观察页面状态时,改用文本快照(`browser_snapshot`)、DOM/`browser_evaluate` 取文本或控制台日志(`browser_console_messages`)等纯文本手段。
- **提交信息**用 conventional commits + 中文描述,如 `feat(configengine): 资产类型与 hash 助手`、`chore: 项目脚手架`、`docs: ...`。

## 开发工作流

本项目用 **superpowers 的 subagent-driven-development(SDD)+ TDD** 推进,按任务逐个实现:

- **设计规格**:`docs/superpowers/specs/`(当前 `2026-07-02-code-agent-sentinel-p1-design.md`)。
- **实现计划**:`docs/superpowers/plans/`(当前 `2026-07-02-code-agent-sentinel-p1.md`,28 个任务,每个任务含 Files / Interfaces / 逐步骤复选框)。
- **任务执行状态**:`.superpowers/sdd/` 下有 `progress.md`、`task-N-brief.md`、`task-N-report.md`、`review-<range>.diff`。**注意:`.superpowers/sdd/.gitignore` 是 `*`,即该目录是本地工作状态,不进版本库**——查进度看这里,但别指望从 git 历史里找到它。

每个任务的标准 TDD 循环:写失败测试 → `go test` 确认失败 → 实现 → `go test` 确认通过 → 单独提交。计划里的代码片段是**起点不是圣旨**——已落地的 Task 3 就因 Linux 上目录无法 `io.Copy` 而 Adjusted 了 `placeholder`(见 [discover_global.go](internal/configengine/discover_global.go) 注释);遇到类似落地偏差时,在代码里写注释说明原因,而不是默默偏离计划。

## 其他

- **`references/` 目录不是本项目代码**:它装的是两个参考项目(cross-code-organizer、claude-code-studio)的源码,仅供设计参考(对比差异、借鉴反混淆思路)。不要把它们纳入构建,也不要改它们。
- **当前进度**:P1 进行中,Task 1-2 已完成并通过 review,Task 3 已提交待 review。后续任务(security / api / config / web)尚未开始——动手前先读计划文件对应任务的完整步骤。
