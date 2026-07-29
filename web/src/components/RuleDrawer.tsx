import { useState, useEffect, useRef, useCallback } from 'react'
import { Drawer, Descriptions, Typography, Badge as AntBadge, Button, Space, Modal, Input, Alert, message, Form, Select, Switch, Collapse } from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import type { RuleDTO, RuleDomain, Severity } from '../types'
import { useStore } from '../store'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { SEVERITY_LABEL_KEY, SEVERITY_ORDER } from '../lib/severity'
import { ruleName } from '../lib/i18n-names'
import { MatchTreeEditor } from './MatchTreeEditor'
import { matchMapToTree, treeToMatchMap, newLeaf, isUnsupported, type MatchTreeNode } from '../lib/match-tree'

interface RuleDrawerProps {
  rule: RuleDTO | null
  // view=只读 Descriptions;edit=编辑已存在 custom 规则;create=新建 custom 规则。
  mode: 'view' | 'edit' | 'create'
  domain: RuleDomain
  onClose: () => void
  // 保存成功后回调(Settings 关抽屉 + 列表已由 saveRule 内部重拉)。
  onSaved?: () => void
  // fork 成功后回调,传入新创建的 custom RuleDTO(Settings 可切到编辑态打开新规则)。
  onForked?: (created: RuleDTO) => void
}

