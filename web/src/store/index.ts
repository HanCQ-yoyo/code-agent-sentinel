import { create } from 'zustand'
import { apiGet, apiPost } from '../api/client'
import type { Inventory, ScanResult, DetectorStatus } from '../types'

interface State {
  assets: Inventory | null
  scan: ScanResult | null
  detectors: DetectorStatus[]
  loading: boolean
  error: string | null
  fetchAssets: () => Promise<void>
  runScan: (detectors?: string) => Promise<void>
  fetchDetectors: () => Promise<void>
  switchProject: (path: string) => Promise<void>
}
export const useStore = create<State>((set) => ({
  assets: null, scan: null, detectors: [], loading: false, error: null,
  fetchAssets: async () => set({ assets: await apiGet<Inventory>('/api/assets') }),
  runScan: async (d) => {
    set({ loading: true, error: null })
    try {
      set({ scan: await apiPost<ScanResult>(d ? `/api/scan?detectors=${d}` : '/api/scan') })
    } catch (e) {
      set({ error: String(e) })
    } finally {
      set({ loading: false })
    }
  },
  fetchDetectors: async () => set({ detectors: await apiGet<DetectorStatus[]>('/api/detectors') }),
  switchProject: async (path) => { await apiPost(`/api/project?path=${encodeURIComponent(path)}`); set({ assets: await apiGet<Inventory>('/api/assets') }) },
}))
