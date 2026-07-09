import { useEffect } from 'react'
import { Alert, Empty } from 'antd'
import { useStore } from '../store'
import { FindingTable } from '../components/FindingTable'

export default function Findings() {
  const { scan, error, detectors, fetchDetectors } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error ? <Alert type="error" message="加载失败" description={error} showIcon /> : null}
      {!scan ? <Empty description="尚未扫描 · 去看板点击‘重新扫描’" /> : (
        <FindingTable findings={scan.findings} startedAt={scan.started_at} detectors={detectors} />
      )}
    </div>
  )
}
