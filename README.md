# Code Agent Sentinel

> A local single-binary security console for Claude Code configuration assets — discover, scan, edit, and monitor the `~/.claude/` surface with an explainable health score.

English | [中文](README.zh-CN.md)

## Features

- **Asset discovery & parsing**: reads `~/.claude/` and project `.claude/` — settings, permissions, hooks, MCP servers, skills, commands, agents, plugins, CLAUDE.md/memory, keybindings, scripts, **credential** (`auth.json` / `.env` / `*.pem` etc., content not exposed) — 12 asset types. Supports multiple code agents: **Claude Code** (`~/.claude/`) and **OpenAI Codex CLI** (`~/.codex/config.toml`, `AGENTS.md`, `prompts/`, `hooks.json`). `sentinel setup` auto-detects installed agents; the dashboard aggregates multiple agents with independent scans.
- **Discovery & cross-asset rules**: Claude L1-L5 — `managed-mcp.json` (enterprise mode hint), global `.mcp.json`, project `hooks/` directory, project `keybindings.json`, `skip_dangerous_mode_permission_prompt` field; Codex C2/C3 — project-level `.codex/config.toml` + `[hooks.state]` modeling; Codex C4/L6 — `auth.json` credential + project-root sensitive files. **6 cross-asset combo rules** (skip-perm+Bash(*) / Codex danger+never / credential exfil etc.) via the unified rule engine's new `ComboRule` second pass. Codex project-level discovery reads the sentinel `known_projects` list (independent of `~/.claude.json`).
- **Security detection**: unified rule engine (256 built-in rules + 6 cross-asset combo rules) + prompt-injection scanning with deobfuscation + secret scanning (gitleaks) + dependency audit (govulncheck / npm-audit). Missing scanner binaries degrade gracefully.
- **Configurable rules (sqlite-backed)**: rules for both the detection and interception domains are stored in a single sqlite db (`~/.claude-sentinel/sentinel.db`, WAL mode, `0o600`) — three tables: `rules` / `overrides` (enable-disable overlay, JOIN-derives `enabled`) / `combos`. Built-in rules are synced from embed on startup (legacy `~/.claude-sentinel/rules/*.yaml` auto-migrate in). Custom rules can be created / edited / forked-from-builtin / enabled-disabled / deleted via the Settings page (`RuleDrawer`) and the symmetric `/api/{detect|intercept}-rules` CRUD + `/validate` endpoints; built-in rules are read-only (POST over a builtin id → 409). `RulesDetector` reads the db live at scan time (hot reload, no restart); `sentinel guard` reads interception rules from the db and fail-opens to built-ins on db failure. File-path and db-path rule evaluation are verified equivalent (per asset_type).
- **Unified disposition lifecycle**: collapses the former `baseline.json` + `suppressions.yaml` into a single `finding_states.yaml` overlay (`findingstate` package: Status / Priority / Note / Category / ContributingRuleIDs). `sentinel baseline --create` bulk-accepts all currently-undisposed findings; `--prune` prints a prune report. Old files auto-migrate (renamed `.legacy`, not deleted). Accepted findings no longer drag down the health score.
- **Health score**: `Score = 100 × (1 − Σ(R(asset)·w(asset)) / (Rmax · Σ w(asset)))`, Rmax=10, 0–100, 5-tier — explainable, monotone, restorable.
- **Config editing**: atomic writes with automatic backup + migration (`internal/editor`); configengine stays read-only.
- **Scheduled scanning**: in-process scheduler (`scan_interval` / `scan_enabled`) keeps history fresh; `sentinel scan` does one-shot discover→scan→write-history without the server.
- **Custom `.claude` directory**: `claude_dir` + `discovery.disabled_asset_types` let you point at an alternate config root and skip asset types you don't care about.
- **Bilingual UI**: in-app `zh` / `en` switch (react-i18next) with `language` config default; backend strings remain Chinese.
- **Finding-location highlighting**: rule findings carry `Location{Line,StartCol,EndCol}` surfaced in the Monaco viewer.
- **Capability panel**: structured view of allowed-tools / hook events / mcp commands / memory outline per asset, replacing the old one-line description.
- **FP reduction**: negation-context suppression (findings prefixed with "forbidden/not allowed" no longer fire); per-asset dedup (same-location multi-rule hits collapse into one finding with `ContributingRuleIDs`); dual-view FindingTable (by finding / by asset).
- **Scan-task view**: per-agent history page with detection scope/target columns (`ScanSummary.ScopePath`: `global` / `project:<path>` / `asset:<id>`).
- **Runtime command interception (Claude + Codex)**: `sentinel guard` runs as a `PreToolUse` Bash hook for both Claude Code and OpenAI Codex CLI, evaluating each shell command through a span-aware pipeline (parse → recursive short-circuit → quick-reject → deobfuscating normalize → heredoc extraction → **chain splitting + span classification** → rule-engine eval → decision+record) and denying destructive ones (`rm -rf /`, `git reset --hard`, `git commit -m "x" && rm -rf /` chained, ANSI-C obfuscation, `bash -c "..."` inline scripts). **Span classifier** (quote/comment/command-substitution state machine) restricts destructive-regex matching to executed regions, suppressing data-area literal false positives (`echo "rm -rf /"` no longer denied). **Chain splitter** splits on `&&`/`;`/`||`/`|` and evaluates each segment independently (closes the Stage R2 chained-bypass gap). **Confidence scoring** (high/low/unknown based on where the hit lands in a span) + **Mode** (`strict` default = deny on uncertain / `lenient` = ask on uncertain; high-confidence hits deny in both). **Allowlist** (independent `allowlist.yaml`, exact double-match normalize before+after, no wildcards) lets approved commands through even when rules match. fail-open iron law: the hook always exits 0; deny is expressed only via stdout JSON. **Codex protocol adaptation**: `turn_id` field auto-disambiguates Claude/Codex; Codex gets a minimal 3-field deny payload (strict-parser-safe); low-confidence `ask` degrades to `deny` under Codex. `sentinel setup` auto-registers the hook in `~/.claude/settings.json` and `~/.codex/hooks.json`; decisions log to `~/.claude-sentinel/intercept/` (with `confidence` + `matched_span`) and render read-only on the `/intercept` page. Configurable via the `guard` config section (`enabled` / `policy` / `deadline_ms` / `max_command_bytes` / `mode` / `allowlist_enabled`, hot-reloaded via `PUT /api/guard/config`); the Settings page exposes a GuardConfig editor + Allowlist editor.
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
| `known_projects` | list | `{path, name}` entries — sentinel's independent known-projects list; `setup` auto-imports from `~/.claude.json` projects as initial value. Used for Codex project-level discovery (and Claude). Empty → Claude falls back to `~/.claude.json` projects. |
| `dir_tags` | map | Per-path label overrides. |
| `favorites` | []string | Pinned asset IDs (persisted server-side). |
| `backup_dir` | string | Backup root; empty = `~/.claude-sentinel/backups`. |
| `max_backups` | int | `0` = default 20. |
| `sentinel_rules_dir` | string | Global custom rules dir; empty = `~/.claude-sentinel/rules`. |
| `finding_states_path` | string | Disposition overlay file; empty = `~/.claude-sentinel/finding_states.yaml`. Legacy `baseline.json` / `suppressions.yaml` auto-migrate on first start (renamed `.legacy`). |
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
| `sentinel guard` | Runtime interception hook (invoked by Claude Code `PreToolUse`). Reads stdin JSON, evaluates the Bash command, writes a deny/allow decision to stdout. Always exits 0 (fail-open). Flags: `--config`, `--deadline`, `--debug`. Normally auto-registered by `sentinel setup`. |
| `sentinel uninstall` | Delete `~/.claude-sentinel/` (history, backups, finding_states, rules db). Does **not** touch `~/.claude` or the binary. `--yes` skips confirmation; `--keep-config` retains `config.yaml`. |
| `sentinel baseline` | `--create` bulk-accepts all currently-undisposed findings into `finding_states.yaml`; `--prune` prints a prune report for non-reproducing fingerprints. |
| `sentinel rules` | `list` prints id/severity/source/valid (reads from the sqlite db); `validate [file]` checks rule files (no arg = builtin + global). Rule create/edit/enable/fork/delete is via the Settings page + `/api/{detect|intercept}-rules`. |

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
