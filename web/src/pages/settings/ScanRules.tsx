import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { RuleDTO } from '../../types'
import { RulesTable } from '../../components/RulesTable'
import { RuleDrawer } from '../../components/RuleDrawer'

export default function ScanRules() {
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
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <RulesTable domain="detect" onCreate={handleCreate} onEdit={handleEdit} onFork={handleFork} />
      <RuleDrawer
        rule={editingRule}
        mode={drawerMode}
        domain="detect"
        onClose={handleDrawerClose}
        onSaved={handleSaved}
        onForked={handleForked}
      />
    </div>
  )
}
