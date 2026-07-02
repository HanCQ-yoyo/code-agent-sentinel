import { useEffect } from 'react'
import { useStore } from '../store'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { DetectorStatusList } from '../components/DetectorStatus'

export default function Dashboard() {
  const { scan, detectors, runScan, fetchDetectors, loading } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl">态势看板</h1>
        <button onClick={() => runScan()} disabled={loading} className="px-4 py-2 bg-bg-border rounded">{loading ? '扫描中…' : '重新扫描'}</button>
      </div>
      <div className="grid grid-cols-3 gap-4">
        <HealthScoreCard h={scan?.health_score} />
        <SeverityChart findings={scan?.findings ?? []} />
        <DetectorStatusList list={detectors} />
      </div>
    </div>
  )
}
