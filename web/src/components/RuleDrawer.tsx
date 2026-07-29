import { useState, useEffect, useRef, useCallback, lazy, Suspense } from 'react'
import { Drawer, Descriptions, Typography, Badge as AntBadge, Button, Space, Modal, Input, Alert, Spin, message } from 'antd'
import { useTranslation } from 'react-i18next'
// js-yaml:用命名导入(v5 ESM 仅有命名导出,无 default;@types/js-yaml 同步为命名导出)。
// yamlDump/yamlLoad 对应 js-yaml 的 dump/load(brief 原稿命名)。
import { dump as yamlDump, load as yamlLoad } from 'js-yaml'
import type { RuleDTO, RuleDomain, Severity } from '../types'
import { useStore } from '../store'
import { useTheme } from '../theme'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { SEVERITY_LABEL_KEY } from '../lib/severity'
import { ruleName } from '../lib/i18n-names'
// MonacoViewer 懒加载:与 ContentArea/MonacoBlock/RawFilePanel 一致,保持 monaco chunk 独立
// (静态导入会触发 vite 警告 + 把 monaco 并入主 chunk,增大首屏)。
const MonacoViewer = lazy(() => import('./MonacoViewer'))

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

// RuleDTO → YAML 编辑器文本对象:去掉派生字段(source/enabled/can_edit/domain),
// 这些由后端管理,不应进 YAML 草稿;id 在 create 模式下用户可改,edit 模式下保留。
// 序列化字段顺序按 RuleDTO 声明顺序(js-yaml dump 按 Object insertion order)。
function ruleToYamlObj(rule: RuleDTO, mode: 'edit' | 'create'): Record<string, unknown> {
  const obj: Record<string, unknown> = {
    id: rule.id,
    severity: rule.severity,
    asset_type: rule.asset_type,
    match: rule.match,
  }
  if (rule.deobfuscation) obj.deobfuscation = rule.deobfuscation
  if (rule.dotall) obj.dotall = rule.dotall
  if (rule.paths) obj.paths = rule.paths
  if (rule.post_exclude) obj.post_exclude = rule.post_exclude
  if (rule.remediation) obj.remediation = rule.remediation
  if (rule.description) obj.description = rule.description
  if (rule.metadata) obj.metadata = rule.metadata
  // create 模式默认 enabled=true,便于用户直接编辑(后端 POST 接收 enabled)。
  if (mode === 'create') obj.enabled = true
  else obj.enabled = rule.enabled
  return obj
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
    const handle = setTimeout(() => {
      const cleanup = effectRef.current()
      if (typeof cleanup === 'function') cleanup()
    }, delayMs)
    return () => clearTimeout(handle)
    // value 变化触发防抖;delayMs 稳定常量。effect 经 ref 取最新,不入依赖(否则每次重渲染都重置定时器)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, delayMs])
}

// 安全解析 YAML 字符串 → 对象;解析失败返回 null(供校验/保存分支识别)。
// 包裹 js-yaml load:抛异常 / 非对象 / 数组 → null。
function parseYaml(text: string): Record<string, unknown> | null {
  try {
    const parsed = yamlLoad(text)
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return null
    return parsed as Record<string, unknown>
  } catch {
    return null
  }
}

