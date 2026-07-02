# Code Agent Sentinel — P1 Design Spec

**Date:** 2026-07-02
**Status:** Approved direction; pending implementation plan
**Scope:** Phase 1 (P1) only — see Phasing below for what is deferred

## Problem & Positioning

Vibe coding harnesses (Claude Code first) are configured through scattered files and directories under `~/.claude/` (and per-project `.claude/`). These assets — `settings.json` permissions, hooks, MCP server tool descriptions, skills, commands, agents, plugins, CLAUDE.md, scripts — directly steer what the agent can do and what enters its prompt. A single risky permission, a poisoned MCP tool description, or a hook that exfiltrates data can compromise the developer's machine and codebase. There is no local, single-pane view that treats these assets as **security-governed surface** and detects risk across them.

**Positioning:** Claude Code CLI's configuration / plugins / data are the **security assets to govern**. The core capability is **security detection**; configuration browsing is both the entry point and a productivity aid. The project is also an explicit learning vehicle for Claude Code's design philosophy and AI-security productization. Claude Code is chosen because it is the harness the author uses most; AI security is the long-term direction; lightweight R&D-productivity features accompany the security core.

**Differentiation from references:**
- *Cross-Code Organizer (CCO)* and *Claude Code Studio* both already do "config management + security scan" well. Sentinel does **not** try to match their full breadth. It leads with **security posture as a first-class, quantified, explainable concept** (health score + weighted deductions + baseline governance), keeps config management to the minimum needed for security visibility plus a few productivity aids, and is built on a Go single-binary stack rather than Node.
- UI is **deliberately independent** of both references (SOC-style, high-density, semantic-color — see Frontend).

## Goal (P1)

A local, single-binary tool `sentinel` that discovers and parses Claude Code's config assets (global + one selected project), runs static security detection across them (baseline + prompt-injection + secret/dependency), computes an explainable health score, and presents a read-only security-posture dashboard. P1 is **read-only** — no config editing.

## Non-Goals (P1)

- Config editing / writes (P2)
- Backup and migration (P2)
- Embedded agent for CLAUDE.md memory updates / Q&A (P3)
- Session history management (P3)
- Dynamic detection: live MCP connection, hash baselining, rug-pull / change monitoring (P4)
- Team / baseline rule externalization (P4)
- Multi-harness support (Codex, Cursor, …) — Claude Code only for now
- LLM-judge detection (calling an LLM to judge maliciousness) — static only

## Phasing

| Phase | Scope | Status |
|---|---|---|
| **P1 (this spec)** | Foundation (config discovery + asset model) + security detection v1 (baseline + static injection + secret/dependency) + read-only dashboard skeleton + read-only config browsing | ← now |
| P2 | Surgical config editing (diff preview + auto-backup) + backup/migration | future |
| P3 | Embedded agent (CLAUDE.md memory updates + Q&A) + session history | future |
| P4 | Dynamic security: live MCP tool-definition fetch + hash baseline + rug-pull/change monitoring + team baseline rule externalization | future |

Each phase gets its own spec → plan → implementation cycle.

## Tech Stack

