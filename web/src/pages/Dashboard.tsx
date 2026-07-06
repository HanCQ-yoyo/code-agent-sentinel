import { useEffect } from 'react'
import { useStore } from '../store'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { DetectorStatusList } from '../components/DetectorStatus'

export default function Dashboard() {
  // 扫描触发由 TopBar 统一承载(Task 7);本页只保留数据展示、错误提示与检测器拉取。
  const { scan, detectors, fetchDetectors, error, authError } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  return (
    <div className="space-y-4">
      {error && (
        <div className="border border-sev-critical text-sev-critical bg-bg-card rounded-lg p-3 text-sm break-all">
          扫描失败:{error}
        </div>
      )}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <HealthScoreCard h={scan?.health_score} />
        <SeverityChart findings={scan?.findings ?? []} />
        <DetectorStatusList list={detectors} />
      </div>
      {authError && <div className="text-sm text-text-muted">认证失效,请用带 token 的 URL 重新访问。</div>}
    </div>
  )
}
