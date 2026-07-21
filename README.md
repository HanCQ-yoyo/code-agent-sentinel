# Code Agent Sentinel

> A local single-binary security console for Claude Code configuration assets — discover, scan, edit, and monitor the `~/.claude/` surface with an explainable health score.

English | [中文](README.zh-CN.md)

## Features

- **Asset discovery & parsing**: reads `~/.claude/` and project `.claude/` — settings, permissions, hooks, MCP servers, skills, commands, agents, plugins, CLAUDE.md/memory, keybindings, scripts.
- **Security detection**: unified rule engine (63 built-in rules) + prompt-injection scanning with deobfuscation + secret scanning (gitleaks) + dependency audit (govulncheck / npm-audit). Missing scanner binaries degrade gracefully.
- **Suppressions & baseline**: silence known findings via `suppressions.yaml`; snapshot accepted fingerprints in `baseline.json` (create / prune from CLI or API).
- **Health score**: `Score = 100 × (1 − Σ(R(asset)·w(asset)) / (Rmax · Σ w(asset)))`, Rmax=10, 0–100, 5-tier — explainable, monotone, restorable.
- **Config editing**: atomic writes with automatic backup + migration (`internal/editor`); configengine stays read-only.
- **Scheduled scanning**: in-process scheduler (`scan_interval` / `scan_enabled`) keeps history fresh; `sentinel scan` does one-shot discover→scan→write-history without the server.
- **Custom `.claude` directory**: `claude_dir` + `discovery.disabled_asset_types` let you point at an alternate config root and skip asset types you don't care about.
- **Bilingual UI**: in-app `zh` / `en` switch (react-i18next) with `language` config default; backend strings remain Chinese.
- **Finding-location highlighting**: rule findings carry `Location{Line,StartCol,EndCol}` surfaced in the Monaco viewer.
- **Project pinning**: pin frequently-used projects to the top of the Assets page with a color tag (`pinned_projects`).
- **Dashboard**: health-score card, risk summary, detector status, asset inventory, history trends.

## Install

Prebuilt binaries are released as single archives (frontend embedded). For a local build:

```bash
git clone <repo> && cd code-agent-sentinel
make build          # builds web (npm run build) + Go binary -> bin/sentinel
```

Requires Go 1.25 and Node.js (for `make web`). The resulting `bin/sentinel` is fully self-contained.

## Quick Start

```bash
./bin/sentinel                  # 127.0.0.1 + random port, auto-opens browser
# Token is printed to stdout and passed via URL fragment (#token=...).

# One-shot scan without starting the server:
sentinel scan

# Remote dev box (service stays loopback-only; tunnel the port):
ssh -L <port>:127.0.0.1:<port> <devhost>
```

## Configuration

`~/.claude-sentinel/config.yaml` — deliberately outside `~/.claude/` to avoid self-scan. Empty fields fall back to defaults via `Resolve*` methods.

| Field | Type | Description |
| --- | --- | --- |
| `bind` | string | Bind address. Default `127.0.0.1`; non-loopback requires `allowed_cidrs` (or `--i-know-its-risky`). |
| `port` | int | `0` = random ephemeral port. |
| `allowed_cidrs` | []string | IP allowlist; mandatory for non-loopback binds. |
| `basic_auth` | object | `user` + bcrypt `password_hash`. Token auth remains primary. |
| `home_dir` | string | Overrides `$HOME` for discovery (debug). |
| `claude_dir` | string | Absolute path to `.claude` root; empty = `<home>/.claude`. |
| `discovery.disabled_asset_types` | []string | Skip asset types (e.g. `mcp`, `scripts`) during discovery. |
| `scan_interval` | duration string | e.g. `30m`, `1h`; empty/invalid = off. |
| `scan_enabled` | bool | Master switch for the in-process scheduler. |
| `language` | string | `zh` / `en`; empty = browser-detect then fall back to `zh`. |
| `pinned_projects` | list | `{path, color}` entries pinned on the Assets page. |
| `dir_tags` | map | Per-path label overrides. |
| `favorites` | []string | Pinned asset IDs (persisted server-side). |
| `backup_dir` | string | Backup root; empty = `~/.claude-sentinel/backups`. |
| `max_backups` | int | `0` = default 20. |
| `sentinel_rules_dir` | string | Global custom rules dir; empty = `~/.claude-sentinel/rules`. |
| `suppress_path` | string | Suppressions file; empty = `~/.claude-sentinel/suppressions.yaml`. |
| `baseline_path` | string | Baseline file; empty = `~/.claude-sentinel/baseline.json`. |
| `suppression_discount` | float | Residual risk factor for suppressed findings; `0`/negative = 0.3. |
| `detectors` | object | Per-detector `enabled` toggles + binary paths (rules / secret / dep). |

Example:

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

## CLI

All subcommands accept `--config` to override the config path. `--home` overrides `$HOME` for debug/test.

| Command | Purpose |
| --- | --- |
| `sentinel` | Start the local SOC dashboard server (default). Flags: `--config`, `--bind`, `--port`, `--no-browser`, `--i-know-its-risky`, `--home`, `--token`, `--claude-dir`. |
| `sentinel scan` | One-shot scan (discover → scan → write history), no server. `--detectors=rules,secret` restricts which detectors run. |
| `sentinel uninstall` | Delete `~/.claude-sentinel/` (history, backups, baseline, suppressions, rules). Does **not** touch `~/.claude` or the binary. `--yes` skips confirmation; `--keep-config` retains `config.yaml`. |
| `sentinel baseline` | `--create` merges current findings into `baseline.json`; `--prune` removes non-reproducing fingerprints. |
| `sentinel rules` | `list` prints id/severity/source/valid; `validate [file]` checks rule files (no arg = builtin + global). |

## Security Model

- **Loopback by default**: `bind` defaults to `127.0.0.1`. Non-loopback binds require a non-empty `allowed_cidrs`, otherwise startup is refused (override with `--i-know-its-risky`).
- **Token via URL fragment**: the random token is delivered through `#token=` — it never reaches server logs or `Referer` headers — and is verified on every API request.
- **Host header + strict CORS**: guards against DNS rebinding.
- **No auto-open on non-loopback**: opening a browser would leak the token through `xdg-open` argv on multi-user hosts.
- **Graceful degradation**: missing `gitleaks` / `govulncheck` / `npm` binaries mark the detector `unavailable` with a reason; the overall scan continues.
- **Scoped uninstall**: `sentinel uninstall` only deletes `~/.claude-sentinel/`; your Claude Code config and the binary are untouched.

## Development

```bash
make build          # web + Go binary -> bin/sentinel
make test           # go test ./...
make run            # build + run
make web            # build frontend only (vite build -> internal/api/web_dist)
make web-install    # cd web && npm install
make clean          # remove bin/, web/dist, web_dist
make release        # cross-platform archives for linux/darwin/windows
make build-cross GOOS=darwin GOARCH=arm64     # single platform
make build-cross-fast GOOS=linux GOARCH=arm64 # skip frontend rebuild
```

Frontend e2e: `cd web && npm run test:e2e` (Playwright).

Tech stack: **Go 1.25** (Gin + cobra + embed) + **React 18 / Vite / TypeScript / antd v5 / zustand / monaco-editor / react-i18next**. Single binary distribution — the React build is embedded into the Go binary.
