import { useEffect, useState } from 'react'
import { Card, Form, Radio, Switch, Button, Space, InputNumber, Typography, message } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { GuardConfig } from '../types'

// Settings 页「拦截配置」tab(Stage R3 Task 12)。
//
// 后端 PUT /api/guard/config 顶层键校验拒绝部分体(见 handlers_guard.go):必须发送全部 6 键
// (enabled/policy/deadline_ms/max_command_bytes/mode/allowlist_enabled),否则 400。
// 因此本组件 fetch 全量配置 → 原地编辑 form(展开所有字段)→ 保存时整体 PUT form 对象。
// form 始终持有完整 6 字段(fetchGuardConfig 返回完整快照);即便 UI 只暴露部分字段,
// 未暴露字段也随 form 原样回传,不会丢键。
//
// mode: strict(默认,不确定时 deny)/ lenient(不确定时 ask)。高置信度命中两模式都 deny,
// Mode 只影响"不确定"降级(安全不变量 #1,见 config/guard.go)。
// allowlist_enabled: 控制管线 ⑦ 是否做精确匹配放行;false 跳过放行(放行清单 tab 仍可编辑)。
export function SettingsGuard() {
  const { t } = useTranslation()
  const { guardConfig, fetchGuardConfig, saveGuardConfig } = useStore()
  const [form, setForm] = useState<GuardConfig | null>(null)
  const [saving, setSaving] = useState(false)

  // 挂载即拉全量配置(与 SettingsSchedules 一致:无条件刷新,不依赖守卫模式)。
  useEffect(() => { fetchGuardConfig() }, [fetchGuardConfig])
  // store.guardConfig → 本地 form(只在 store 变化时同步,避免覆盖用户正在编辑的值)。
  useEffect(() => { setForm(guardConfig) }, [guardConfig])

  const patch = (p: Partial<GuardConfig>) => setForm((prev) => prev ? { ...prev, ...p } : prev)

  const onSave = async () => {
    if (!form) return
    setSaving(true)
    // PUT 全量 6 键:form 从 fetchGuardConfig 拿到完整快照,即便 UI 未暴露 policy/max_command_bytes
    // 也随 form 原样回传,满足后端顶层键校验(缺键 400)。
    const ok = await saveGuardConfig(form)
    setSaving(false)
    if (ok) message.success(t('guard.saved'))
  }

  if (!form) return null
  return (
    <Card title={t('guard.title')}>
      <Form layout="vertical">
        <Form.Item label={t('guard.enabled')}>
          <Space align="center">
            <Switch checked={form.enabled} onChange={(v) => patch({ enabled: v })} />
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('guard.enabledHint')}</Typography.Text>
          </Space>
        </Form.Item>
        <Form.Item label={t('guard.mode')}>
          <Radio.Group value={form.mode} onChange={(e) => patch({ mode: e.target.value })}>
            <Radio value="strict">{t('guard.modeStrict')}</Radio>
            <Radio value="lenient">{t('guard.modeLenient')}</Radio>
          </Radio.Group>
        </Form.Item>
        <Form.Item label={t('guard.allowlistEnabled')}>
          <Space align="center">
            <Switch checked={form.allowlist_enabled} onChange={(v) => patch({ allowlist_enabled: v })} />
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('guard.allowlistEnabledHint')}</Typography.Text>
          </Space>
        </Form.Item>
        <Form.Item label={t('guard.deadlineMs')}>
          <InputNumber
            min={0}
            value={form.deadline_ms}
            onChange={(v) => patch({ deadline_ms: typeof v === 'number' ? v : 0 })}
            style={{ width: 200 }}
          />
        </Form.Item>
        <Form.Item label={t('guard.maxCommandBytes')}>
          <InputNumber
            min={0}
            value={form.max_command_bytes}
            onChange={(v) => patch({ max_command_bytes: typeof v === 'number' ? v : 0 })}
            style={{ width: 200 }}
          />
        </Form.Item>
        <Space>
          <Button type="primary" loading={saving} onClick={onSave}>{t('common.save')}</Button>
        </Space>
      </Form>
    </Card>
  )
}
