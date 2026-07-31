import { useState } from 'react'
import { Card, Button } from 'antd'
import { useTranslation } from 'react-i18next'
import type { RuleDTO } from '../../types'
import { RulesTable } from '../../components/RulesTable'
import { RuleDrawer } from '../../components/RuleDrawer'
import { SettingsAllowlist } from '../../components/SettingsAllowlist'

export default function InterceptRules() {
  const { t } = useTranslation()
  const [editingRule, setEditingRule] = useState<RuleDTO | null>(null)
  const [drawerMode, setDrawerMode] = useState<'view' | 'edit' | 'create'>('view')

  const handleEdit = (r: RuleDTO) => { setEditingRule(r); setDrawerMode('edit') }
  const handleFork = (r: RuleDTO) => { setEditingRule(r); setDrawerMode('view') }
  const handleCreate = () => { setEditingRule(null); setDrawerMode('create') }
  const handleSaved = () => { setEditingRule(null); setDrawerMode('view') }
  const handleForked = (created: RuleDTO) => { setEditingRule(created); setDrawerMode('edit') }
  const handleDrawerClose = () => { setEditingRule(null); setDrawerMode('view') }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 拦截规则 */}
      <Card title={t('settings.subRules')}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button type="primary" onClick={handleCreate}>{t('rulesManage.create')}</Button>
          </div>
          <RulesTable domain="intercept" onEdit={handleEdit} onFork={handleFork} />
        </div>
      </Card>

      {/* 白名单 */}
      <SettingsAllowlist />

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
