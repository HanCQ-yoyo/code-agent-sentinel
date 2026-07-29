import { useEffect, useState } from 'react'
import { Card, Tabs, Switch, Input, Button, Segmented } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { DetectorMeta, DetectorsConfig, RuleDTO, RuleDomain } from '../types'
import { RulesTable } from '../components/RulesTable'
import { RuleDrawer } from '../components/RuleDrawer'
import { DetectorPanel } from '../components/DetectorPanel'
import { SettingsGuard } from '../components/SettingsGuard'
import { SettingsAllowlist } from '../components/SettingsAllowlist'
import { SettingsAgents } from './SettingsAgents'
import { SettingsSchedules } from './SettingsSchedules'

// 检测器配置控件:启用开关 + 二进制路径(rules 仅开关;secret 单二进制;dep 每引擎一行)。
function DetectorConfigControls({ d, draft, setDraft }: { d: DetectorMeta; draft: DetectorsConfig | null; setDraft: (c: DetectorsConfig) => void }) {
  const { t } = useTranslation()
  if (!draft) return null
  const patch = (p: Partial<DetectorsConfig>) => setDraft({ ...draft, ...p })
  return (
    <Card size="small" style={{ background: 'var(--surface-2)', marginTop: 8 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {d.id === 'rules' ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontSize: 13 }}>{t('common.enabled')}</span>
            <Switch size="small" checked={draft.rules.enabled} onChange={(v) => patch({ rules: { ...draft.rules, enabled: v } })} />
          </div>
        ) : null}
        {d.id === 'secret' ? (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13 }}>{t('common.enabled')}</span>
              <Switch size="small" checked={draft.secret.enabled} onChange={(v) => patch({ secret: { ...draft.secret, enabled: v } })} />
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13, width: 80 }}>{t('settings.binaryPath')}</span>
              <Input size="small" style={{ flex: 1 }} placeholder={t('settings.defaultBinary', { name: 'gitleaks' })} value={draft.secret.binary} onChange={(e) => patch({ secret: { ...draft.secret, binary: e.target.value } })} />
            </div>
          </>
        ) : null}
        {d.id === 'dep' ? (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13 }}>{t('common.enabled')}</span>
              <Switch size="small" checked={draft.dep.enabled} onChange={(v) => patch({ dep: { ...draft.dep, enabled: v } })} />
            </div>
            {['npm', 'govulncheck'].map((name) => {
              const e = draft.dep.engines[name] ?? { enabled: true, binary: '' }
              return (
                <div key={name} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Switch size="small" checked={e.enabled} onChange={(v) => patch({ dep: { ...draft.dep, engines: { ...draft.dep.engines, [name]: { ...e, enabled: v } } } })} />
                  <span style={{ fontSize: 13, width: 100 }}>{name}</span>
                  <Input size="small" style={{ flex: 1 }} placeholder={t('settings.defaultBinary', { name })} value={e.binary} onChange={(ev) => patch({ dep: { ...draft.dep, engines: { ...draft.dep.engines, [name]: { ...e, binary: ev.target.value } } } })} />
                </div>
              )
            })}
          </>
        ) : null}
      </div>
    </Card>
  )
}