// 规则详情抽屉:支持 view(只读)/edit(编辑 custom)/create(新建 custom)三态 + builtin fork。
// Task 16:从旧 FlatRule(含 detector/syntax/valid/source_file/project_path)迁移到 RuleDTO。
//   - view 模式:Descriptions 展示 RuleDTO 字段 + match 摘要(规则语法已并入 match,无独立 syntax)。
//   - edit/create 模式:Monaco YAML 编辑器 + 防抖实时校验(POST /validate)+ Save/Cancel。
//   - builtin 规则:抽屉顶部「复制为自定义」按钮 → Modal 填 new_id → forkRule → onForked。
// 抽屉由调用方(Settings Task 17 / RulesTable 行点击 view)控制 open/close,RuleDrawer 只管自身编辑态。
export function RuleDrawer({ rule, mode, domain, onClose, onSaved, onForked }: RuleDrawerProps) {
  const { t } = useTranslation()
  const { theme } = useTheme()
  const saveRule = useStore((s) => s.saveRule)
  const forkRule = useStore((s) => s.forkRule)
  const validateRuleDraft = useStore((s) => s.validateRuleDraft)
  const loadingRuleId = useStore((s) => s.loadingRuleId)

  // YAML 草稿文本(编辑器内容);实时校验结果;保存中标记。
  const [draftYaml, setDraftYaml] = useState('')
  const [validation, setValidation] = useState<{ valid: boolean; errors: string[] } | null>(null)
  const [saving, setSaving] = useState(false)
  // fork Modal 状态:newId 输入 + Modal 开关。
  const [forkOpen, setForkOpen] = useState(false)
  const [newId, setNewId] = useState('')
  const [forking, setForking] = useState(false)

  // 进入 edit/create 时(或 rule 切换时),把 rule 序列化成 YAML 填入 draft + 清旧校验。
  // create 模式 rule 可能为 null → 用空模板(id 占位 custom.<...>)。
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
    setDraftYaml(yamlDump(ruleToYamlObj(base, mode)))
    setValidation(null)
    // 依赖 mode/rule.id/domain:rule 切换或模式切换时重置草稿。rule.id 而非 rule 整体,
    // 避免 rule 对象引用变化(列表重拉)导致正在编辑的草稿被覆盖。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, rule?.id, domain])

  // 防抖实时校验:草稿变化后 500ms 无新输入 → 解析 + POST /validate。
  // YAML 解析失败 → 直接展示语法错误(不调后端)。解析成功但缺 id 等也交后端 validate 报错。
  useDebouncedEffect(draftYaml, 500, () => {
    if (mode === 'view') return
    const parsed = parseYaml(draftYaml)
    if (parsed === null) {
      setValidation({ valid: false, errors: [t('ruleDrawer.yamlParseError')] })
      return
    }
    let stale = false
    validateRuleDraft(domain, parsed as Partial<RuleDTO>).then((res) => {
      if (!stale && res) setValidation(res)
    })
    return () => { stale = true }
  })

  // 保存:解析草稿 → saveRule(source 判定 create/update)。后端对 builtin PUT 返回 409,
  // 但 edit 模式仅对 custom 规则开放(builtin 抽屉无编辑入口),故此处 source 必为 'custom'。
  // create 模式不带 source(草稿未落库)→ saveRule 走 POST。
  const onSave = useCallback(async () => {
    const parsed = parseYaml(draftYaml)
    if (parsed === null) {
      message.error(t('ruleDrawer.yamlParseError'))
      return
    }
    setSaving(true)
    // edit:带 source='custom' → PUT /:id;create:不带 source → POST。
    // id 优先用草稿里的(用户可能改了 id,仅 create 有意义;edit 的 id 后端按 path 取)。
    const payload = mode === 'edit'
      ? { ...(parsed as object), id: rule?.id ?? parsed.id, source: 'custom' as const }
      : { ...(parsed as object), id: parsed.id ?? '' }
    await saveRule(domain, payload as Parameters<typeof saveRule>[1])
    setSaving(false)
    // saveRule 失败时 wrap 写入 store.error,loadingRuleId 已复位;此处不判成功失败,
    // 与项目其他保存 action 一致(错误由全局 error 展示)。成功后关抽屉。
    onSaved?.()
  }, [draftYaml, mode, rule?.id, domain, saveRule, onSaved, t])

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
  // builtin 规则在 view 模式提供 fork 入口;custom 规则不提供(后端 fork 仅 builtin→custom)。
  const canFork = mode === 'view' && rule?.source === 'builtin'
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
              <Button type="primary" loading={saving || loadingRuleId === rule?.id} onClick={onSave}>
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
              <pre style={{ margin: 0, fontSize: 12, fontFamily: 'var(--font-mono)', wordBreak: 'break-all', color: 'var(--text)' }}>
                {rule.match && Object.keys(rule.match).length > 0 ? JSON.stringify(rule.match, null, 2) : t('ruleDrawer.none')}
              </pre>
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

      {/* edit/create 模式:Monaco YAML 编辑器 + 实时校验 Alert。 */}
      {isEditing ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {/* create 模式无 rule,提示用户编辑草稿;builtin 在 edit 模式不应出现(调用方保证)。 */}
          {rule && rule.source === 'builtin' ? (
            <Alert type="warning" message={t('rulesManage.builtinReadonly')} showIcon style={{ marginBottom: 4 }} />
          ) : null}
          <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
            <MonacoViewer
              value={draftYaml}
              readOnly={false}
              language="yaml"
              theme={theme}
              height="400px"
              onChange={(v) => setDraftYaml(v ?? '')}
            />
          </Suspense>
          {validation && !validation.valid ? (
            <Alert type="error" message={validation.errors.join('; ')} showIcon />
          ) : null}
          {validation && validation.valid ? (
            <Alert type="success" message={t('ruleDrawer.validationOk')} showIcon />
          ) : null}
        </div>
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
