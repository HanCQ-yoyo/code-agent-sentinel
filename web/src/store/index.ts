import { create } from 'zustand'
import { apiGet, apiPost, apiPut, apiDelete, AuthError } from '../api/client'
import type { Asset, Inventory, ScanResult, DetectorMeta, ScanSummary, ScanRecord, AgentsResponse, ScheduleStatus, TreeNode, Project, PinnedProject, DirTagsResponse, RawFile, PreviewResult, EditResult, DetectorsConfig, DashboardData, AgentScanResult, Agent, Finding, FindingState, InterceptRecord, GuardConfig, AllowlistBody, RuleDTO, RuleDomain } from '../types'
import { type DirTag, type DirTagsMap } from '../lib/dirTags'
import i18n from '../i18n'

type ProjectTab = { kind: 'global' } | { kind: 'project'; path: string }

interface State {
  assets: Inventory | null
  scan: ScanResult | null
  dashboard: DashboardData | null
  detectors: DetectorMeta[]
  detectorConfig: DetectorsConfig | null
  history: ScanSummary[]
  // Stage R2:Intercept 拦截日志(/api/intercept)。list=列表;fetchIntercepts 可带 ?outcome= 过滤;
  // fetchInterceptDetail 拉单条(返回值交组件本地 state 渲染抽屉,不入 store);deleteIntercept 删单条。
  intercept: InterceptRecord[]
  // Stage R3 Task 12:Guard 配置 + 白名单(GET/PUT /api/guard/{config,allowlist})。
  // guardConfig 初始 null(设置页拦截配置 tab/高级弹框 fetch;null → 组件返回 null 不渲染)。
  // allowlist 初始 [](SettingsAllowlist 挂载时 fetch)。PUT 均要求全量(见 types.ts 注释)。
  guardConfig: GuardConfig | null
  allowlist: string[]
  // Task 14:规则 CRUD(detect/intercept 两域对称)。后端 /api/{detect,intercept}-rules(sqlite 单一真相)。
  // detectRules/interceptRules 为 null = 未加载;[] = 已加载但空(与 null 区分,避免组件渲染"加载中"时误判为空)。
  // loadingRuleId:启停/fork 防抖(同一 rule 同时只发一个请求),saveRule 也占用。
  detectRules: RuleDTO[] | null
  interceptRules: RuleDTO[] | null
  loadingRuleId: string | null
  loading: boolean
  error: string | null
  authError: boolean
  // agent
  agents: AgentsResponse | null
  // Task 9:多 agent 选择(纯视图状态,不持久化到后端)。
  // 空 = 全选聚合(默认,Dashboard 渲染所有 agent 的聚合视图);
  // 非空 = 用户筛选后的 agent IDs(单选 → [id],多选 → [id1,id2])。
  selectedAgents: string[]
  // scan_enabled !== false 的 agent 子集(供各页可选项 + 默认扫描目标)。
  scanEnabledAgents: Agent[]
  // 定时扫描任务列表(GET /api/schedules)
  schedules: ScheduleStatus[]
  // 目录树
  tree: TreeNode | null
  // 项目列表(供 Tabs)
  projects: Project[]
  // 当前选中的项目 tab(纯视图,默认全局)
  activeProjectTab: ProjectTab
  // 目录标签:默认 + 用户覆盖 + 当前筛选选中标签(空集=显示全部)
  dirTagsDefaults: DirTagsMap
  dirTagsOverrides: DirTagsMap
  // selectedTagFilter:null = 不过滤;否则只显示该标签(untagged 项在「全部」时显示,
  // 选 config/runtime 时隐藏非选中)。前端 Assets 用。
  selectedTagFilter: DirTag | null
  // Task 11:fetchAssets/fetchProjects/fetchTree 加可选 agentID 参数。
  // Assets L1 tab 显式传入 agentID;其他调用方(commitAssetEdit 等)不传 → 回退 selectedAgents[0] ?? ''。
  // 注意:这三个绝不走 agentQuery()——/api/assets、/api/tree、/api/project 不支持 ?agent=all 聚合,
  // 只接受单 ID 或空(后端回退首 agent)。agentID ?? selectedAgents[0] ?? '' 产出空或单 ID,永不为 all。
  fetchAssets: (agentID?: string) => Promise<void>
  // Task 9:runScan 改多 agent。agentIDs 空数组 → 后端回退到所有 scan_enabled agent。
  // 响应为 AgentScanResult[](数组,非单个 ScanResult),含每 agent 的 findings/health_score/error。
  // 注:新响应无整体 findings 数组(每 agent 各自有),故不再 set scan;fetchDashboard/fetchHistory 负责刷新视图。
  runScan: (agentIDs: string[], detectors?: string, scope?: { type: string; path?: string }) => Promise<AgentScanResult[] | undefined>
  fetchDetectors: () => Promise<void>
  fetchDetectorConfig: () => Promise<void>
  saveDetectorConfig: (cfg: DetectorsConfig) => Promise<boolean>
  fetchLatestScan: () => Promise<void>
  fetchHistory: () => Promise<void>
  fetchDashboard: () => Promise<void>
  fetchHistoryDetail: (id: string) => Promise<ScanRecord | undefined>
  // Task 12:Findings 页多 agent 视图的数据源。/api/findings 支持 ?agent=all 聚合(Task 8)
  // 也支持 ?agent=<id> 单 agent。agentID 显式传入(Assets/History 详情)优先;否则用 agentQuery()。
  // 全选 → ?agent=all 聚合(返回拼接 []Finding,每条带 agent_id);多选 → ?agent=id1,id2。
  findings: Finding[]
  fetchFindings: (agentID?: string) => Promise<void>
  // Task 12:Finding 治理字段 CRUD(/api/finding-state)。后端落盘到 ~/.claude-sentinel/finding_states.yaml,
  // API 读时把 status/priority/note 合并到 Finding 上(见 /api/findings 响应)。
  // 三个 action 成功后都 fetchFindings() 重拉(不带参 = 用当前 agentQuery,与 runScan 后刷新模式一致)。
  // setFindingState:POST /api/finding-state { fingerprint, status, priority?, note? } → upsert 单条。
  // bulkAccept:POST /api/finding-state/bulk-accept { fingerprints, source } → 批量标记 accepted。
  // resetFindingState:DELETE /api/finding-state/:fp → 清除该指纹的处置状态(回到 open 默认)。
  setFindingState: (fingerprint: string, status: string, priority?: string, note?: string) => Promise<void>
  bulkAccept: (fingerprints: string[]) => Promise<void>
  resetFindingState: (fingerprint: string) => Promise<void>
  deleteHistory: (id: string) => Promise<void>
  // Stage R2:Intercept 拦截日志只读闭环(列表+详情+删除)。
  // fetchIntercepts:GET /api/intercept?outcome=<value>(outcome 空则不带),成功后 set intercept。
  // fetchInterceptDetail:GET /api/intercept/:id,返回单条(用于详情抽屉展示)。
  // deleteIntercept:DELETE /api/intercept/:id,成功后重拉列表(与 deleteHistory 模式一致)。
  fetchIntercepts: (outcome?: string) => Promise<void>
  fetchInterceptDetail: (id: string) => Promise<InterceptRecord | undefined>
  deleteIntercept: (id: string) => Promise<void>
  // Stage R3 Task 12:Guard 配置 + 白名单 CRUD。API 内联(项目约定:复用 apiGet/apiPut + wrap,
  // 不建独立 *Api.ts)。PUT /api/guard/config 要求全量 6 键(见 types.ts 注释)——设置页高级弹框
  // fetch 全量后原地编辑、整体保存,保证不丢字段。saveGuardConfig 返回 boolean(与 saveDetectorConfig
  // 一致),供组件判定是否关闭 saving 状态(实际错误已由 wrap 写入 store.error)。
  fetchGuardConfig: () => Promise<void>
  saveGuardConfig: (cfg: GuardConfig) => Promise<boolean>
  fetchAllowlist: () => Promise<void>
  saveAllowlist: (list: string[]) => Promise<boolean>
  // Task 14:规则 CRUD actions(detect/intercept 两域对称,共用同一组 action + domain 参数分流)。
  // fetchDetectRules/fetchInterceptRules:拉列表 → set 对应 state(空数组兜底,null 区分未加载)。
  // saveRule:创建(POST)或更新(PUT)custom 规则。入参 source 可选,据此判定 create/update:
  //   - source 缺省(新草稿,Task 16 构造未落库的 draft 时不带 source)→ POST 创建
  //   - source 为 'builtin'/'custom'(已从 db 加载的规则)→ PUT 更新(后端对 builtin PUT 返回 409,UI 在 Task 16 灰掉编辑)
  //   不用 brief 原稿的 `rule.id !== rule.id`(恒 false 的 bug),改用 source 是否已赋值。
  //   入参类型用 Omit<RuleDTO,'source'> & { source?: 'builtin'|'custom' }:RuleDTO 本身保持 source 必填
  //   (db 加载形状),仅 saveRule 入参放宽为可选,与 validateRuleDraft(domain, rule: Partial<RuleDTO>)同思路。
  // toggleRule:PUT /enabled(builtin/custom 都可禁用,builtin 禁用走 override 表不改规则行)。
  // forkRule:POST /fork(只允许 builtin → custom,后端对 custom 返回 409)。返回新 RuleDTO 供调用方跳转。
  // deleteRule:DELETE(后端对 builtin 返回 409,UI 在 Task 16 不显删除按钮)。
  // validateRuleDraft:POST /validate(不落库,返回 {valid, errors};注意 HTTP 200 即使 valid=false,
  //   wrap 不会吞掉——后端永远 r.ok,errors 在 body 里)。返回值供 RuleDrawer 实时校验提示。
  fetchDetectRules: () => Promise<void>
  fetchInterceptRules: () => Promise<void>
  saveRule: (domain: RuleDomain, rule: Omit<RuleDTO, 'source'> & { source?: 'builtin' | 'custom' }) => Promise<void>
  toggleRule: (domain: RuleDomain, id: string, enabled: boolean) => Promise<void>
  forkRule: (domain: RuleDomain, id: string, newId: string) => Promise<RuleDTO | undefined>
  deleteRule: (domain: RuleDomain, id: string) => Promise<void>
  validateRuleDraft: (domain: RuleDomain, rule: Partial<RuleDTO>) => Promise<{ valid: boolean; errors: string[] } | undefined>
  fetchAgents: () => Promise<void>
  // Task 9:替换 setSelectedAgent。空数组=全选聚合;[id]=单选;[id1,id2]=多选。
  setSelectedAgents: (ids: string[]) => void
  // Task 9:agentQuery 拼 ?agent= 查询串(全选→?agent=all;否则 ?agent=id1,id2)。
  // fetchDashboard/fetchLatestScan 用之(这两个支持聚合 ?agent=all)。
  // fetchAssets/fetchTree/fetchProjects 不用之(Task 11 起接受 agentID 参数,单 agent 或空,不支持 all)。
  agentQuery: () => string
  // Task 9:per-agent 扫描开关持久化(PUT /api/agents/:id)。成功后刷新 agents + scanEnabledAgents。
  saveAgentScanEnabled: (agentID: string, enabled: boolean) => Promise<void>
  fetchSchedules: () => Promise<void>
  createSchedule: (agent_id: string, interval: string, enabled: boolean) => Promise<boolean>
  updateSchedule: (agent_id: string, interval: string, enabled: boolean) => Promise<boolean>
  deleteSchedule: (agent_id: string) => Promise<boolean>
  fetchProjects: (agentID?: string) => Promise<void>
  fetchTree: (tab: ProjectTab, agentID?: string) => Promise<void>
  setActiveProjectTab: (tab: ProjectTab) => void
  fetchDirTags: () => Promise<void>
  saveDirTags: (overrides: DirTagsMap) => Promise<void>
  // 资产收藏/置顶:持久化到后端 config.yaml(跨重启/跨端口),非 localStorage。
  favorites: string[]
  fetchFavorites: () => Promise<void>
  saveFavorites: (ids: string[]) => Promise<void>
  // 项目前置(右键置顶 + 颜色 + 排序):持久化到后端 /api/pinned-projects。
  pinnedProjects: PinnedProject[]
  fetchPinnedProjects: () => Promise<void>
  savePinnedProjects: (items: PinnedProject[]) => Promise<void>
  // 语言:持久化到后端 /api/settings(跨重启/跨端口),i18n 同步。
  language: string
  // 扫描总开关 + 默认间隔(无 per-agent schedule 时的回退):持久化到后端 /api/settings。
  // scanEnabled=false → ScheduleManager.Paused=true(后端 Task 2),所有定时任务暂停。
  // scanInterval 仅作回退默认,不覆盖已有 schedule.interval(后者以 /api/schedules 为准)。
  scanEnabled: boolean
  scanInterval: string
  fetchSettings: () => Promise<void>
  saveLanguage: (lang: string) => Promise<void>
  saveScanToggle: (enabled: boolean, interval: string) => Promise<boolean>
  setSelectedTagFilter: (tag: DirTag | null) => void
  fetchRaw: (path: string) => Promise<RawFile | undefined>
  // 拉单资产(含 content),供发现页详情抽屉按 finding.asset_id 展示资产文件内容。
  fetchAsset: (id: string) => Promise<Asset | undefined>
  // P2 写编辑
  previewResult: PreviewResult | null
  editError: string | null
  previewAssetEdit: (id: string, newContent: string, baseHash: string) => Promise<PreviewResult | undefined>
  commitAssetEdit: (id: string, newContent: string, baseHash: string) => Promise<EditResult | undefined>
  clearEditError: () => void
  // P3 抑制(suppressions)与 baseline:Task 15 删除 addSuppression/generateBaseline
  // 及其 fetchSuppressions/deleteSuppression/suppressions 状态——后端 /api/suppressions 的
  // POST/GET/DELETE 全部在 Task 11 删除,这些是死代码(无消费方,调用即 404)。
  // /api/baseline 重定义为 bulk-accept,用 bulkAccept action 取代。
  clearError: () => void
  // P3 Task 16:页面级 rescan 入口(项目右键 + 资产详情)预填 scope。
  // openRescan 传 initial 则预填(scopeType/scopePath),不传则默认 global。
  // closeRescan 关闭并清空 initial(避免下次打开残留上次预填)。
  rescanOpen: boolean
  rescanInitial: { type: string; path?: string } | undefined
  openRescan: (initial?: { type: string; path?: string }) => void
  closeRescan: () => void
}

