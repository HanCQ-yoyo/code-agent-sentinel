import { useEffect, useState } from 'react'
import { Card, Switch, InputNumber, Radio, Space, Typography, Popconfirm, Button, Form, Alert } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../../store'
import type { GuardConfig } from '../../types'

export default function InterceptConfig() {
  const { t } = useTranslation()
  const { guardConfig, fetchGuardConfig, saveGuardConfig } = useStore()
  const [form, setForm] = useState<GuardConfig | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => { fetchGuardConfig() }, [fetchGuardConfig])
  useEffect(() => { setForm(guardConfig) }, [guardConfig])

  const patch = (p: Partial<GuardConfig>) => setForm((prev) => prev ? { ...prev, ...p } : prev)

  const onSave = async () => {
    if (!form) return
    setSaving(true)
    await saveGuardConfig(form)
    setSaving(false)
  }

  if (!guardConfig || !form) return null

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 总开关 */}
      <Card title={t('guard.enabled')} size="small">
        <Space align="center">
          <Popconfirm
            title={guardConfig.enabled ? t('guard.confirmDisable') : t('guard.confirmEnable')}
            okText={t('common.save')}
            cancelText={t('common.cancel')}
            onConfirm={async () => {
              await saveGuardConfig({ ...guardConfig, enabled: !guardConfig.enabled })
            }}
          >
            <Switch checked={guardConfig.enabled} />
          </Popconfirm>
          <Typography.Text type="secondary" style={{ fontSize: 'var(--fs-sm)' }}>{t('guard.enabledHint')}</Typography.Text>
        </Space>
      </Card>

      {/* 平铺的 4 字段(原高级 Modal 内容) */}
      <Card title={t('settings.advanced')}>
        <Form layout="vertical">
          <Form.Item label={t('guard.mode')}>
            <Radio.Group value={form.mode} onChange={(e) => patch({ mode: e.target.value })}>
              <Radio value="strict">{t('guard.modeStrict')}</Radio>
              <Radio value="lenient">{t('guard.modeLenient')}</Radio>
            </Radio.Group>
          </Form.Item>
          <Form.Item label={t('guard.allowlistEnabled')}>
            <Space align="center">
              <Switch checked={form.allowlist_enabled} onChange={(v) => patch({ allowlist_enabled: v })} />
              <Typography.Text type="secondary" style={{ fontSize: 'var(--fs-sm)' }}>{t('guard.allowlistEnabledHint')}</Typography.Text>
            </Space>
          </Form.Item>
          <Form.Item label={t('guard.deadlineMs')}>
            <InputNumber min={0} value={form.deadline_ms} onChange={(v) => patch({ deadline_ms: typeof v === 'number' ? v : 0 })} style={{ width: 200 }} />
          </Form.Item>
          <Form.Item label={t('guard.maxCommandBytes')}>
            <InputNumber min={0} value={form.max_command_bytes} onChange={(v) => patch({ max_command_bytes: typeof v === 'number' ? v : 0 })} style={{ width: 200 }} />
          </Form.Item>
        </Form>
        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <Button type="primary" loading={saving} onClick={onSave}>{t('settings.saveConfig')}</Button>
        </div>
      </Card>

      {/* per-agent 占位 */}
      <Alert
        type="info"
        showIcon
        message={t('nav.sub.interceptConfig')}
        description={t('settings.interceptPerAgentPlaceholder')}
      />
    </div>
  )
}
