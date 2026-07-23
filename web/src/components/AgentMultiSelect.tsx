import { Select } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { agentMeta } from '../lib/agents'

// AgentMultiSelect:多 agent 筛选器,选项来自 store.scanEnabledAgents。
// value=[] 表示全选聚合(Dashboard 渲染所有 agent 的聚合视图);
// 非空 = 用户选中的 agent IDs(单选 → [id],多选 → [id1,id2])。
// 用于 Dashboard / Findings / History 页面顶部,Task 10 起接入。
export function AgentMultiSelect({ value, onChange }: { value: string[]; onChange: (ids: string[]) => void }) {
  const { t } = useTranslation()
  const scanEnabledAgents = useStore((s) => s.scanEnabledAgents)
  // 选项标签沿用 TopBar 旧风格:图标 + 显示名,保持视觉一致(agentMeta 已处理未知 agent 回退)。
  const options = scanEnabledAgents.map((a) => {
    const m = agentMeta(a)
    return { value: a.id, label: `${m.icon} ${m.label}` }
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
