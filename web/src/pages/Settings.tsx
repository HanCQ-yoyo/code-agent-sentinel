import { useEffect } from 'react'
import { useStore } from '../store'
import { DetectorStatusList } from '../components/DetectorStatus'
export default function Settings() {
  const { detectors, fetchDetectors } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  return (
    <div className="space-y-4">
      <p className="text-text-muted text-sm">只读模式 · 规则版本:P1 内置基线/注入规则集(embedded)</p>
      <DetectorStatusList list={detectors} />
    </div>
  )
}
