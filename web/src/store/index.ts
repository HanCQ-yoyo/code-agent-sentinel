import { create } from 'zustand'
import { apiGet, apiPost, AuthError } from '../api/client'
import type { Inventory, ScanResult, DetectorStatus } from '../types'

interface State {
  assets: Inventory | null
  scan: ScanResult | null
  detectors: DetectorStatus[]
  loading: boolean
  error: string | null
  authError: boolean
  fetchAssets: () => Promise<void>
  runScan: (detectors?: string) => Promise<void>
  fetchDetectors: () => Promise<void>
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

export const useStore = create<State>((set) => ({
  assets: null, scan: null, detectors: [], loading: false, error: null, authError: false,
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
    const list = await wrap(() => apiGet<DetectorStatus[]>('/api/detectors'), set)
    if (list) set({ detectors: list })
  },
  switchProject: async (path) => {
    await wrap(() => apiPost(`/api/project?path=${encodeURIComponent(path)}`), set)
    const inv = await wrap(() => apiGet<Inventory>('/api/assets'), set)
    if (inv) set({ assets: inv })
  },
  clearError: () => set({ error: null, authError: false }),
}))