- **Backend:** Go single binary. Gin (HTTP API) + cobra (CLI) + `tidwall/sjson`/`gjson` (surgical JSON access) + `embed` (frontend assets). (fsnotify is a P4 dependency for change monitoring, not used in P1.)
- **Frontend:** React + Vite + TypeScript + Tailwind CSS + shadcn/ui + zustand. Built separately, bundled into the binary via `embed`.
- **External scanners (subprocess):** gitleaks (secrets), govulncheck (Go deps), npm-audit (JS deps in skills/commands/plugins). semgrep optional for multi-language SAST on scripts.
- **Distribution:** single binary; `sentinel` launches a local server and opens the browser. (vs. the references' `npx` model — single-binary is an asset for a security tool.)

Python is **not** required: the detection here is pattern/rule/baseline/static-analysis driven, not ML inference. OSS scanners run as subprocesses regardless of their own language.

## Architecture

Single Go binary `sentinel`. Startup: pick bind address + port per config, generate a per-session token, start Gin API, open browser to `http://<bind>:<port>/#token=<token>`.

Layered, dependencies point downward only (mirrors the Studio core/server/web split, with a security layer added):

```
cmd/sentinel          ← cobra: launch / port / token / open browser (later: `scan --json` headless)
internal/api          ← Gin HTTP+JSON, serves embedded SPA, token + Host validation + bind policy
internal/security     ← Detector interface + registry, 4 detectors, Scan orchestrator, health score
internal/configengine ← pure logic: discovery + parse + asset model + read-only queries (no side effects, fixture-testable)
internal/web          ← embed.FS holding the built frontend
web/                  ← React/Vite/TS source (built separately, output embedded)
```

`configengine ← security ← api`. `configengine` stays pure and reusable (P2 writes and P4 dynamic detection both build on it).

## Asset Model (configengine)

**Discovery scope (P1):**
- **Global (user scope):** `~/.claude/settings.json`, `~/.claude.json` (MCP servers + per-project state; machine-managed, read-only), `~/.claude/CLAUDE.md` + memory, `agents/`, `skills/`, `commands/`, `plugins/` (+ marketplace), hooks (within settings), `keybindings.json`.
- **Project (via project switcher):** `<proj>/.claude/settings.json`, `settings.local.json`, `.mcp.json`, `CLAUDE.md`, `agents/`, `skills/`, `commands/`.
- **Managed / enterprise policy files:** displayed read-only when present (they win precedence; show, don't edit).

**Asset types (the security-governed objects):** Settings, Permissions (allow/deny/ask — broken out because security-critical), Hooks (event→command), MCP servers (name/scope/transport/command/url/env), Skills, Commands, Agents, Plugins, CLAUDE.md/memory, Keybindings, Scripts referenced by hooks/commands.

Each asset carries: `id / type / scope (global·project·managed) / source_path / name / parsed fields / mtime / hash`.

**Precedence handling (P1):** discovery + parse + scope annotation + **duplicate detection** (same MCP installed in two places). Full effective-resolution (shadowed/conflict/ancestor + click-through) is deferred to P1.5/P2 — not required for security, higher complexity.

**configengine is read-only in P1.** All writes are P2.

## Security Detection Engine (security)

**Unified abstraction — `Detector` interface:**
```go
type Detector interface {
    ID() string                  // e.g. "baseline.permissions"
    Covers() []AssetType         // which assets it scans
    Scan(ctx, []Asset) (*ScanResult, error)
}
```
Detectors register with a `Registry`; a `Scan` orchestrator runs all matching detectors over a batch of assets and aggregates results into `Finding` records (`severity / asset / rule_id / evidence / remediation`). Adding a detection = implement the interface + register; no API/UI change. This is the core extensibility point and the productization learning centerpiece.

**Four P1 detectors:**

| Detector | Implementation | Data-driven? |
|---|---|---|
| **1. Config baseline** `baseline.*` | Go-native; reads settings.json + hooks + env | Yes — rule set YAML (dangerous-flag blacklist, over-broad permission patterns, dangerous env names) |
| **2. Content / prompt-injection** `content.injection` | Go-native; scans MCP tool descriptions / skills / commands / agents / CLAUDE.md / scripts as text | Yes — injection-pattern rule set (hidden instructions, obfuscation: zero-width / base64 / leetspeak / HTML comments — derived from CCO's 9 deobfuscation categories) |
| **3. Secret scan** `secret.*` | Subprocess: gitleaks | — |
| **4. Dependency vulnerability** `dep.*` | Subprocess: govulncheck (Go) / npm-audit (JS in skills/commands/plugins) | — |

**Rule sets (detectors 1 & 2)** are embedded YAML under `internal/security/rules/*.yaml`, bundled via `embed`. Each rule has `id / severity / description / pattern / deobfuscation[] / remediation`. Rules are **not user-editable in P1**; externalization is P4.

**Subprocess detector adapter (3 & 4):** implement the same `Detector` interface; shell out, parse scanner JSON output, normalize into `Finding`. Graceful degradation when a scanner is missing — the detector marks itself `unavailable` with a reason, scan continues.

**Baseline rule source:** self-authored P1 rule set, derived from Claude Code settings docs + CCO thinking. Claude Code's config semantics are harness-specific (`skipDangerousModePermissionPrompt`, `Bash(*)`, dangerous hook commands) and not expressible by generic scanners; authoring the rule set is itself part of the baseline-productization learning.

**Injection detection depth (P1):** text-level static only (pattern match + deobfuscation). No LLM-judge.

**Scan trigger (P1):** manual (user clicks "scan") + an automatic baseline quick-scan at startup. File-watcher-triggered incremental scans are deferred (avoid v1 state-management complexity).

## Health Score

A single aggregated metric compressing security posture into a comparable, explainable number. Three hard principles: **explainable** (every deduction traces to a concrete Finding), **monotonic** (fixing a Finding never lowers the score), **reproducible** (raw Findings recompute to the same score).

**Model:**
- **Asset base N** = count of in-scope assets. Each asset has a **base risk weight** `w(asset)` = the weight of its type `w_type` (MCP server and Hook highest, since they execute or inject into the prompt).
- **Each Finding** has a **deduction coefficient** `p_sev` by severity (Critical / High / Medium / Low; Critical largest).
- **Per-asset risk** `R(asset) = Σ(p_sev of that asset's Findings)`, capped at `Rmax`.
- **Score** `= 100 × (1 − Σ_assets(R(asset)·w(asset)) / (Rmax · Σ_assets w(asset)))`. This is 100 when there are no Findings (numerator 0) and 0 when every asset is at max risk `Rmax` (numerator = denominator). Mapped to 5 labeled bands: Excellent / Good / Fair / At-Risk / Critical.
- **Weighted deduction breakdown** shown: "MCP server 'xxx' · prompt-injection High · −6 pts". The user sees both the score and the why/what-to-fix.

**Weights (P1):** built-in default weight table (reasonable initial values per asset type / severity), adjustable in rule-set YAML but **not exposed in the UI for editing in P1** (deferred to P4 team baseline). Weights are explicit, auditable constants — not magic numbers.

**Learning value:** this "asset × Finding × severity → explainable aggregate score" is a general posture-quantification paradigm for security products (analogous to CVSS explainability, lighter) — a complete AI-security-metric productization exercise.

## Dashboard (P1 skeleton)

P1 dashboard is **read-only posture view only**, no writes. Content (layout mockups deferred to the UI implementation stage via the visual companion):

- **Health score card:** big Score number + band + in-session delta vs. the previous scan this session (P1 keeps only the current and one prior scan in memory; no persisted history).
- **Asset inventory:** counts by type, grouped Global/Project — plugin count, skill count, MCP server count, hook count, etc.
- **Risk summary:** Findings stacked bar by severity; heatmap by asset type; Top-N high-risk assets list.
- **Detector status:** each of the 4 detectors — enabled / available (subprocess present?) / last-scan finding count / last-scan duration.
- **Recent scans:** timeline + one-click "rescan".

**Health-score trend deferred to P1.5:** P1 does not persist scan history (each scan recomputes); trends need state storage, deferred to avoid v1 state complexity. All dashboard data comes from the in-memory current scan result.

## API & Local-Server Security (P1, read-only)

Gin HTTP+JSON, serves embedded SPA. Per-session token. The local server can read sensitive data (`~/.claude.json` MCP credentials, settings env tokens, CLAUDE.md secrets), so unauthorized access = credential/secret exposure — access control is mandatory, not optional.

**Bind policy (hybrid):**
- Config file `~/.claude-sentinel/config.yaml` sets `bind` (default `127.0.0.1`) and `port` (default random high). The config file lives **outside** `~/.claude/` to avoid sentinel scanning its own config / recursion.
- **Default loopback → remote access via SSH port forwarding** (`ssh -L <port>:127.0.0.1:<port> dev`); the server stays zero-network-exposed, auth fully reuses SSH. Sentinel prints the matching access method (and tunnel command) at startup.
- **When `bind` is non-loopback:** startup warning + **mandatory non-empty IP allowlist** (`allowed_cidrs`; empty → refuse to start unless `--i-know-its-risky`) + optional basic auth (password bcrypt-hashed, never plaintext).
- Token always required, delivered via **URL fragment** (`#token=`, not query param — keeps it out of server logs and Referer headers), validated on every API request.
- Strict CORS (no cross-origin) + `Host` header validation (DNS-rebinding defense).
- No remote network calls except npm/marketplace metadata where a feature requires it.

**P1 resources (all read-only except POST /scan):**

| Method | Path | Description |
|---|---|---|
| GET | `/api/assets` | Asset inventory (`?type=` `?scope=` filters) |
| GET | `/api/assets/:id` | Single asset detail (parsed fields + raw content preview) |
| POST | `/api/scan` | Trigger scan (`?detectors=baseline,secret` selective) → ScanResult |
| GET | `/api/scan/result` | Latest in-memory scan result (P1 not persisted) |
| GET | `/api/findings` | Findings list (`?severity=` `?asset=` filters) |
| GET | `/api/health` | Health score + weighted deduction breakdown |
| GET | `/api/dashboard` | Aggregate: asset counts + risk summary + detector status (one call) |
| GET | `/api/detectors` | Detector registry + availability status |
| GET | `/api/project` | Current selected project + selectable project list |
| POST | `/api/project` | Switch project |

**Scan concurrency:** `POST /api/scan` is synchronous (P1 asset scale is small); context timeout governs the whole scan so a hung subprocess can't stall it. Each subprocess detector's own timeout folds into the same context.

**Error convention:** JSON `{error: {code, message, details?}}`. A malformed asset file does not fail the whole scan — the asset is flagged `parse_error` and surfaced as a Finding; scan continues.

## Frontend (web/)

```
web/src/
  pages/        Dashboard / Assets / Findings / Settings
  components/   HealthScoreCard / AssetList / FindingTable / SeverityChart / DetectorStatus / ProjectSwitcher
  api/          API client (token injection, bind-aware base URL)
  store/        zustand (scan-result cache, asset tree)
```
Routes: Dashboard / Assets / Findings / Settings (read-only: show detectors, rule versions, scan settings). No edit pages in P1 (fully read-only).

**UI style (deliberately independent of both references):**
- shadcn/ui + Tailwind — copy-paste-owned, fully themeable, imposes no visual brand.
- Style direction: **SOC (security-ops-center) aesthetic** — dark-first, high information density, semantic color for risk (Critical=red / High=orange / Medium=amber / Low=teal), monospace for config/rule text, card-based posture.
- Deliberately unlike CCO's light web style and Studio's form-based settings style.

**Layout mockups are deferred:** this spec fixes only the *style direction* (SOC dark high-density + semantic color); concrete wireframes/mockups are produced during UI implementation via the visual companion, where the user makes the call. This keeps the spec focused and unblocked by undecided visual detail.

## Error Handling

- Malformed JSON in an existing file: surface the file with the parse error located; offer raw-preview; never write through a parse failure (P1 is read-only, so this is display-only).
- Failed subprocess invocation: surface stdout/stderr verbatim with the exact command run; mark detector unavailable.
- Missing scanner binary: detector reports `unavailable` + reason; overall scan succeeds with reduced coverage.
- Bind-policy violation (non-loopback + empty allowlist): refuse to start with a clear message and the `--i-know-its-risky` override hint.

## Testing

- **configengine:** unit tests against fixture home directories in temp dirs — the bulk of test effort (discovery, parse, scope annotation, duplicate detection, malformed-file handling).
- **security:** unit tests per detector with fixture assets (baseline rule matches, injection deobfuscation, score monotonicity/reproducibility); subprocess detectors tested with fixture scanner output (and an integration path that skips when the binary is absent).
- **api:** integration tests over the engine with fixtures (read endpoints, scan trigger, error convention).
- **Frontend:** a small number of Playwright e2e flows — load dashboard → trigger scan → see findings + score.

## Distribution

- Single binary; `sentinel` works without install. Frontend assets embedded (no build step at user's machine).
- External scanners (gitleaks/govulncheck/npm-audit) are optional runtime dependencies — detected at startup; missing ones degrade gracefully.

## Future (explicitly out of P1)

- P2 surgical config editing + backup/migration.
- P3 embedded agent + session history.
- P4 dynamic detection (live MCP, hash baseline, rug-pull monitoring) + team baseline rule externalization + scan-history persistence / health-score trends + multi-harness.
