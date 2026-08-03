import { useState } from 'react'
import { Tabs, Tag } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../../store'
import type { RuleDTO } from '../../types'
import { RulesTable } from '../../components/RulesTable'
import { RuleDrawer } from '../../components/RuleDrawer'
import { SettingsAllowlist } from '../../components/SettingsAllowlist'

export default function InterceptRules() {
  const { t } = useTranslation()
  const { guardConfig } = useStore()
  const [subTab, setSubTab] = useState<string>('rules')
  const [editingRule, setEditingRule] = useState<RuleDTO | null>(null)
  const [drawerMode, setDrawerMode] = useState<'view' | 'edit' | 'create'>('view')

  const handleEdit = (r: RuleDTO) => { setEditingRule(r); setDrawerMode('edit') }
  const handleFork = (r: RuleDTO) => { setEditingRule(r); setDrawerMode('view') }
  const handleCreate = () => { setEditingRule(null); setDrawerMode('create') }
  const handleSaved = () => { setEditingRule(null); setDrawerMode('view') }
  const handleForked = (created: RuleDTO) => { setEditingRule(created); setDrawerMode('edit') }
  const handleDrawerClose = () => { setEditingRule(null); setDrawerMode('view') }

  const allowlistEnabled = guardConfig?.allowlist_enabled

  const tabItems = [
    {
      key: 'rules',
      label: t('settings.subRules'),
      children: <RulesTable domain="intercept" onCreate={handleCreate} onEdit={handleEdit} onFork={handleFork} />,
    },
    {
      key: 'allowlist',
      label: (
        <span>
          {t('guard.allowlistTitle')}
          <Tag
            style={{
              marginLeft: 8,
              fontSize: 'var(--fs-xs)',
              ...(allowlistEnabled
                ? { background: 'var(--sev-low-solid)', color: 'var(--badge-text)', border: 'none' }
                : {}),
            }}
          >
            {allowlistEnabled ? t('guard.allowlistOn') : t('guard.allowlistOff')}
          </Tag>
        </span>
      ),
      children: <SettingsAllowlist />,
    },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Tabs
        activeKey={subTab}
        onChange={setSubTab}
        items={tabItems}
      />
      <RuleDrawer
        rule={editingRule}
        mode={drawerMode}
        domain="intercept"
        onClose={handleDrawerClose}
        onSaved={handleSaved}
        onForked={handleForked}
      />
    </div>
  )
}
