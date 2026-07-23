import { useEffect, useState } from 'react'
import { Alert, Empty } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { FindingTable } from '../components/FindingTable'
import { FindingDrawer } from '../components/FindingDrawer'
import { AgentMultiSelect } from '../components/AgentMultiSelect'
import type { Finding } from '../types'

export default function Findings() {
  const { t } = useTranslation()
  // Task 12:findings 改为 store 中的 fetchFindings 拉取(支持 ?agent=all 聚合),
  // 不再读 scan?.findings(单 agent 旧路径)。selectedAgents 变化 → 重新拉取。
  const { findings, selectedAgents, setSelectedAgents, error, detectors, fetchDetectors, fetchFindings } = useStore()
  const [selected, setSelected] = useState<Finding | null>(null)
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  useEffect(() => { fetchFindings() }, [fetchFindings, selectedAgents])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error ? <Alert type="error" message={t('common.loadFailed')} description={error} showIcon /> : null}
      {/* Task 12:多 agent 筛选器(空=全选聚合,与 Dashboard 一致)。 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <AgentMultiSelect value={selectedAgents} onChange={setSelectedAgents} />
      </div>
      {findings.length === 0 ? <Empty description={t('findings.notScannedHint')} /> : (
        <FindingTable
          findings={findings}
          detectors={detectors}
          onSelect={setSelected}
        />
      )}
      <FindingDrawer
        finding={selected}
        detectors={detectors}
        onClose={() => setSelected(null)}
      />
    </div>
  )
}
