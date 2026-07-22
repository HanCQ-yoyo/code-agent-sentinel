import { useEffect, useState } from 'react'
import { Alert, Empty } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { FindingTable } from '../components/FindingTable'
import { FindingDrawer } from '../components/FindingDrawer'
import type { Finding } from '../types'

export default function Findings() {
  const { t } = useTranslation()
  const { dashboard, scan, error, detectors, fetchDetectors } = useStore()
  const [selected, setSelected] = useState<Finding | null>(null)
  useEffect(() => { fetchDetectors() }, [fetchDetectors])

  // Task 10:与 Dashboard 一致的 agent 上下文行(scan 来自 dashboard.last_scan)。
  const agentLabel = dashboard?.agent_name ?? dashboard?.agent ?? '-'
  const lastScanTime = scan?.started_at ? new Date(scan.started_at).toLocaleString() : '-'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error ? <Alert type="error" message={t('common.loadFailed')} description={error} showIcon /> : null}
      <div style={{ marginBottom: 12, color: 'var(--text-secondary)', fontSize: 13 }}>
        {t('dashboard.agentContext', { agent: agentLabel, time: lastScanTime })}
      </div>
      {!scan ? <Empty description={t('findings.notScannedHint')} /> : (
        <FindingTable
          findings={scan.findings}
          startedAt={scan.started_at}
          detectors={detectors}
          onSelect={setSelected}
        />
      )}
      <FindingDrawer
        finding={selected}
        detectors={detectors}
        startedAt={scan?.started_at}
        onClose={() => setSelected(null)}
      />
    </div>
  )
}
