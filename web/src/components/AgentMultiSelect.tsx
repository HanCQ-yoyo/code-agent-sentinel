import { Select } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { agentMeta } from '../lib/agents'
import { AgentIcon } from './AgentIcon'

// AgentMultiSelect:多 agent 筛选器,选项来自 store.scanEnabledAgents。
// value=[] 表示全选聚合(Dashboard 渲染所有 agent 的聚合视图);
// 非空 = 用户选中的 agent IDs(单选 → [id],多选 → [id1,id2])。
// 用于 Dashboard / Findings / History 页面顶部,Task 10 起接入。
export function AgentMultiSelect({ value, onChange }: { value: string[]; onChange: (ids: string[]) => void }) {
  const { t } = useTranslation()
  const scanEnabledAgents = useStore((s) => s.scanEnabledAgents)
  // 选项 label 用 AgentIcon(品牌 logo)+ 显示名(ReactNode,antd Select label 支持)。
  const options = scanEnabledAgents.map((a) => {
    const m = agentMeta(a)
    return { value: a.id, label: <span style={{ whiteSpace: 'nowrap' }}><AgentIcon id={a.id} /> {m.label}</span> }
  })
  return (
    <Select
      mode="multiple"
      allowClear
      placeholder={t('dashboard.allAgents')}
      style={{ minWidth: 220 }}
      value={value}
      onChange={(ids: string[]) => onChange(ids)}
      options={options}
      maxTagCount="responsive"
    />
  )
}
