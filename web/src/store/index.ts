import { create } from 'zustand'
import { apiGet, apiPost, apiDelete, AuthError } from '../api/client'
import type { Inventory, ScanResult, DetectorMeta, ScanSummary, ScanRecord } from '../types'

interface State {
  assets: Inventory | null
  scan: ScanResult | null
  detectors: DetectorMeta[]
  history: ScanSummary[]
  loading: boolean
  error: string | null
  authError: boolean
  fetchAssets: () => Promise<void>
  runScan: (detectors?: string) => Promise<void>
  fetchDetectors: () => Promise<void>
  fetchLatestScan: () => Promise<void>
  fetchHistory: () => Promise<void>
  fetchHistoryDetail: (id: string) => Promise<ScanRecord | undefined>
  deleteHistory: (id: string) => Promise<void>
  switchProject: (path: string) => Promise<void>
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
    // 后端 DetectorMeta 未直接含顶层 available/reason;从 engines 聚合(任一引擎可用则检测器可用)
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
  switchProject: async (path) => {
    await wrap(() => apiPost(`/api/project?path=${encodeURIComponent(path)}`), set)
    const inv = await wrap(() => apiGet<Inventory>('/api/assets'), set)
    if (inv) set({ assets: inv })
  },
  clearError: () => set({ error: null, authError: false }),
}))
