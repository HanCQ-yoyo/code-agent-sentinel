import { useEffect, useState } from 'react'
import { Alert, Empty } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { FindingTable } from '../components/FindingTable'
import { FindingDrawer, DispositionModal } from '../components/FindingDrawer'
import { AgentMultiSelect } from '../components/AgentMultiSelect'
import type { Finding } from '../types'

export default function Findings() {
  const { t } = useTranslation()
  // Task 12:findings 改为 store 中的 fetchFindings 拉取(支持 ?agent=all 聚合),
  // 不再读 scan?.findings(单 agent 旧路径)。selectedAgents 变化 → 重新拉取。
  const { findings, selectedAgents, setSelectedAgents, error, detectors, fetchDetectors, fetchFindings } = useStore()
  const [selected, setSelected] = useState<Finding | null>(null)
  // Task 9:列表操作列「处置」按钮触发的弹框 finding(与抽屉 selected 独立,复用 DispositionModal)。
  const [disposeFinding, setDisposeFinding] = useState<Finding | null>(null)
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  useEffect(() => { fetchFindings() }, [fetchFindings, selectedAgents])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error ? <Alert type="error" message={t('common.loadFailed')} description={error} showIcon /> : null}
      {/* Task 7:AgentMultiSelect 始终渲染(空态也要能切 agent 拉取),用 .filter-toolbar 共享样式非裸独占行。 */}
      <div className="filter-toolbar">
        <AgentMultiSelect value={selectedAgents} onChange={setSelectedAgents} />
      </div>
      {findings.length === 0 ? <Empty description={t('findings.notScannedHint')} /> : (
        <FindingTable
          findings={findings}
          detectors={detectors}
          onSelect={setSelected}
          onDispose={(f) => { if (f.fingerprint) setDisposeFinding(f) }}
        />
      )}
      <FindingDrawer
        finding={selected}
        detectors={detectors}
        onClose={() => setSelected(null)}
      />
      {/* Task 9:列表操作列触发的处置弹框(复用 FindingDrawer 导出的 DispositionModal)。
          disposeFinding 为 null 时不渲染(弹框 open 始终 true,由挂载控制开关)。
          key={disposeFinding.fingerprint} 对齐 Drawer 路径(FindingDrawer L266):强制在 finding
          切换时重挂载,防 useState(status/priority/note) 残留上一 finding 的脏 state。 */}
      {disposeFinding ? (
        <DispositionModal key={disposeFinding.fingerprint} finding={disposeFinding} open={!!disposeFinding} onClose={() => setDisposeFinding(null)} />
      ) : null}
    </div>
  )
}
