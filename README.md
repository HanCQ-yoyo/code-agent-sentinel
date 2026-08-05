# Code Agent Sentinel

> A local single-binary security console for AI coding agents —
> discover, detect, intercept, and monitor your Claude Code configuration surface.

English | [中文](README.zh-CN.md)

## Why Sentinel?

AI coding agents like Claude Code and Codex CLI accumulate a significant
configuration attack surface: MCP servers, hooks, skills, credentials, custom
commands — spread across 12+ asset types. Manual review doesn't scale.

Sentinel gives you a SOC-style dashboard from one binary:
**asset discovery → static detection → runtime interception → explainable health score.**

## Core Capabilities

### Multi-Agent Asset Discovery

Discover and parse assets from Claude Code (`~/.claude/`) and OpenAI Codex CLI
(`~/.codex/`). Covers settings, permissions, hooks, MCP servers, skills,
commands, agents, plugins, CLAUDE.md/memory, keybindings, scripts, and
credential files — 12 asset types across both agents.

`code-agent-sentinel setup` auto-detects installed agents. The dashboard
aggregates multi-agent views with independent per-agent scans.

### Security Detection Engine

- **Unified rule engine**: 256 built-in rules + 6 cross-asset combo rules
- **Prompt injection scanning** with deobfuscation
- **Secret scanning** (gitleaks) + **dependency audit** (govulncheck / npm-audit)
- Rules stored in sqlite, hot-reloaded — create, edit, fork, enable, or disable
  rules via the Settings UI with no restart needed. Missing scanner binaries
  degrade gracefully.

### Runtime Command Interception

A `PreToolUse` Bash hook for both Claude Code and Codex CLI evaluates every
shell command through a span-aware pipeline:

parse → short-circuit → deobfuscate → chain-split (`&&`/`;`/`||`/`|`) →
span classifier (quote/comment/command-substitution) → rule engine → decision

- Blocks destructive commands (`rm -rf /`, `git reset --hard`, chained
  bypasses, ANSI-C obfuscation)
- Avoids false positives on data literals (`echo "rm -rf /"` is safely allowed)
- Confidence scoring (high/low/unknown) + strict/lenient modes + allowlist
- **fail-open**: the hook always exits 0; denial is expressed via stdout JSON
- Full audit log with confidence + matched-span metadata on the `/intercept` page

### Risk Dashboard & Lifecycle

- **Health score**: 0–100, 5-tier, based on an explainable formula — monotone
  and restorable. Fix a finding, and the score only goes up.
- **Unified disposition lifecycle**: open → in_progress → resolved /
  false_positive / accepted, with bulk-accept and prune
- **Capability panel**: structured view of allowed-tools, hook events, MCP
  commands, and memory outline per asset
- **False-positive reduction**: negation-context suppression, per-asset dedup,
  dual-view FindingTable (by finding / by asset)
- **Scan-task view**: per-agent history with scope and target columns

### Config Management

All runtime settings are managed through the Web UI and persisted to sqlite —
no manual YAML editing needed:

- Per-detector enable/disable + binary paths
- Per-agent scan schedules with an in-process scheduler
- Guard config (policy, deadline, mode) — hot-reloaded
- Project pinning with color tags

## Quick Start

```bash
./bin/code-agent-sentinel                  # 127.0.0.1 + random port, auto-opens browser
# Token is printed to stdout and passed via URL fragment (#token=...).

# One-shot scan without starting the server:
code-agent-sentinel scan

# Remote dev box (service stays loopback-only; tunnel the port):
ssh -L <port>:127.0.0.1:<port> <devhost>
```

## Configuration

`~/.code-agent-sentinel/config.yaml` — deliberately outside `~/.claude/` to
avoid self-scan.

> Language, favorites, pinned projects, scan schedules, detector config, guard
> settings, and other runtime options are managed in the Web UI and persisted
> to SQLite. No manual YAML editing needed.

Only bootstrap fields belong in config.yaml:

| Field | Type | Description |
|---|---|---|
| `bind` | string | Bind address. Default `127.0.0.1`; non-loopback requires `allowed_cidrs` (or `--i-know-its-risky`). |
| `port` | int | `0` = random ephemeral port. |
| `allowed_cidrs` | []string | IP allowlist; mandatory for non-loopback binds. |
| `basic_auth` | object | `user` + bcrypt `password_hash`. |
| `home_dir` | string | Override `$HOME` for discovery (debug). |
| `claude_dir` | string | Absolute path to `.claude` root; empty = `<home>/.claude`. |
| `discovery.disabled_asset_types` | []string | Skip asset types during discovery. |
| `backup_dir` | string | Backup root; empty = `~/.code-agent-sentinel/backups`. |
| `max_backups` | int | `0` = default 20. |
| `log_path` | string | Log file path; empty = stderr. |
| `token` | string | Pre-set access token; empty = random at startup. |
| `known_projects` | list | `{path, name}` entries; `setup` auto-imports from `~/.claude.json`. |
| `agents` | list | Agent definitions (`id`/`enabled`/`root_dir`/`claude_json`); `setup` populates. |

Example:

```yaml
bind: 127.0.0.1
port: 0
language: en
claude_dir: /home/me/.claude
discovery:
  disabled_asset_types: [scripts]
```

## CLI Reference

All subcommands accept `--config` to override the config path.

| Command | Purpose |
|---|---|
| `code-agent-sentinel` | Start the local SOC dashboard server (default). |
| `code-agent-sentinel scan` | One-shot scan (discover → scan → write history), no server. |
| `code-agent-sentinel guard` | Runtime interception hook (invoked by Claude Code `PreToolUse`). |
| `code-agent-sentinel setup` | Interactive agent configuration. |
| `code-agent-sentinel uninstall` | Clean up `~/.code-agent-sentinel/`. Does not touch `~/.claude`. |
| `code-agent-sentinel baseline` | `--create` bulk-accepts findings; `--prune` removes stale ones. |
| `code-agent-sentinel rules` | `list` prints rules; `validate` checks rule files. |
| `code-agent-sentinel service` | `install`/`uninstall`/`status` systemd service. |

## Security Model

- **Loopback by default**: `bind` defaults to `127.0.0.1`. Non-loopback binds
  require a non-empty `allowed_cidrs` (or `--i-know-its-risky`).
- **Token via URL fragment**: the random token is delivered through `#token=` —
  never reaches server logs or `Referer` headers — and is verified on every
  API request.
- **Host header + strict CORS**: guards against DNS rebinding.
- **No auto-open on non-loopback**: avoids leaking the token through
  `xdg-open` argv on multi-user hosts.
- **Graceful degradation**: missing `gitleaks`/`govulncheck`/`npm` binaries
  mark the detector `unavailable`; the overall scan continues.
- **Scoped uninstall**: `code-agent-sentinel uninstall` only deletes
  `~/.code-agent-sentinel/`; your Claude Code config and the binary are
  untouched.

## Development

```bash
make build          # web + Go binary -> bin/code-agent-sentinel
make test           # go test ./...
make run            # build + run
make web            # build frontend only
make web-install    # cd web && npm install
make clean          # remove bin/, web/dist
make release        # cross-platform archives
```

Frontend e2e: `cd web && npm run test:e2e` (Playwright).

Tech stack: **Go 1.25** (Gin + cobra + embed) + **React 18 / Vite / TypeScript /
antd v5 / zustand / monaco-editor / react-i18next**. Single binary distribution
— the React build is embedded into the Go binary.
