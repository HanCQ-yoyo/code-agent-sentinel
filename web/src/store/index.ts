import { create } from 'zustand'
import { apiGet, apiPost, apiPut, apiDelete, AuthError } from '../api/client'
import type { Asset, Inventory, ScanResult, DetectorMeta, ScanSummary, ScanRecord, AgentsResponse, TreeNode, Project, DirTagsResponse, RawFile, PreviewResult, EditResult } from '../types'
import { type DirTag, type DirTagsMap } from '../lib/dirTags'

type ProjectTab = { kind: 'global' } | { kind: 'project'; path: string }

interface State {
  assets: Inventory | null
  scan: ScanResult | null
  detectors: DetectorMeta[]
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
  fetchLatestScan: () => Promise<void>
  fetchHistory: () => Promise<void>
  fetchHistoryDetail: (id: string) => Promise<ScanRecord | undefined>
  deleteHistory: (id: string) => Promise<void>
  fetchAgents: () => Promise<void>
  fetchProjects: () => Promise<void>
  fetchTree: (tab: ProjectTab) => Promise<void>
  setActiveProjectTab: (tab: ProjectTab) => void
  fetchDirTags: () => Promise<void>
  saveDirTags: (overrides: DirTagsMap) => Promise<void>
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
  assets: null, scan: null, detectors: [], history: [], loading: false, error: null, authError: false,
  agents: null, tree: null, projects: [], activeProjectTab: { kind: 'global' },
  dirTagsDefaults: {}, dirTagsOverrides: {}, selectedTagFilter: null,
  previewResult: null, editError: null,
  fetchAssets: async () => {
    const inv = await wrap(() => apiGet<Inventory>('/api/assets'), set)
    if (inv) set({ assets: inv })
  },
  runScan: async (d) => {
    set({ loading: true, error: null })
    const res = await wrap(() => apiPost<ScanResult>(d ? `/api/scan?detectors=${d}` : '/api/scan'), set)
    set({ scan: res ?? null, loading: false })
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
  fetchLatestScan: async () => {
    const res = await wrap(() => apiGet<ScanRecord>('/api/scan/result'), set)
    if (res && res.findings) set({ scan: res })
  },
  fetchHistory: async () => {
    const list = await wrap(() => apiGet<ScanSummary[]>('/api/history'), set)
    if (list) set({ history: list })
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
}))
