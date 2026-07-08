import { create } from 'zustand'
import { apiGet, apiPost, apiDelete, AuthError } from '../api/client'
import type { Inventory, ScanResult, DetectorMeta, ScanSummary, ScanRecord, AgentsResponse, TreeNode, Project } from '../types'

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
      const normalized = list.map(m => ({
        ...m,
        available: m.engines.length > 0 && m.engines.some(e => e.available),
        reason: m.engines.find(e => !e.available && e.reason)?.reason,
      }))
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
  clearError: () => set({ error: null, authError: false }),
}))
