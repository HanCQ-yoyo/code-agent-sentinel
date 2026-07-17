import { create } from 'zustand'
import { apiGet, apiPost, apiPut, apiDelete, AuthError } from '../api/client'
import type { Asset, Inventory, ScanResult, DetectorMeta, ScanSummary, ScanRecord, AgentsResponse, TreeNode, Project, DirTagsResponse, RawFile, PreviewResult, EditResult, SuppressionItem, BaselineResult, DetectorsConfig, DashboardData } from '../types'
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
  runScan: (detectors?: string) => Promise<void>
  fetchDetectors: () => Promise<void>
  fetchDetectorConfig: () => Promise<void>
  saveDetectorConfig: (cfg: DetectorsConfig) => Promise<boolean>
  fetchLatestScan: () => Promise<void>
  fetchHistory: () => Promise<void>
  fetchDashboard: () => Promise<void>
  fetchHistoryDetail: (id: string) => Promise<ScanRecord | undefined>
  deleteHistory: (id: string) => Promise<void>
  fetchAgents: () => Promise<void>
  fetchProjects: () => Promise<void>
  fetchTree: (tab: ProjectTab) => Promise<void>
  setActiveProjectTab: (tab: ProjectTab) => void
  fetchDirTags: () => Promise<void>
  saveDirTags: (overrides: DirTagsMap) => Promise<void>
  // 资产收藏/置顶:持久化到后端 config.yaml(跨重启/跨端口),非 localStorage。
  favorites: string[]
  fetchFavorites: () => Promise<void>
  saveFavorites: (ids: string[]) => Promise<void>
  // 语言:持久化到后端 /api/settings(跨重启/跨端口),i18n 同步。
  language: string
  fetchSettings: () => Promise<void>
  saveLanguage: (lang: string) => Promise<void>
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
  agents: null, tree: null, projects: [], activeProjectTab: { kind: 'global' },
  dirTagsDefaults: {}, dirTagsOverrides: {}, selectedTagFilter: null,
  favorites: [],
  language: '',
  previewResult: null, editError: null,
  suppressions: [],
  fetchAssets: async () => {
    const inv = await wrap(() => apiGet<Inventory>('/api/assets'), set)
    if (inv) set({ assets: inv })
  },
  runScan: async (d) => {
    set({ loading: true, error: null })
    const res = await wrap(() => apiPost<ScanResult>(d ? `/api/scan?detectors=${d}` : '/api/scan'), set)
    set({ scan: res ?? null, loading: false })
    // 扫描成功后刷新 Dashboard + History(新版 Dashboard 读 dashboard/history 而非 scan,
    // 不刷新则点"重新扫描"后看板无可见更新)。镜像 saveDetectorConfig 的不 await 模式。
    if (res) {
      get().fetchDashboard()
      get().fetchHistory()
    }
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
    const res = await wrap(() => apiGet<ScanRecord>('/api/scan/result'), set)
    if (res && res.findings) set({ scan: res })
  },
  fetchHistory: async () => {
    const list = await wrap(() => apiGet<ScanSummary[]>('/api/history'), set)
    if (list) set({ history: list })
  },
  fetchDashboard: async () => {
    const res = await wrap(() => apiGet<DashboardData>('/api/dashboard'), set)
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
    if (res) set({ agents: res })
  },
  fetchProjects: async () => {
    const res = await wrap(() => apiGet<{ projects: Project[] }>('/api/project'), set)
    if (res) set({ projects: res.projects ?? [] })
  },
  fetchTree: async (tab) => {
    const url = tab.kind === 'global'
      ? '/api/tree?scope=global'
      : `/api/tree?scope=project&path=${encodeURIComponent(tab.path)}`
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
  fetchSettings: async () => {
    const res = await wrap(() => apiGet<{ language: string; scan_interval: string; scan_enabled: boolean }>('/api/settings'), set)
    if (res) {
      set({ language: res.language })
      // 后端 config.language 层(落实 Task 11 brief 顺序 localStorage → 后端 → zh):
      // localStorage 已有用户偏好时尊重之(最高优先);否则用后端配置,后端空串回退 zh。
      // 后端 config.Language 默认空串,约定「空 = 回退 zh」(见 config.DefaultConfig)。
      if (!localStorage.getItem('sentinel.lang')) {
        await i18n.changeLanguage(res.language || 'zh')
      }
    }
  },
  saveLanguage: async (lang) => {
    const res = await wrap(() => apiPut<{ language: string }>('/api/settings', { language: lang }), set)
    if (res) set({ language: res.language })
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
}))