export default function Settings() {
  const { t } = useTranslation()
  const { detectors, fetchDetectors, detectorConfig, fetchDetectorConfig, saveDetectorConfig } = useStore()
  const [filter, setFilter] = useState<string | undefined>(undefined)
  const [draft, setDraft] = useState<DetectorsConfig | null>(null)
  const [saving, setSaving] = useState(false)
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  useEffect(() => { fetchDetectorConfig() }, [fetchDetectorConfig])
  useEffect(() => { if (detectorConfig) setDraft(detectorConfig) }, [detectorConfig])

  const selected = filter ? detectors.find((d) => d.id === filter) : undefined

  // Task 17:规则域切换(detect/intercept)+ 编辑/新建抽屉所有权。
  // RuleDrawer 有两个实例:① RulesTable 内部的 view-only 抽屉(行点击只读);② 此处的 edit/create 抽屉。
  // 两者不会同时打开同一条规则:RulesTable 的 view 抽屉由 selectedRule 控制,本处的 edit/create 抽屉
  //   由 editingRule+drawerMode 控制。用户点 Edit/Fork → onEdit/onFork → 关 view 抽屉(若开着)+ 开 edit 抽屉。
  //   实际中用户点操作按钮前不会同时开两个抽屉(操作按钮在表格行内,行点击才开 view 抽屉),故无需显式互斥。
  const [ruleDomain, setRuleDomain] = useState<RuleDomain>('detect')
  const [editingRule, setEditingRule] = useState<RuleDTO | null>(null)
  const [drawerMode, setDrawerMode] = useState<'view' | 'edit' | 'create'>('view')

  // onEdit:custom 规则点编辑 → 进 edit 模式打开抽屉。
  const handleEdit = (r: RuleDTO) => { setEditingRule(r); setDrawerMode('edit') }
  // onFork:builtin 规则点「复制为自定义」→ 进 view 模式打开抽屉(RuleDrawer view 模式自带 fork Modal 入口)。
  //   选 view 而非 edit:builtin 不能直接 edit(RuleDrawer 对 builtin edit 会禁用 Save + 只读编辑器);
  //   view 模式顶部有 Fork 按钮,用户填 new_id → forkRule → onForked 切到新 custom 的 edit 态。
  const handleFork = (r: RuleDTO) => { setEditingRule(r); setDrawerMode('view') }
  // 新建规则:create 模式(rule=null)。
  const handleCreate = () => { setEditingRule(null); setDrawerMode('create') }
  // 保存成功:关抽屉(saveRule 内部已重拉列表,store state 已更新)。
  const handleSaved = () => { setEditingRule(null); setDrawerMode('view') }
  // fork 成功:切到新 custom 规则的 edit 态(created 为新创建的 RuleDTO,source=custom、can_edit=true)。
  const handleForked = (created: RuleDTO) => { setEditingRule(created); setDrawerMode('edit') }
  const handleDrawerClose = () => { setEditingRule(null); setDrawerMode('view') }

  const detectorsAndRules = (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <DetectorPanel detectors={detectors} selectedId={filter} onSelect={setFilter} />
      {selected ? (
        <>
          <DetectorConfigControls d={selected} draft={draft} setDraft={setDraft} />
          <div>
            <Button type="primary" size="small" loading={saving} onClick={async () => {
              if (!draft) return
              setSaving(true)
              const ok = await saveDetectorConfig(draft)
              setSaving(false)
              if (!ok) { /* error 已由 wrap 写入 store.error */ }
            }}>{t('settings.saveConfig')}</Button>
          </div>
        </>
      ) : null}
      {/* Task 17:域切换 Segmented(detect/intercept)+ 新建规则按钮。
          RulesTable 按 ruleDomain 拉对应域规则;切换域时 RulesTable 内部 useEffect 重拉。
          新建规则按钮:打开 create 模式抽屉(rule=null)。 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
        <Segmented
          value={ruleDomain}
          onChange={(v) => setRuleDomain(v as RuleDomain)}
          options={[
            { label: t('rulesManage.detectRules'), value: 'detect' },
            { label: t('rulesManage.interceptRules'), value: 'intercept' },
          ]}
        />
        <Button type="primary" onClick={handleCreate}>{t('rulesManage.create')}</Button>
      </div>
      <RulesTable domain={ruleDomain} onEdit={handleEdit} onFork={handleFork} />
      {/* edit/create 抽屉(Settings 拥有,与 RulesTable 的 view 抽屉分离)。
          open 由 rule!==null || mode==='create' 控制(RuleDrawer 内部)。
          domain 传当前 ruleDomain:fork/create 都基于当前选中域。 */}
      <RuleDrawer
        rule={editingRule}
        mode={drawerMode}
        domain={ruleDomain}
        onClose={handleDrawerClose}
        onSaved={handleSaved}
        onForked={handleForked}
      />
    </div>
  )

  const items = [
    { key: 'agents', label: t('settings.agentsTab'), children: <SettingsAgents /> },
    { key: 'schedules', label: t('settings.schedulesTab'), children: <SettingsSchedules /> },
    { key: 'detectors-rules', label: t('settings.rulesConfig'), children: detectorsAndRules },
    // Stage R3 Task 12:Guard 运行时拦截配置 + 放行清单编辑面板。
    { key: 'guard', label: t('settings.guardTab'), children: <SettingsGuard /> },
    { key: 'allowlist', label: t('settings.allowlistTab'), children: <SettingsAllowlist /> },
  ]

  return (
    <div>
      <Tabs items={items} />
    </div>
  )
}
