import { useState, useEffect } from 'react'
import { Modal, Radio, Select, Checkbox, Space, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { AgentIcon } from './AgentIcon'

const { Text } = Typography

interface Props {
  open: boolean
  onClose: () => void
  initialScope?: { type: string; path?: string } // 页面级入口预填
}

export function RescanModal({ open, onClose, initialScope }: Props) {
  const { t } = useTranslation()
  const { agents, scanEnabledAgents, projects, detectors, runScan, loading } = useStore()
  const { assets } = useStore()
  const [scopeType, setScopeType] = useState('global')
  const [scopePath, setScopePath] = useState<string | undefined>(undefined)
  // Task 13:本地多选 agent 状态(非全局 selectedAgents 筛选器)。默认 = scanEnabledAgents。
  // 空 = 不传 ?agents=,后端回退到所有 scan_enabled agent。
  const [agentIDs, setAgentIDs] = useState<string[]>([])
  const [detIDs, setDetIDs] = useState<string[]>([])

  // 打开时重置/预填:scope 来自页面入口,agent 默认全选 scanEnabledAgents,检测器默认全选。
  useEffect(() => {
    if (open) {
      setScopeType(initialScope?.type ?? 'global')
      setScopePath(initialScope?.path)
      setAgentIDs(scanEnabledAgents.map(a => a.id))
      setDetIDs((detectors ?? []).map(d => d.id)) // 默认全选
    }
  }, [open, initialScope, scanEnabledAgents, detectors])

  const availDetectors = (detectors ?? []).map(d => ({ label: d.name ?? d.id, value: d.id, disabled: d.available === false }))

  // agent 多选 options:所有 agents(含扫描关闭的),label 带开关状态。
  // 允许选已关闭的 agent(强制扫一次),故不 disable。label 用 AgentIcon 品牌图标 + 名 + 开关标记。
  const agentOptions = (agents?.agents ?? []).map(a => ({
    value: a.id,
    label: <span style={{ whiteSpace: 'nowrap' }}><AgentIcon id={a.id} /> {a.name}{a.scan_enabled ? '' : t('rescan.scanOff')}</span>,
  }))

  const start = async () => {
    // 全选=不传(后端全量);否则传逗号分隔的 id 列表。
    const det = detIDs.length === (detectors ?? []).length ? undefined : detIDs.join(',')
    // Task 13:runScan 接收 agentIDs 数组(空 → 后端回退全部 scan_enabled)。
    await runScan(agentIDs, det, { type: scopeType, path: scopePath })
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
          <Select mode="multiple" style={{ width: '100%', marginTop: 4 }} value={agentIDs}
            options={agentOptions} onChange={setAgentIDs}
            placeholder={t('rescan.agent')} />
        </div>
        <div>
          <Text strong>{t('rescan.detectors')}</Text>
          <Checkbox.Group value={detIDs} onChange={(v) => setDetIDs(v as string[])} options={availDetectors} style={{ display: 'block', marginTop: 4 }} />
        </div>
      </Space>
    </Modal>
  )
}
