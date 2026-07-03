# code-agent-sentinel

针对 Coding Agent CLI(以 Claude Code 为先)用户级配置、能力扩展、行为规则与状态文件的**安全管理平台**。把 Claude Code 的配置 / 插件 / 数据当作安全管控资产,做静态安全检测、可解释健康分与只读态势看板。

> P1 阶段:只读。配置编辑 / 内嵌 agent / 会话历史 / 动态检测在后续阶段。

## 功能

- **配置与能力管理**:发现并解析 `~/.claude/` 与项目 `.claude/` 下的 settings、permissions、hooks、MCP server、skills、commands、agents、plugins、CLAUDE.md/memory、keybindings、scripts。
- **安全检测**:配置基线检查 + 提示注入扫描(静态)+ 密钥扫描(gitleaks)+ 依赖漏洞(npm audit / govulncheck)。
- **健康分**:资产×Finding×严重度的可解释聚合分(0–100,5 档)。
- **Dashboard**:健康分卡、风险摘要、检测器状态、资产盘点。

## 构建

```bash
make build          # 构建前端 + Go 二进制 bin/sentinel
make test           # Go 测试
cd web && npm run test:e2e   # 前端 e2e
```

## 运行

```bash
./bin/sentinel              # 默认 127.0.0.1 + 随机端口,自动开浏览器
```

远程开发机访问(推荐 SSH 隧道,服务零暴露):

```bash
# 在本地 PC:
ssh -L <端口>:127.0.0.1:<端口> <开发机>
```

## 配置

`~/.claude-sentinel/config.yaml`(在 `~/.claude/` 之外,避免自扫):

```yaml
bind: 127.0.0.1          # 默认 loopback;非 loopback 须配 allowed_cidrs
port: 0                  # 0=随机
allowed_cidrs: []        # 非 loopback 时必填
# basic_auth:
#   user: admin
#   password_hash: "$2a$..."   # bcrypt
```

## 安全

- 默认仅绑 127.0.0.1,token 经 URL fragment 传递,严格 CORS + Host 校验。
- 非 loopback 启动强制非空 IP 白名单(否则拒绝);basic_auth 字段已解析但 P1 尚未强制(认证以 token 为准)。
- 扫描器(gitleaks/govulncheck/npm)缺失时优雅降级。

## 技术栈

Go(Gin + cobra + embed)+ React/Vite/TypeScript/Tailwind。单二进制分发。

## 后续阶段

P2 配置编辑+备份迁移 / P3 内嵌 agent+会话历史 / P4 动态检测+团队基线+趋势。