const wrap = async <T>(fn: () => Promise<T>, set: (p: Partial<State>) => void): Promise<T | undefined> => {
  try {
    return await fn()
  } catch (e) {
    if (e instanceof AuthError) {
      set({ authError: true })
      return undefined
    }
    set({ error: String(e) })
    return undefined
  }
}

export const useStore = create<State>((set, get) => ({
  assets: null, scan: null, dashboard: null, detectors: [], detectorConfig: null, history: [], loading: false, error: null, authError: false,
  intercept: [],
  guardConfig: null, allowlist: [],
  detectRules: null, interceptRules: null, loadingRuleId: null,
  agents: null, selectedAgents: [], scanEnabledAgents: [], schedules: [], tree: null, projects: [], activeProjectTab: { kind: 'global' },
  dirTagsDefaults: {}, dirTagsOverrides: {}, selectedTagFilter: null,
  findings: [],
  favorites: [],
  pinnedProjects: [],
  language: '',
  scanEnabled: true,
  scanInterval: '',
  previewResult: null, editError: null,
  rescanOpen: false,
  rescanInitial: undefined,
  // Task 9:agentQuery — 全选聚合(?agent=all)或逗号分隔 IDs。
  // fetchDashboard/fetchLatestScan 用之(支持聚合);fetchAssets/fetchTree/fetchProjects 不用之(Task 11 起 agentID 参数,单 agent)。
  agentQuery: () => {
    const ids = get().selectedAgents
    if (ids.length === 0) return '?agent=all'
    return `?agent=${encodeURIComponent(ids.join(','))}`
  },
  fetchAssets: async (agentID?: string) => {
    // Task 11:agentID 显式传入(Assets L1 tab)优先;否则回退 selectedAgents[0] ?? ''(其他调用方兼容)。
    // 绝不用 agentQuery():/api/assets 不支持 ?agent=all,只接受单 ID 或空(后端回退首 agent)。
    const a = agentID ?? get().selectedAgents[0] ?? ''
    const q = a ? `?agent=${encodeURIComponent(a)}` : ''
    const inv = await wrap(() => apiGet<Inventory>(`/api/assets${q}`), set)
    if (inv) set({ assets: inv })
  },
  runScan: async (agentIDs, detectors, scope) => {
    set({ loading: true, error: null })
    // agentIDs 空数组 → 不带 ?agents=,后端回退到所有 scan_enabled agent。
    const params = new URLSearchParams()
    if (agentIDs.length > 0) params.set('agents', agentIDs.join(','))
    if (detectors) params.set('detectors', detectors)
    // scope=global 不传 query,后端缺省 global,等价旧行为。
    if (scope?.type && scope.type !== 'global') {
      params.set('scope', scope.type)
      if (scope.path) params.set('path', scope.path)
    }
    const q = params.toString() ? `?${params.toString()}` : ''
    // Task 6:响应为 AgentScanResult[](数组),非单个 ScanResult。
    // 新响应无整体 findings(每 agent 各自有),不再 set scan;fetchDashboard/fetchHistory 刷新视图。
    const res = await wrap(() => apiPost<AgentScanResult[]>(`/api/scan${q}`), set)
    set({ loading: false })
    if (res) {
      // 新响应是 AgentScanResult[] 无整体 findings,不再 set scan。重扫成功后需刷新四个视图:
      // - fetchDashboard:Dashboard 聚合视图 + scan(聚合模式 last_scan=undefined→null,见下)
      // - fetchHistory:History 列表(新扫描入列)
      // - fetchFindings:Findings 页数据源(仅 fetchFindings 填 store.findings;不调则 Findings 页
      //   显示重扫前的旧 findings —— RescanModal 是 overlay 不会 unmount Findings,故必须主动刷新)
      // - fetchLatestScan:Assets 风险徽章数据源(scan?.findings)。聚合模式 fetchDashboard 把 scan
      //   置 null(last_scan undefined),需重拉 latest 填回 scan,否则徽章消失。fetchLatestScan 在
      //   selectedAgents 空时不带 ?agent= → 后端返回全局最近扫描 → scan 恢复(Task 11/9 已知局限:
      //   聚合模式 scan 反映全局最近扫描而非 per-active-agent,此处仅恢复徽章,不改语义)。
      // 四个都是独立 GET,无共享状态突变,无 loading/runScan 触发,不会循环。
      get().fetchDashboard()
      get().fetchHistory()
      get().fetchFindings()   // 修 Findings 页陈旧(重扫后立即刷新)
      get().fetchLatestScan() // 修 Assets 风险徽章消失(聚合模式 scan 被 fetchDashboard 置 null,重拉)
    }
    return res
  },
  fetchDetectors: async () => {
    const list = await wrap(() => apiGet<DetectorMeta[]>('/api/detectors'), set)
    if (list) {
      const normalized = list.map(m => {
        const engines = m.engines ?? []
        return {
          ...m,
          available: engines.length > 0 && engines.some(e => e.available),
          reason: engines.find(e => !e.available && e.reason)?.reason,
        }
      })
      set({ detectors: normalized })
    }
  },
  fetchDetectorConfig: async () => {
    const res = await wrap(() => apiGet<DetectorsConfig>('/api/detectors/config'), set)
    if (res) set({ detectorConfig: res })
  },
  saveDetectorConfig: async (cfg) => {
    const res = await wrap(() => apiPut<DetectorsConfig>('/api/detectors/config', cfg), set)
    if (res) {
      set({ detectorConfig: res })
      // 配置改了:检测器 enabled/available 变化,刷新 detectors
      get().fetchDetectors()
      return true
    }
    return false
  },
  fetchLatestScan: async () => {
    // selectedAgents 空(全选聚合)时不带 ?agent= → 后端 getScanResult("") → LatestForAgent("")
    // 返回全局最近一条扫描(空串是 "所有 agent" 的合法语义)。
    // 注意:不能用 ?agent=all —— /api/scan/result 不解析 all,会按 AgentID=="all" 过滤返回 {}。
    // selectedAgents 非空(单/多 agent 筛选)时用 agentQuery()。
    const ids = get().selectedAgents
    const q = ids.length > 0 ? get().agentQuery() : ''
    const res = await wrap(() => apiGet<ScanRecord>(`/api/scan/result${q}`), set)
    if (res && res.findings) set({ scan: res })
  },
  fetchHistory: async () => {
    const list = await wrap(() => apiGet<ScanSummary[]>('/api/history'), set)
    if (list) set({ history: list })
  },
  fetchDashboard: async () => {
    // Task 9:dashboard 用 agentQuery()(全选→?agent=all 聚合;否则单/多 agent)。
    // 聚合模式无顶层 last_scan/agent/agent_name,改返回 is_aggregate + agent_scans。
    // 单 agent 模式仍返回 last_scan,scan 取 res.last_scan ?? null。
    const q = get().agentQuery()
    const res = await wrap(() => apiGet<DashboardData>(`/api/dashboard${q}`), set)
    if (res) {
      // 归一化 detectors 的 available/reason(与 fetchDetectors 一致)
      const detectors = (res.detectors ?? []).map(m => {
        const engines = m.engines ?? []
        return { ...m, available: engines.length > 0 && engines.some(e => e.available), reason: engines.find(e => !e.available && e.reason)?.reason }
      })
      set({ dashboard: { ...res, detectors }, scan: res.last_scan ?? null })
    }
  },
  fetchHistoryDetail: async (id) => {
    return wrap(() => apiGet<ScanRecord>(`/api/history/${id}`), set)
  },
  // Task 12:Findings 页多 agent 数据源。/api/findings 支持 ?agent=all 聚合(Task 8),
  // 返回拼接 []Finding(每条带 agent_id)。agentID 显式传入(详情页)优先;否则用 agentQuery()。
  fetchFindings: async (agentID?: string) => {
    const q = agentID != null ? `?agent=${encodeURIComponent(agentID)}` : get().agentQuery()
    const res = await wrap(() => apiGet<Finding[]>(`/api/findings${q}`), set)
    if (res) set({ findings: res })
  },
  // Task 12:Finding 治理字段 CRUD。用 apiPost/apiDelete + wrap(与项目其他 action 一致),
  // 不用 raw fetch + authHeaders(brief 写法,实际项目无此封装)。成功后 fetchFindings() 重拉
  // (后端合并 finding-state 到 Finding 上,前端 store.findings 整体更新,消费方自动重渲染)。
  setFindingState: async (fingerprint, status, priority, note) => {
    await wrap(() => apiPost<FindingState>('/api/finding-state', { fingerprint, status, priority, note }), set)
    await get().fetchFindings()
  },
  bulkAccept: async (fingerprints) => {
    await wrap(() => apiPost('/api/finding-state/bulk-accept', { fingerprints, source: 'bulk-accept' }), set)
    await get().fetchFindings()
  },
  resetFindingState: async (fingerprint) => {
    await wrap(() => apiDelete(`/api/finding-state/${encodeURIComponent(fingerprint)}`), set)
    await get().fetchFindings()
  },
  deleteHistory: async (id) => {
    await wrap(() => apiDelete(`/api/history/${id}`), set)
    await get().fetchHistory()
  },
  // Stage R2:Intercept 拦截日志。用 store-inline API 模式(与 history 一致),
  // 不建独立 interceptApi.ts(项目约定:API 调用内联在 store action,复用 wrap+apiGet/apiDelete)。
  fetchIntercepts: async (outcome?: string) => {
    const qs = outcome ? `?outcome=${encodeURIComponent(outcome)}` : ''
    const list = await wrap(() => apiGet<InterceptRecord[]>(`/api/intercept${qs}`), set)
    if (list) set({ intercept: list })
  },
  fetchInterceptDetail: async (id) => {
    return wrap(() => apiGet<InterceptRecord>(`/api/intercept/${encodeURIComponent(id)}`), set)
  },
  deleteIntercept: async (id) => {
    await wrap(() => apiDelete(`/api/intercept/${encodeURIComponent(id)}`), set)
    await get().fetchIntercepts()
  },
  // Stage R3 Task 12:Guard 配置 + 白名单。GET 全量 → 前端原地编辑 → PUT 全量(后端顶层键
  // 校验拒绝部分体,见 handlers_guard.go / handlers_allowlist.go)。saveGuardConfig/saveAllowlist
  // 返回 boolean(与 saveDetectorConfig 一致):wrap 返回 undefined 表示出错(已写入 store.error)。
  fetchGuardConfig: async () => {
    const res = await wrap(() => apiGet<GuardConfig>('/api/guard/config'), set)
    if (res) set({ guardConfig: res })
  },
  saveGuardConfig: async (cfg) => {
    const res = await wrap(() => apiPut<GuardConfig>('/api/guard/config', cfg), set)
    if (res) {
      set({ guardConfig: res })
      return true
    }
    return false
  },
  fetchAllowlist: async () => {
    const body = await wrap(() => apiGet<AllowlistBody>('/api/guard/allowlist'), set)
    if (body) set({ allowlist: body.allowlist ?? [] })
  },
  saveAllowlist: async (list) => {
    const body = await wrap(() => apiPut<AllowlistBody>('/api/guard/allowlist', { allowlist: list }), set)
    if (body) {
      set({ allowlist: body.allowlist ?? [] })
      return true
    }
    return false
  },
  // Task 14:规则 CRUD(detect/intercept 两域对称)。用 wrap+apiGet/apiPost/apiPut/apiDelete
  // (与项目其他 action 一致,不建独立 *Api.ts)。成功后重拉对应域列表(与 setFindingState→fetchFindings 同模式)。
  fetchDetectRules: async () => {
    const data = await wrap(() => apiGet<RuleDTO[]>('/api/detect-rules'), set)
    if (data) set({ detectRules: data })
  },
  fetchInterceptRules: async () => {
    const data = await wrap(() => apiGet<RuleDTO[]>('/api/intercept-rules'), set)
    if (data) set({ interceptRules: data })
  },
  // saveRule:创建或更新 custom 规则。入参 source 可选:缺省(新草稿,Task 16 未落库 draft)→ POST 创建;
  // 已赋值 'builtin'/'custom'(已从 db 加载)→ PUT 更新(后端对 builtin PUT 返回 409,UI 在 Task 16 灰掉编辑)。
  // 不用 brief 原稿 `rule.id !== rule.id`(恒 false 的 bug)。PUT 走 /:id(path id 即 rule.id,后端忽略 body.id)。
  saveRule: async (domain, rule) => {
    set({ loadingRuleId: rule.id })
    const isCreate = !rule.source
    if (isCreate) {
      await wrap(() => apiPost<RuleDTO>(`/api/${domain}-rules`, rule), set)
    } else {
      await wrap(() => apiPut<RuleDTO>(`/api/${domain}-rules/${encodeURIComponent(rule.id)}`, rule), set)
    }
    if (domain === 'detect') await get().fetchDetectRules()
    else await get().fetchInterceptRules()
    set({ loadingRuleId: null })
  },
  toggleRule: async (domain, id, enabled) => {
    set({ loadingRuleId: id })
    await wrap(() => apiPut(`/api/${domain}-rules/${encodeURIComponent(id)}/enabled`, { enabled }), set)
    if (domain === 'detect') await get().fetchDetectRules()
    else await get().fetchInterceptRules()
    set({ loadingRuleId: null })
  },
  forkRule: async (domain, id, newId) => {
    const result = await wrap(() => apiPost<RuleDTO>(`/api/${domain}-rules/${encodeURIComponent(id)}/fork`, { new_id: newId }), set)
    if (result) {
      if (domain === 'detect') await get().fetchDetectRules()
      else await get().fetchInterceptRules()
    }
    return result
  },
  deleteRule: async (domain, id) => {
    await wrap(() => apiDelete(`/api/${domain}-rules/${encodeURIComponent(id)}`), set)
    if (domain === 'detect') await get().fetchDetectRules()
    else await get().fetchInterceptRules()
  },
  validateRuleDraft: async (domain, rule) => {
    // 后端 validate 永远返回 HTTP 200(即使 valid=false),wrap 不会吞掉;
    // 返回 {valid, errors} 供 RuleDrawer 实时校验提示。
    return wrap(() => apiPost<{ valid: boolean; errors: string[] }>(`/api/${domain}-rules/validate`, rule), set)
  },
  fetchAgents: async () => {
    const res = await wrap(() => apiGet<AgentsResponse>('/api/agents'), set)
    if (res) {
      set({ agents: res })
      // Task 9:派生 scanEnabledAgents(scan_enabled !== false 的子集,供各页可选项)。
      const sea = (res.agents ?? []).filter(a => a.scan_enabled !== false)
      set({ scanEnabledAgents: sea })
      // selectedAgents 保持空(全选聚合,默认值),不回填——Task 10 Dashboard 默认全选聚合视图。
      // 旧 fetchAgents 在 selectedAgent 为空时回填 res.current/首 agent;新设计下空=全选聚合,
      // 回填会破坏默认全选语义。res.current 仍在 AgentsResponse 类型里(后端仍返回),但不再用于选择。
    }
  },
  setSelectedAgents: (ids) => set({ selectedAgents: ids }),
  // Task 9:per-agent 扫描开关持久化。PUT /api/agents/:id { scan_enabled: bool }。
  // apiPut 返回 { agent_id, scan_enabled }(后端 Task 4),成功后 fetchAgents 刷新 scanEnabledAgents。
  saveAgentScanEnabled: async (agentID, enabled) => {
    const ok = await wrap(() => apiPut<{ agent_id: string; scan_enabled: boolean }>(`/api/agents/${encodeURIComponent(agentID)}`, { scan_enabled: enabled }), set)
    if (ok) get().fetchAgents()
  },
  fetchSchedules: async () => {
    const res = await wrap(() => apiGet<{ schedules: ScheduleStatus[] }>('/api/schedules'), set)
    if (res) set({ schedules: res.schedules ?? [] })
  },
  createSchedule: async (agent_id, interval, enabled) => {
    const res = await wrap(() => apiPost('/api/schedules', { agent_id, interval, enabled }), set)
    if (res) await get().fetchSchedules()
    return !!res
  },
  updateSchedule: async (agent_id, interval, enabled) => {
    const res = await wrap(() => apiPut(`/api/schedules/${encodeURIComponent(agent_id)}`, { agent_id, interval, enabled }), set)
    if (res) await get().fetchSchedules()
    return !!res
  },
  deleteSchedule: async (agent_id) => {
    const res = await wrap(() => apiDelete(`/api/schedules/${encodeURIComponent(agent_id)}`), set)
    if (res) await get().fetchSchedules()
    return !!res
  },
  fetchProjects: async (agentID?: string) => {
    // Task 11:agentID 显式传入(Assets L1 tab)优先;否则回退 selectedAgents[0] ?? ''。
    const a = agentID ?? get().selectedAgents[0] ?? ''
    const q = a ? `?agent=${encodeURIComponent(a)}` : ''
    const res = await wrap(() => apiGet<{ projects: Project[] }>(`/api/project${q}`), set)
    if (res) set({ projects: res.projects ?? [] })
  },
  fetchTree: async (tab, agentID?: string) => {
    // Task 11:agentID 显式传入(Assets L1 tab)优先;否则回退 selectedAgents[0] ?? ''。
    // 仅非空时加 &agent=<single>;绝不走 agentQuery()(不支持 ?agent=all)。
    const a = agentID ?? get().selectedAgents[0] ?? ''
    const agentParam = a ? `&agent=${encodeURIComponent(a)}` : ''
    const url = tab.kind === 'global'
      ? `/api/tree?scope=global${agentParam}`
      : `/api/tree?scope=project&path=${encodeURIComponent(tab.path)}${agentParam}`
    const tree = await wrap(() => apiGet<TreeNode>(url), set)
    if (tree) set({ tree })
  },
  // setActiveProjectTab 仅 setState;fetchTree 由 Assets useEffect(activeProjectTab deps)统一驱动,
  // 避免阶段 B 遗留的双重 fetchTree 冗余(setActiveProjectTab + Assets effect 各调一次)。
  setActiveProjectTab: (tab) => {
    set({ activeProjectTab: tab })
  },
  fetchDirTags: async () => {
    const res = await wrap(() => apiGet<DirTagsResponse>('/api/dir-tags'), set)
    if (res) set({ dirTagsDefaults: res.defaults ?? {}, dirTagsOverrides: res.overrides ?? {} })
  },
  saveDirTags: async (overrides) => {
    const res = await wrap(() => apiPut<DirTagsResponse>('/api/dir-tags', { overrides }), set)
    if (res) set({ dirTagsOverrides: res.overrides ?? {} })
  },
  fetchFavorites: async () => {
    const res = await wrap(() => apiGet<{ favorites: string[] }>('/api/favorites'), set)
    if (res) set({ favorites: res.favorites ?? [] })
  },
  saveFavorites: async (ids) => {
    const res = await wrap(() => apiPut<{ favorites: string[] }>('/api/favorites', { favorites: ids }), set)
    if (res) set({ favorites: res.favorites ?? [] })
  },
  fetchPinnedProjects: async () => {
    const res = await wrap(() => apiGet<{ pinned_projects: PinnedProject[] }>('/api/pinned-projects'), set)
    if (res) set({ pinnedProjects: res.pinned_projects ?? [] })
  },
  savePinnedProjects: async (items) => {
    const res = await wrap(() => apiPut<{ pinned_projects: PinnedProject[] }>('/api/pinned-projects', { pinned_projects: items }), set)
    if (res) set({ pinnedProjects: res.pinned_projects ?? [] })
  },
  fetchSettings: async () => {
    const res = await wrap(() => apiGet<{ language: string; scan_interval: string; scan_enabled: boolean }>('/api/settings'), set)
    if (res) {
      set({ language: res.language, scanEnabled: res.scan_enabled, scanInterval: res.scan_interval })
      // 语言优先级:localStorage(用户主动切换,最高)> 后端 config.language > 默认 en。
      // localStorage 由 i18n detection 在 init 时读取(见 i18n/index.ts),此处不再重复应用。
      // 后端层仅在 localStorage 无偏好时生效:后端空串 → 保持默认 en(不 changeLanguage),
      // 后端非空(如 'zh')→ changeLanguage 到该值(不写 localStorage,避免覆盖用户偏好)。
      if (!localStorage.getItem('sentinel.lang') && res.language) {
        await i18n.changeLanguage(res.language)
      }
    }
  },
  saveLanguage: async (lang) => {
    const res = await wrap(() => apiPut<{ language: string }>('/api/settings', { language: lang }), set)
    if (res) set({ language: res.language })
    // 持久化双写:localStorage(i18n detection 读取,刷新生效)+ 后端(跨重启/跨端口生效)。
    // TopBar 切换处已 localStorage.setItem + i18n.changeLanguage,这里仅完成后端落盘。
  },
  // saveScanToggle:写后端 scan_enabled/scan_interval(后端 Task 2 传播到 ScheduleManager.Paused)。
  // scan_interval 仅作无 per-agent schedule 时的回退默认;已有任务的 interval 由 /api/schedules 改。
  saveScanToggle: async (enabled, interval) => {
    const res = await wrap(() => apiPut<{ scan_enabled: boolean; scan_interval: string }>('/api/settings', { scan_enabled: enabled, scan_interval: interval }), set)
    if (res) set({ scanEnabled: res.scan_enabled, scanInterval: res.scan_interval })
    return !!res
  },
  setSelectedTagFilter: (tag) => set({ selectedTagFilter: tag }),
  fetchRaw: async (path) => wrap(() => apiGet<RawFile>(`/api/raw?path=${encodeURIComponent(path)}`), set),
  fetchAsset: async (id) => wrap(() => apiGet<Asset>(`/api/assets/${encodeURIComponent(id)}`), set),
  // P2 写编辑:preview 走 POST,commit 走 PUT(后端 Task 9 注册为 PUT /api/assets/:id/content)。
  previewAssetEdit: async (id, newContent, baseHash) => {
    const r = await wrap(() => apiPost<PreviewResult>(`/api/assets/${encodeURIComponent(id)}/preview`, { new_content: newContent, base_hash: baseHash }), set)
    set({ previewResult: r ?? null })
    return r
  },
  commitAssetEdit: async (id, newContent, baseHash) => {
    const r = await wrap(() => apiPut<EditResult>(`/api/assets/${encodeURIComponent(id)}/content`, { new_content: newContent, base_hash: baseHash }), set)
    if (r) {
      // 刷新资产(new_findings 反映到资产 content/hash/mtime)
      get().fetchAssets()
    }
    return r
  },
  clearEditError: () => set({ editError: null }),
  clearError: () => set({ error: null, authError: false }),
  openRescan: (initial) => set({ rescanOpen: true, rescanInitial: initial }),
  closeRescan: () => set({ rescanOpen: false, rescanInitial: undefined }),
}))