// 防抖 effect hook:value 变化后 delayMs 内无新变化才触发 effect。
// 自实现(项目无现成 debounce hook),ref+setTimeout 模式,最小自包含。
function useDebouncedEffect(
  value: string,
  delayMs: number,
  effect: () => void | (() => void),
) {
  const effectRef = useRef(effect)
  useEffect(() => { effectRef.current = effect })
  useEffect(() => {
    // 关键:effect 返回的 cleanup 必须延迟到 React 清理阶段(下次防抖/unmount)调用,
    // 不能在 effect 同步执行后立即调用——否则 stale 立即被置 true,异步 validateRuleDraft
    // 的 .then 永远命中 stale=true,实时校验结果永远不展示(Task 16 review Critical #1)。
    let cleanupFn: (() => void) | undefined
    const handle = setTimeout(() => {
      const ret = effectRef.current()
      if (typeof ret === 'function') cleanupFn = ret
    }, delayMs)
    return () => {
      clearTimeout(handle)
      if (typeof cleanupFn === 'function') cleanupFn()
    }
    // value 变化触发防抖;delayMs 稳定常量。effect 经 ref 取最新,不入依赖(否则每次重渲染都重置定时器)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, delayMs])
}

// 表单 draft:RuleDTO 可编辑字段的结构化状态。source/enabled/can_edit/domain 为派生/管理态,
// id 在 edit 模式只读,故 draft 只持用户可改字段。matchTree 是 match map 的树形镜像。
interface RuleDraft {
  id: string
  severity: string
  asset_type: string
  matchTree: MatchTreeNode | null    // null = match map 不支持形状(降级只读)
  matchMap: Record<string, unknown>  // 原始 match(降级块显示 + 不支持形状保存回写)
  description: string
  remediation: string
  dotall: boolean
  deobfuscation: string[]
  post_exclude: string[]
  pathsInclude: string[]
  pathsExclude: string[]
  metadata: { key: string; value: string }[]  // 动态行(序列化为 map)
  enabled: boolean
}

// RuleDTO → draft。match 用 matchMapToTree 解析;不支持形状 → matchTree=null(降级)。
function ruleToDraft(rule: RuleDTO): RuleDraft {
  const matchMap = (rule.match && Object.keys(rule.match).length > 0) ? rule.match : {}
  const matchTree = Object.keys(matchMap).length > 0 ? matchMapToTree(matchMap) : newLeaf()
  return {
    id: rule.id,
    severity: rule.severity,
    asset_type: rule.asset_type,
    matchTree,
    matchMap,
    description: rule.description ?? '',
    remediation: rule.remediation ?? '',
    dotall: rule.dotall ?? false,
    deobfuscation: rule.deobfuscation ?? [],
    post_exclude: rule.post_exclude ?? [],
    pathsInclude: rule.paths?.include ?? [],
    pathsExclude: rule.paths?.exclude ?? [],
    metadata: rule.metadata
      ? Object.entries(rule.metadata).map(([k, v]) => ({ key: k, value: String(v) }))
      : [],
    enabled: rule.enabled,
  }
}

// draft → RuleDTO map(用于 validate / save)。空字段不写入(对齐旧 ruleToYamlObj 序列化规则)。
function draftToRuleDTO(draft: RuleDraft, mode: 'edit' | 'create'): Partial<RuleDTO> {
  const match = draft.matchTree ? treeToMatchMap(draft.matchTree) : draft.matchMap
  const dto: Record<string, unknown> = {
    id: draft.id,
    severity: draft.severity,
    asset_type: draft.asset_type,
    match,
    enabled: draft.enabled,
  }
  if (draft.description) dto.description = draft.description
  if (draft.remediation) dto.remediation = draft.remediation
  if (draft.dotall) dto.dotall = draft.dotall
  if (draft.deobfuscation.length) dto.deobfuscation = draft.deobfuscation
  if (draft.post_exclude.length) dto.post_exclude = draft.post_exclude
  if (draft.pathsInclude.length || draft.pathsExclude.length) {
    dto.paths = { include: draft.pathsInclude, exclude: draft.pathsExclude }
  }
  const metaObj: Record<string, unknown> = {}
  for (const m of draft.metadata) {
    if (m.key) metaObj[m.key] = isNaN(Number(m.value)) ? m.value : Number(m.value)
  }
  if (Object.keys(metaObj).length) dto.metadata = metaObj
  if (mode === 'create') dto.enabled = draft.enabled
  return dto as Partial<RuleDTO>
}

// 规则详情抽屉:支持 view(只读)/edit(编辑 custom)/create(新建 custom)三态 + builtin fork。
// Task 16:从旧 FlatRule(含 detector/syntax/valid/source_file/project_path)迁移到 RuleDTO。
//   - view 模式:Descriptions 展示 RuleDTO 字段 + match 摘要(规则语法已并入 match,无独立 syntax)。
//   - edit/create 模式:结构化表单(基础区 + match 树编辑器 + 高级折叠区)+ 防抖实时校验(POST /validate)+ Save/Cancel。
//   - builtin 规则:抽屉顶部「复制为自定义」按钮 → Modal 填 new_id → forkRule → onForked。
// 抽屉由调用方(Settings Task 17 / RulesTable 行点击 view)控制 open/close,RuleDrawer 只管自身编辑态。
export function RuleDrawer({ rule, mode, domain, onClose, onSaved, onForked }: RuleDrawerProps) {
  const { t } = useTranslation()
  const saveRule = useStore((s) => s.saveRule)
  const forkRule = useStore((s) => s.forkRule)
  const validateRuleDraft = useStore((s) => s.validateRuleDraft)
  const loadingRuleId = useStore((s) => s.loadingRuleId)

  const [draft, setDraft] = useState<RuleDraft | null>(null)
  const [validation, setValidation] = useState<{ valid: boolean; errors: string[] } | null>(null)
  const [saving, setSaving] = useState(false)
  const [forkOpen, setForkOpen] = useState(false)
  const [newId, setNewId] = useState('')
  const [forking, setForking] = useState(false)

  // 进入 edit/create 时(或 rule 切换)初始化 draft + 清旧校验。
  useEffect(() => {
    if (mode === 'view') return
    const base: RuleDTO = rule ?? {
      id: 'custom.my-rule',
      severity: 'medium',
      asset_type: '',
      match: {},
      source: 'custom',
      enabled: true,
      can_edit: true,
      domain,
    }
    setDraft(ruleToDraft(base))
    setValidation(null)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, rule?.id, domain])

  // 防抖实时校验:draft 变化后 500ms → 序列化 RuleDTO map → POST /validate。
  const draftKey = draft ? JSON.stringify(draft) : ''
  useDebouncedEffect(draftKey, 500, () => {
    if (mode === 'view' || !draft) return
    const dto = draftToRuleDTO(draft, mode)
    let stale = false
    validateRuleDraft(domain, dto).then((res) => {
      if (!stale && res) setValidation(res)
    })
    return () => { stale = true }
  })

  const onSave = useCallback(async () => {
    if (!draft) return
    if (mode === 'view') return
    const dto = draftToRuleDTO(draft, mode)
    setSaving(true)
    const payload = mode === 'edit'
      ? { ...dto, id: rule?.id ?? draft.id, source: 'custom' as const }
      : { ...dto, id: draft.id }
    useStore.getState().clearError()
    await saveRule(domain, payload as Parameters<typeof saveRule>[1])
    setSaving(false)
    if (!useStore.getState().error) {
      onSaved?.()
    }
  }, [draft, mode, rule?.id, domain, saveRule, onSaved])

  // fork builtin → custom:Modal 填 new_id → forkRule(domain, rule.id, newId) → onForked(新 RuleDTO)。
  // 调用方(Settings Task 17)收到 onForked 后可切到新 custom 的 edit 态。
  const onForkOk = useCallback(async () => {
    if (!rule) return
    if (!newId.trim()) {
      message.error(t('ruleDrawer.forkIdRequired'))
      return
    }
    setForking(true)
    const created = await forkRule(domain, rule.id, newId.trim())
    setForking(false)
    if (created) {
      setForkOpen(false)
      setNewId('')
      message.success(t('ruleDrawer.forkSuccess'))
      onForked?.(created)
    }
  }, [rule, newId, domain, forkRule, onForked, t])

  const isEditing = mode !== 'view'
  // builtin 规则在 edit 模式:防御纵深(Task 17 调用方应保证 builtin 不进 edit,此处兜底)。
  // can_edit=false → 表单只读 + Save 禁用;保留 warning + Fork 入口(view 正文已无,extra 仍显示)。
  const isBuiltinEdit = mode === 'edit' && rule?.source === 'builtin'
  // builtin 规则在 view 模式提供 fork 入口;custom 规则不提供(后端 fork 仅 builtin→custom)。
  const canFork = (mode === 'view' || isBuiltinEdit) && rule?.source === 'builtin'
  // 抽屉标题:view=规则详情;edit=编辑规则;create=新建规则。
  const title = isEditing
    ? (mode === 'create' ? t('rulesManage.create') : t('rulesManage.edit'))
    : t('ruleDrawer.title')

  return (
    <Drawer
      title={title}
      placement="right"
      width="50%"
      open={rule !== null || mode === 'create'}
      onClose={onClose}
      maskClosable
      keyboard
      rootClassName="rule-drawer"
      styles={{ body: { padding: 16, overflow: 'auto' } }}
      // 编辑模式下顶部放 Save/Cancel + (builtin)Fork 按钮;view 模式仅 builtin 规则放 Fork 按钮。
      extra={
        <Space>
          {canFork ? (
            <Button onClick={() => setForkOpen(true)}>{t('rulesManage.fork')}</Button>
          ) : null}
          {isEditing ? (
            <>
              <Button onClick={onClose}>{t('common.cancel')}</Button>
              <Button
                type="primary"
                loading={saving || loadingRuleId === rule?.id}
                onClick={onSave}
                disabled={isBuiltinEdit}
              >
                {t('common.save')}
              </Button>
            </>
          ) : null}
        </Space>
      }
    >
      {/* view 模式:只读 Descriptions(适配 RuleDTO 字段)。 */}
      {(!isEditing && rule) ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <Descriptions title={t('ruleDrawer.infoTitle')} size="small" column={1} bordered>
            <Descriptions.Item label={t('ruleDrawer.ruleId')}>
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 14 }}>{rule.id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.ruleName')}>{ruleName({ id: rule.id, description: rule.description ?? '' })}</Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.severity')}>
              <SevBadge tone={`sev-${rule.severity}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[rule.severity as Severity])}</SevBadge>
            </Descriptions.Item>
            {/* 来源:RuleDTO.source 只有 builtin/custom(ruleTable 双语 key 复用)。 */}
            <Descriptions.Item label={t('ruleDrawer.source')}>
              {rule.source === 'custom' ? t('ruleTable.sourceCustom') : t('ruleTable.sourceBaseline')}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.assetType')}>
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{rule.asset_type || '--'}</Typography.Text>
            </Descriptions.Item>
            {/* match 摘要:RuleDTO 把规则语法并入 match 对象(如 {regex: ...});展示完整 JSON。 */}
            <Descriptions.Item label={t('ruleDrawer.ruleSyntax')}>
              {rule.match && Object.keys(rule.match).length > 0 ? (
                <MatchTreeEditor
                  value={matchMapToTree(rule.match)}
                  matchMap={rule.match}
                  assetType={rule.asset_type}
                  readOnly
                  onChange={() => {}}
                />
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.remediation')}>
              <span style={{ fontSize: 13 }}>{rule.remediation || '--'}</span>
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.pathFilter')}>
              {rule.paths ? (
                <span style={{ fontSize: 12 }}>
                  {rule.paths.include?.length ? `${t('ruleDrawer.pathInclude', { items: rule.paths.include.join(', ') })} ` : ''}
                  {rule.paths.exclude?.length ? `${t('ruleDrawer.pathExclude', { items: rule.paths.exclude.join(', ') })}` : ''}
                  {!rule.paths.include?.length && !rule.paths.exclude?.length ? t('ruleDrawer.none') : ''}
                </span>
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.postExclude')}>
              {rule.post_exclude?.length ? (
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{rule.post_exclude.join(', ')}</span>
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.deobfuscation')}>
              {rule.deobfuscation?.length ? (
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{rule.deobfuscation.join(', ')}</span>
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.dotall')}>{rule.dotall ? t('common.yes') : t('common.no')}</Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.metadata')}>
              {rule.metadata && Object.keys(rule.metadata).length > 0 ? (
                <pre style={{ margin: 0, fontSize: 11, fontFamily: 'var(--font-mono)' }}>{JSON.stringify(rule.metadata, null, 2)}</pre>
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
          </Descriptions>

          {/* builtin 只读提示 + fork 入口(重复 extra 的 fork 按钮,view 正文底部更显眼)。 */}
          {rule.source === 'builtin' ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <AntBadge status="warning" text={t('rulesManage.builtinReadonly')} />
              <Button onClick={() => setForkOpen(true)}>{t('rulesManage.fork')}</Button>
            </div>
          ) : null}
        </div>
      ) : null}

      {isEditing && draft ? (
        <Form layout="vertical" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {rule && rule.source === 'builtin' ? (
            <Alert type="warning" message={t('rulesManage.builtinReadonly')} showIcon style={{ marginBottom: 4 }} />
          ) : null}

          {/* 基础区 */}
          <Typography.Title level={5} style={{ marginTop: 0 }}>{t('ruleForm.basicSection')}</Typography.Title>
          <Form.Item label={t('ruleDrawer.ruleId')}>
            <Input
              value={draft.id}
              disabled={isBuiltinEdit || mode === 'edit'}
              onChange={(e) => setDraft({ ...draft, id: e.target.value })}
            />
          </Form.Item>
          <Form.Item label={t('ruleDrawer.severity')}>
            <Select
              value={draft.severity || undefined}
              disabled={isBuiltinEdit}
              onChange={(v: string) => setDraft({ ...draft, severity: v })}
            >
              {SEVERITY_ORDER.map((s) => (
                <Select.Option key={s} value={s}>
                  <Space><SevBadge tone={`sev-${s}` as BadgeTone}>{t(SEVERITY_LABEL_KEY[s as Severity])}</SevBadge></Space>
                </Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item label={t('ruleDrawer.assetType')}>
            <Select
              value={draft.asset_type || undefined}
              disabled={isBuiltinEdit}
              allowClear
              placeholder={t('ruleForm.anyAssetType')}
              onChange={(v: string) => setDraft({ ...draft, asset_type: v ?? '' })}
            >
              {['settings','permissions','hook','mcp_server','skill','command','agent','plugin','memory','keybinding','script','credential'].map((a) => (
                <Select.Option key={a} value={a}>{a}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item label={t('ruleDrawer.dotall')} style={{ marginBottom: 0 }}>
            <Switch
              checked={draft.dotall}
              disabled={isBuiltinEdit}
              onChange={(v) => setDraft({ ...draft, dotall: v })}
            />
          </Form.Item>

          {/* match 树 */}
          <Typography.Title level={5}>{t('ruleForm.matchTreeTitle')}</Typography.Title>
          <MatchTreeEditor
            value={draft.matchTree}
            matchMap={draft.matchMap}
            assetType={draft.asset_type}
            readOnly={isBuiltinEdit}
            onChange={(next) => setDraft({ ...draft, matchTree: next })}
          />
          {validation && !validation.valid ? (
            <Alert type="error" message={validation.errors.join('; ')} showIcon />
          ) : null}
          {validation && validation.valid ? (
            <Alert type="success" message={t('ruleDrawer.validationOk')} showIcon />
          ) : null}

          {/* 高级区(折叠) */}
          <Collapse
            ghost
            items={[{
              key: 'advanced',
              label: t('ruleForm.advancedSection'),
              children: (
                <>
                  <Form.Item label={t('ruleDrawer.ruleName')}>
                    <Input value={draft.description} disabled={isBuiltinEdit}
                      onChange={(e) => setDraft({ ...draft, description: e.target.value })} />
                  </Form.Item>
                  <Form.Item label={t('ruleDrawer.remediation')}>
                    <Input.TextArea rows={2} value={draft.remediation} disabled={isBuiltinEdit}
                      onChange={(e) => setDraft({ ...draft, remediation: e.target.value })} />
                  </Form.Item>
                  <Form.Item label={t('ruleDrawer.deobfuscation')}>
                    <Select mode="multiple" value={draft.deobfuscation} disabled={isBuiltinEdit} placeholder="--"
                      onChange={(v: string[]) => setDraft({ ...draft, deobfuscation: v })}
                      options={['zero_width','html_comment','base64','leetspeak','wrapper_strip','ansi_c_decode'].map((d) => ({ value: d, label: d }))} />
                  </Form.Item>
                  <Form.Item label={t('ruleDrawer.postExclude')}>
                    <Select mode="tags" value={draft.post_exclude} disabled={isBuiltinEdit}
                      onChange={(v: string[]) => setDraft({ ...draft, post_exclude: v })} />
                  </Form.Item>
                  <Form.Item label={t('ruleDrawer.pathFilter')}>
                    <Select mode="tags" value={draft.pathsInclude} disabled={isBuiltinEdit} placeholder="include"
                      onChange={(v: string[]) => setDraft({ ...draft, pathsInclude: v })} />
                    <Select mode="tags" value={draft.pathsExclude} disabled={isBuiltinEdit} placeholder="exclude" style={{ marginTop: 4 }}
                      onChange={(v: string[]) => setDraft({ ...draft, pathsExclude: v })} />
                  </Form.Item>
                  <Form.Item label={t('ruleDrawer.metadata')} style={{ marginBottom: 0 }}>
                    {draft.metadata.map((m, i) => (
                      <Space key={i} style={{ display: 'flex', marginBottom: 4 }} align="baseline">
                        <Input value={m.key} disabled={isBuiltinEdit} placeholder="key" style={{ width: 140 }}
                          onChange={(e) => setDraft({ ...draft, metadata: draft.metadata.map((x, j) => j === i ? { ...x, key: e.target.value } : x) })} />
                        <Input value={m.value} disabled={isBuiltinEdit} placeholder="value" style={{ width: 200 }}
                          onChange={(e) => setDraft({ ...draft, metadata: draft.metadata.map((x, j) => j === i ? { ...x, value: e.target.value } : x) })} />
                        {!isBuiltinEdit && <Button type="text" danger icon={<DeleteOutlined />}
                          onClick={() => setDraft({ ...draft, metadata: draft.metadata.filter((_, j) => j !== i) })} />}
                      </Space>
                    ))}
                    {!isBuiltinEdit && <Button type="dashed" icon={<PlusOutlined />} size="small"
                      onClick={() => setDraft({ ...draft, metadata: [...draft.metadata, { key: '', value: '' }] })}>+</Button>}
                  </Form.Item>
                </>
              ),
            }]}
          />
        </Form>
      ) : null}

      {/* fork Modal:填 new_id(custom.xxx)→ forkRule → onForked。 */}
      <Modal
        open={forkOpen}
        title={t('rulesManage.forkTitle')}
        onCancel={() => { setForkOpen(false); setNewId('') }}
        onOk={onForkOk}
        okButtonProps={{ loading: forking }}
        destroyOnClose
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
          {t('ruleDrawer.forkHint', { id: rule?.id ?? '' })}
        </Typography.Paragraph>
        <Input
          value={newId}
          onChange={(e) => setNewId(e.target.value)}
          placeholder="custom.my-rule"
          onPressEnter={onForkOk}
        />
      </Modal>
    </Drawer>
  )
}
