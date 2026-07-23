import { useState, useEffect } from 'react'
import { Modal, Radio, Select, Checkbox, Space, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'

const { Text } = Typography

interface Props {
  open: boolean
  onClose: () => void
  initialScope?: { type: string; path?: string } // 页面级入口预填
}

export function RescanModal({ open, onClose, initialScope }: Props) {
  const { t } = useTranslation()
  const { agents, selectedAgents, projects, detectors, runScan, loading } = useStore()
  const { assets } = useStore()
  const [scopeType, setScopeType] = useState('global')
  const [scopePath, setScopePath] = useState<string | undefined>(undefined)
  // Task 9:TEMPORARY shim — selectedAgents[0] ?? '' 替换 selectedAgent。
  // Task 13 将重建 RescanModal 支持 multi-select agent checkbox。
  const selectedAgent = selectedAgents[0] ?? ''
  const [agent, setAgent] = useState(selectedAgent)
  const [detIDs, setDetIDs] = useState<string[]>([])

  // 打开时重置/预填
  useEffect(() => {
    if (open) {
      setScopeType(initialScope?.type ?? 'global')
      setScopePath(initialScope?.path)
      setAgent(selectedAgent)
      setDetIDs((detectors ?? []).map(d => d.id)) // 默认全选
    }
  }, [open, initialScope, selectedAgent, detectors])

  const availDetectors = (detectors ?? []).map(d => ({ label: d.name ?? d.id, value: d.id, disabled: d.available === false }))

  const start = async () => {
    // 全选=不传(后端全量);否则传逗号分隔的 id 列表。
    const det = detIDs.length === (detectors ?? []).length ? undefined : detIDs.join(',')
    // Task 9:runScan 新签名 (agentIDs[], ...)。TEMPORARY:单选 → [agent](空 → []=后端回退全部 scan_enabled)。
    // Task 13 将加 multi-select agent checkbox,此处改传多 ID 数组。
    await runScan(agent ? [agent] : [], det, { type: scopeType, path: scopePath })
    onClose()
  }

  return (
    <Modal open={open} title={t('rescan.title')} onCancel={onClose} onOk={start} okText={t('rescan.start')} confirmLoading={loading} cancelText={t('common.cancel')}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <div>
          <Text strong>{t('rescan.scope')}</Text>
          <Radio.Group value={scopeType} onChange={(e) => setScopeType(e.target.value)} style={{ marginLeft: 8 }}>
            <Radio value="global">{t('rescan.scopeGlobal')}</Radio>
            <Radio value="project">{t('rescan.scopeProject')}</Radio>
            <Radio value="asset">{t('rescan.scopeAsset')}</Radio>
          </Radio.Group>
        </div>
        {scopeType === 'project' ? (
          <Select style={{ width: '100%' }} placeholder={t('rescan.selectProject')} value={scopePath}
            options={(projects ?? []).map(p => ({ value: p.path, label: p.name }))} onChange={setScopePath} />
        ) : null}
        {scopeType === 'asset' ? (
          <Select style={{ width: '100%' }} placeholder={t('rescan.selectAsset')} value={scopePath}
            options={(assets?.assets ?? []).map(a => ({ value: a.source_path, label: a.source_path }))}
            showSearch onChange={setScopePath} />
        ) : null}
        <div>
          <Text strong>{t('rescan.agent')}</Text>
          <Select style={{ width: 200, marginLeft: 8 }} value={agent}
            options={(agents?.agents ?? []).map(a => ({ value: a.id, label: a.name }))}
            onChange={setAgent} />
        </div>
        <div>
          <Text strong>{t('rescan.detectors')}</Text>
          <Checkbox.Group value={detIDs} onChange={(v) => setDetIDs(v as string[])} options={availDetectors} style={{ display: 'block', marginTop: 4 }} />
        </div>
      </Space>
    </Modal>
  )
}
