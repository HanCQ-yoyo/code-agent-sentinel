import { create } from 'zustand'
import { apiGet, apiPost, apiPut, apiDelete, AuthError } from '../api/client'
import type { Asset, Inventory, ScanResult, DetectorMeta, ScanSummary, ScanRecord, AgentsResponse, ScheduleStatus, TreeNode, Project, PinnedProject, DirTagsResponse, RawFile, PreviewResult, EditResult, SuppressionItem, BaselineResult, DetectorsConfig, DashboardData, AgentScanResult, Agent } from '../types'
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
  fetchAssets: () => Promise<void>
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
  deleteHistory: (id: string) => Promise<void>
  fetchAgents: () => Promise<void>
  // Task 9:替换 setSelectedAgent。空数组=全选聚合;[id]=单选;[id1,id2]=多选。
  setSelectedAgents: (ids: string[]) => void
  // Task 9:agentQuery 拼 ?agent= 查询串(全选→?agent=all;否则 ?agent=id1,id2)。
  // fetchDashboard/fetchLatestScan 用之;fetchAssets/fetchTree/fetchProjects 暂用单 agent 派生(Task 11 改)。
  agentQuery: () => string
  // Task 9:per-agent 扫描开关持久化(PUT /api/agents/:id)。成功后刷新 agents + scanEnabledAgents。
  saveAgentScanEnabled: (agentID: string, enabled: boolean) => Promise<void>
  fetchSchedules: () => Promise<void>
  createSchedule: (agent_id: string, interval: string, enabled: boolean) => Promise<boolean>
  updateSchedule: (agent_id: string, interval: string, enabled: boolean) => Promise<boolean>
  deleteSchedule: (agent_id: string) => Promise<boolean>
  fetchProjects: () => Promise<void>
  fetchTree: (tab: ProjectTab) => Promise<void>
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
  // P3 抑制(suppressions)与 baseline
  suppressions: SuppressionItem[]
  fetchSuppressions: () => Promise<void>
  addSuppression: (body: { fingerprint?: string; rule_id?: string; asset_id?: string; reason: string }) => Promise<boolean>
  deleteSuppression: (id: string) => Promise<void>
  generateBaseline: () => Promise<BaselineResult | undefined>
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
  agents: null, selectedAgents: [], scanEnabledAgents: [], schedules: [], tree: null, projects: [], activeProjectTab: { kind: 'global' },
  dirTagsDefaults: {}, dirTagsOverrides: {}, selectedTagFilter: null,
  favorites: [],
  pinnedProjects: [],
  language: '',
  scanEnabled: true,
  scanInterval: '',
  previewResult: null, editError: null,
  suppressions: [],
  rescanOpen: false,
  rescanInitial: undefined,
  // Task 9:agentQuery — 全选聚合(?agent=all)或逗号分隔 IDs。
  // fetchDashboard/fetchLatestScan 用之;fetchAssets/fetchTree/fetchProjects 暂用单 agent 派生(Task 11 正式改)。
  agentQuery: () => {
    const ids = get().selectedAgents
    if (ids.length === 0) return '?agent=all'
    return `?agent=${encodeURIComponent(ids.join(','))}`
  },
  fetchAssets: async () => {
    // TEMPORARY(Task 11 正式改):Assets 页按单 agent 派生(selectedAgents[0] ?? '' = 空 → 不带 query,
    // 后端回退首 agent)。Task 11 将加 agentID override 参数支持 per-agent tabs。
    const a = get().selectedAgents[0] ?? ''
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
      // 不再 set scan(旧 runScan 从单个 ScanResult 填 scan;新响应是数组无整体 findings)。
      // Dashboard 读 dashboard.last_scan;Findings 也读 scan(=dashboard.last_scan,由 fetchDashboard 同步)。
      // fetchDashboard 完成前 Findings 可能短暂显示旧数据 — Task 12 会重建 Findings 页处理此过渡。
      get().fetchDashboard()
      get().fetchHistory()
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
  deleteHistory: async (id) => {
    await wrap(() => apiDelete(`/api/history/${id}`), set)
    await get().fetchHistory()
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
  fetchProjects: async () => {
    // TEMPORARY(Task 11 正式改):project 列表按单 agent 派生。Task 11 加 agentID override。
    const a = get().selectedAgents[0] ?? ''
    const q = a ? `?agent=${encodeURIComponent(a)}` : ''
    const res = await wrap(() => apiGet<{ projects: Project[] }>(`/api/project${q}`), set)
    if (res) set({ projects: res.projects ?? [] })
  },
  fetchTree: async (tab) => {
    // TEMPORARY(Task 11 正式改):tree 按 &agent=<single>(仅非空时);scope 原有逻辑不变。
    // Task 11 将加 agentID override 参数支持 per-agent tabs。
    const a = get().selectedAgents[0] ?? ''
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
  // P3 抑制与 baseline:豁免列表 CRUD + baseline 生成(POST /api/baseline 跑全量扫描 + union 合并)。
  // 成功后不自动重扫(brief 未要求);用户下次手动扫描时 suppressed 状态即反映。
  fetchSuppressions: async () => {
    const res = await wrap(() => apiGet<{ items: SuppressionItem[] }>('/api/suppressions'), set)
    if (res) set({ suppressions: res.items ?? [] })
  },
  addSuppression: async (body) => {
    const r = await wrap(() => apiPost<SuppressionItem>('/api/suppressions', body), set)
    if (r) {
      // 刷新豁免列表(新条目入列);不重扫,finding 的 suppressed 状态下次扫描才变。
      get().fetchSuppressions()
      return true
    }
    return false
  },
  deleteSuppression: async (id) => {
    await wrap(() => apiDelete(`/api/suppressions/${encodeURIComponent(id)}`), set)
    await get().fetchSuppressions()
  },
  generateBaseline: async () => {
    return wrap(() => apiPost<BaselineResult>('/api/baseline'), set)
  },
  clearError: () => set({ error: null, authError: false }),
  openRescan: (initial) => set({ rescanOpen: true, rescanInitial: initial }),
  closeRescan: () => set({ rescanOpen: false, rescanInitial: undefined }),
}))
