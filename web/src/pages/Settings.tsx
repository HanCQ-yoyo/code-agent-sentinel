import { useEffect, useState } from 'react'
import { Card, Tabs, Switch, Input, Button, Modal, InputNumber, Radio, Form, Space, Typography, Popconfirm } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { DetectorMeta, DetectorsConfig, RuleDTO, RuleDomain, GuardConfig } from '../types'
import { RulesTable } from '../components/RulesTable'
import { RuleDrawer } from '../components/RuleDrawer'
import { DetectorPanel } from '../components/DetectorPanel'
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
  const { detectors, fetchDetectors, detectorConfig, fetchDetectorConfig, saveDetectorConfig, guardConfig, fetchGuardConfig, saveGuardConfig } = useStore()
  const [filter, setFilter] = useState<string | undefined>(undefined)
  const [draft, setDraft] = useState<DetectorsConfig | null>(null)
  const [saving, setSaving] = useState(false)
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  useEffect(() => { fetchDetectorConfig() }, [fetchDetectorConfig])
  useEffect(() => { if (detectorConfig) setDraft(detectorConfig) }, [detectorConfig])

  // Task 19:拦截总开关 + 高级弹框 state。
  // guardConfig 由 fetchGuardConfig 拉全量 6 键;advForm 为弹框编辑快照(含 enabled/policy,
  // 不暴露但随 form 回传,满足后端 PUT 顶层键校验)。patchAdv 局部更新弹框字段。
  useEffect(() => { fetchGuardConfig() }, [fetchGuardConfig])
  const [advOpen, setAdvOpen] = useState(false)
  const [advForm, setAdvForm] = useState<GuardConfig | null>(null)
  useEffect(() => { setAdvForm(guardConfig) }, [guardConfig])
  const patchAdv = (p: Partial<GuardConfig>) => setAdvForm((prev) => prev ? { ...prev, ...p } : prev)

  const selected = filter ? detectors.find((d) => d.id === filter) : undefined

  // Task 17:规则域切换(detect/intercept)+ 编辑/新建抽屉所有权。
  // RuleDrawer 有两个实例:① RulesTable 内部的 view-only 抽屉(行点击只读);② 此处的 edit/create 抽屉。
  // 两者不会同时打开同一条规则:RulesTable 的 view 抽屉由 selectedRule 控制,本处的 edit/create 抽屉
  //   由 editingRule+drawerMode 控制。用户点 Edit/Fork → onEdit/onFork → 关 view 抽屉(若开着)+ 开 edit 抽屉。
  //   实际中用户点操作按钮前不会同时开两个抽屉(操作按钮在表格行内,行点击才开 view 抽屉),故无需显式互斥。
  const [ruleDomain, setRuleDomain] = useState<RuleDomain>('detect')
  // Task 18:顶层 tab 受控,用于让拦截配置子 tab 内的操作强制 domain='intercept'。
  const [activeTab, setActiveTab] = useState('agents')
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

  // Task 18:拦截配置 tab 子 tab 内的规则操作:强制 domain='intercept'(与顶部 Segmented 解耦)。
  // 顶部「扫描配置」tab 的 ruleDomain Segmented 仍保留 detect/intercept 切换;拦截配置子 tab 固定 intercept 域,
  // 故此处 wrapper 在调用既有 handler 前先把 ruleDomain 置为 'intercept',使共享 RuleDrawer(domain={ruleDomain})在拦截域打开。
  const handleInterceptCreate = () => { setRuleDomain('intercept'); handleCreate() }
  const handleInterceptEdit = (r: RuleDTO) => { setRuleDomain('intercept'); handleEdit(r) }
  const handleInterceptFork = (r: RuleDTO) => { setRuleDomain('intercept'); handleFork(r) }
  // 扫描配置 tab 的「新建规则」:显式落到 detect 域。ruleDomain 是与拦截 tab 共享的 state,
  // 用户若先在拦截 tab 操作过(置 'intercept')再回扫描 tab 点新建,会误落到 intercept 域——
  // 此处先 setRuleDomain('detect') 隔离,保证扫描 tab 新建始终是检测规则。
  const handleDetectCreate = () => { setRuleDomain('detect'); handleCreate() }

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
      {/* 扫描配置 tab 只展示「检测规则」。拦截规则的查看/编辑收口到「拦截配置」tab。
          builtin 检测规则与拦截规则同源(db_init.go 把同一份 builtin 灌进 detect/intercept 两表,
          因 destructive 等规则双用:command 字段供运行时拦截 + 静态扫描 hook 资产,content/allow
          字段仅供静态扫描)。故此处不再做检测/拦截切换,避免展示同一份 builtin 造成混淆。
          新建规则显式落到 detect 域(handleDetectCreate),与拦截 tab 的 handleIntercept* 隔离。 */}
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button type="primary" onClick={handleDetectCreate}>{t('rulesManage.create')}</Button>
      </div>
      <RulesTable domain="detect" onEdit={handleEdit} onFork={handleFork} />
      {/* edit/create 抽屉已提升到页面级(见 return),确保任意顶层 tab 下都能挂载:
          antd Tabs 默认卸载非活动 pane,若抽屉留在本(detectors-rules)tab 内,
          切到 intercept-config tab 时抽屉不挂载、handleIntercept* 打不开抽屉。 */}
    </div>
  )

  // Task 18:拦截配置 tab:合并原「拦截配置」+「放行清单」两个顶层 tab。
  // 子 tab 上方放拦截总开关 + 高级按钮(Task 19)。两个子 tab:拦截规则(复用 RulesTable domain=intercept)/ 白名单(SettingsAllowlist)。
  // Task 19:总开关直接操作(带 Popconfirm 二次确认),高级按钮开弹框配 4 字段(mode/allowlist_enabled/deadline_ms/max_command_bytes)。
  //   enabled 字段即总开关本身,policy 随 advForm 回传不暴露。原 5 字段全量 form 已搬进高级弹框(Task 21 删旧组件)。
  const interceptConfig = (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* 总开关 + 高级按钮:总开关直接操作(带确认),高级按钮开弹框配 4 字段。 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
        <Space align="center">
          <Popconfirm
            title={guardConfig?.enabled ? t('guard.confirmDisable', { defaultValue: '确认关闭运行时拦截?' }) : t('guard.confirmEnable', { defaultValue: '确认开启运行时拦截?' })}
            okText={t('common.save')}
            cancelText={t('common.cancel')}
            onConfirm={async () => {
              if (!guardConfig) return
              await saveGuardConfig({ ...guardConfig, enabled: !guardConfig.enabled })
            }}
          >
            <Switch checked={guardConfig?.enabled ?? false} />
          </Popconfirm>
          <Typography.Text>{t('guard.enabled')}</Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('guard.enabledHint')}</Typography.Text>
        </Space>
        <Button onClick={() => setAdvOpen(true)}>{t('settings.advanced', { defaultValue: '高级' })}</Button>
      </div>
      <Tabs
        items={[
          { key: 'intercept-rules', label: t('settings.subRules'), children: (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                <Button type="primary" onClick={handleInterceptCreate}>{t('rulesManage.create')}</Button>
              </div>
              <RulesTable domain="intercept" onEdit={handleInterceptEdit} onFork={handleInterceptFork} />
            </div>
          ) },
          { key: 'allowlist', label: t('settings.subAllowlist'), children: <SettingsAllowlist /> },
        ]}
      />
    </div>
  )

  const items = [
    { key: 'agents', label: t('settings.agentsTab'), children: <SettingsAgents /> },
    { key: 'schedules', label: t('settings.schedulesTab'), children: <SettingsSchedules /> },
    { key: 'detectors-rules', label: t('settings.rulesConfig'), children: detectorsAndRules },
    // 拦截配置(合并原 guard + allowlist 两个 tab):拦截规则子 tab + 白名单子 tab。
    { key: 'intercept-config', label: t('settings.interceptConfigTab'), children: interceptConfig },
  ]

  return (
    <div>
      <Tabs items={items} activeKey={activeTab} onChange={setActiveTab} />
      {/* edit/create 抽屉(Settings 拥有,与 RulesTable 内 view-only 抽屉分离)。
          open 由 rule!==null || mode==='create' 控制(RuleDrawer 内部)。
          domain 传当前 ruleDomain:fork/create 都基于当前选中域。
          提升到页面级:antd Tabs 默认卸载非活动 pane,留在某 tab 内则其他 tab 下的
          handleIntercept* / handleCreate/Edit/Fork 设置了 state 但抽屉不挂载,打不开。 */}
      <RuleDrawer
        rule={editingRule}
        mode={drawerMode}
        domain={ruleDomain}
        onClose={handleDrawerClose}
        onSaved={handleSaved}
        onForked={handleForked}
      />
      {/* Task 19:高级弹框(Mode/allowlist_enabled/deadline_ms/max_command_bytes 4 字段;enabled 即总开关)。
          页面级挂载(同 RuleDrawer):避免 tab pane 卸载影响 Modal Form 状态。
          saveGuardConfig(advForm) 全量回传 6 键(advForm 从 guardConfig 快照,含 enabled/policy)。 */}
      <Modal
        open={advOpen}
        title={t('settings.advanced', { defaultValue: '高级' })}
        onCancel={() => setAdvOpen(false)}
        onOk={async () => {
          if (!advForm) return
          await saveGuardConfig(advForm)
          setAdvOpen(false)
        }}
        destroyOnClose
      >
        {advForm ? (
          <Form layout="vertical">
            <Form.Item label={t('guard.mode')}>
              <Radio.Group value={advForm.mode} onChange={(e) => patchAdv({ mode: e.target.value })}>
                <Radio value="strict">{t('guard.modeStrict')}</Radio>
                <Radio value="lenient">{t('guard.modeLenient')}</Radio>
              </Radio.Group>
            </Form.Item>
            <Form.Item label={t('guard.allowlistEnabled')}>
              <Space align="center">
                <Switch checked={advForm.allowlist_enabled} onChange={(v) => patchAdv({ allowlist_enabled: v })} />
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('guard.allowlistEnabledHint')}</Typography.Text>
              </Space>
            </Form.Item>
            <Form.Item label={t('guard.deadlineMs')}>
              <InputNumber min={0} value={advForm.deadline_ms} onChange={(v) => patchAdv({ deadline_ms: typeof v === 'number' ? v : 0 })} style={{ width: 200 }} />
            </Form.Item>
            <Form.Item label={t('guard.maxCommandBytes')}>
              <InputNumber min={0} value={advForm.max_command_bytes} onChange={(v) => patchAdv({ max_command_bytes: typeof v === 'number' ? v : 0 })} style={{ width: 200 }} />
            </Form.Item>
          </Form>
        ) : null}
      </Modal>
    </div>
  )
}
