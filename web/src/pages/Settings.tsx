import { useEffect } from 'react'
import { useStore } from '../store'
import { DetectorStatusList } from '../components/DetectorStatus'
export default function Settings() {
  const { detectors, fetchDetectors } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  return (
    <div className="space-y-4">
      <h1 className="text-xl">设置(只读)</h1>
      <DetectorStatusList list={detectors} />
      <div className="text-slate-500 text-sm">规则版本:P1 内置基线/注入规则集(embedded)</div>
    </div>
  )
}
