import { useEffect, useState } from 'react'
import { Tabs, Card, Switch, Input, Button, Table, Tag, Typography, InputNumber, Radio, Space, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DownOutlined, RightOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useStore } from '../../store'
import type { DetectorMeta, DetectorsConfig, GuardConfig } from '../../types'
import { SettingsSchedules } from '../SettingsSchedules'

const { Text } = Typography

function statusTag(available: boolean, t: (k: string) => string) {
  return available
    ? <Tag color="green" style={{ marginInlineEnd: 0, fontSize: 'var(--fs-xs)' }}>{t('detector.statusAvailable')}</Tag>
    : <Tag color="red" style={{ marginInlineEnd: 0, fontSize: 'var(--fs-xs)' }}>{t('detector.statusUnavailable')}</Tag>
}

function detectorLabel(id: string, t: (k: string) => string): string {
  const map: Record<string, string> = {
    rules: t('detectors.rules'),
    secret: t('detectors.secret'),
    dep: t('detectors.dep'),
  }
  return map[id] ?? id
}

function detectorDeps(id: string, meta?: DetectorMeta): string {
  if (id === 'rules') return '内置'
  if (id === 'secret') return 'gitleaks'
  if (id === 'dep') return (meta?.engines ?? []).map((e) => e.name).join(' / ')
  return '—'
}

export default function SecurityConfig() {
  const { t } = useTranslation()
  const store = useStore()
  const { detectors, fetchDetectors, detectorConfig, fetchDetectorConfig, saveDetectorConfig } = store
  const { guardConfig, fetchGuardConfig, saveGuardConfig } = store
  const [activeTab, setActiveTab] = useState<string>('scan')

  // 扫描配置状态
  const [draft, setDraft] = useState<DetectorsConfig | null>(null)
  const [savingScan, setSavingScan] = useState(false)
  const [expandedRows, setExpandedRows] = useState<string[]>([])

  // 拦截配置状态
  const [form, setForm] = useState<GuardConfig | null>(null)
  const [savingGuard, setSavingGuard] = useState(false)

  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  useEffect(() => { fetchDetectorConfig() }, [fetchDetectorConfig])
  useEffect(() => { if (detectorConfig) setDraft(detectorConfig) }, [detectorConfig])

  useEffect(() => { fetchGuardConfig() }, [fetchGuardConfig])
  useEffect(() => { setForm(guardConfig) }, [guardConfig])

  // ---- 扫描引擎 ----
  const patchScan = (p: Partial<DetectorsConfig>) => setDraft(draft ? { ...draft, ...p } : draft)

  const onSaveScan = async () => {
    if (!draft) return
    setSavingScan(true)
    await saveDetectorConfig(draft)
    setSavingScan(false)
  }

  const expandedRender = (d: DetectorMeta) => {
    if (d.id === 'rules') {
      return (
        <div style={{ padding: '8px 0' }}>
          <Text type="secondary" style={{ fontSize: 'var(--fs-sm)' }}>{t('detector.rulesEngineDesc', { defaultValue: '内置规则引擎，无需外部依赖' })}</Text>
        </div>
      )
    }
    if (d.id === 'secret') {
      return (
        <div style={{ padding: '8px 0', display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ fontSize: 'var(--fs-sm)', whiteSpace: 'nowrap' }}>{t('settings.binaryPath')}</span>
          <Input
            size="small"
            style={{ flex: 1 }}
            placeholder={t('settings.defaultBinary', { name: 'gitleaks' })}
            value={draft?.secret.binary ?? ''}
            onChange={(e) => patchScan({ secret: { ...draft!.secret, binary: e.target.value } })}
          />
        </div>
      )
    }
    if (d.id === 'dep') {
      return (
        <div style={{ padding: '8px 0', display: 'flex', flexDirection: 'column', gap: 8 }}>
          {(d.engines ?? []).map((eng) => {
            const cfg = draft?.dep.engines[eng.name] ?? { enabled: true, binary: '' }
            return (
              <div key={eng.name} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <Switch
                  size="small"
                  checked={cfg.enabled}
                  onChange={(v) => patchScan({
                    dep: {
                      ...draft!.dep,
                      engines: { ...draft!.dep.engines, [eng.name]: { ...cfg, enabled: v } },
                    },
                  })}
                />
                <span style={{ fontSize: 'var(--fs-sm)', width: 100 }}>{eng.name}</span>
                <Input
                  size="small"
                  style={{ flex: 1 }}
                  placeholder={t('settings.defaultBinary', { name: eng.name })}
                  value={cfg.binary}
                  onChange={(ev) => patchScan({
                    dep: {
                      ...draft!.dep,
                      engines: { ...draft!.dep.engines, [eng.name]: { ...cfg, binary: ev.target.value } },
                    },
                  })}
                />
              </div>
            )
          })}
        </div>
      )
    }
    return null
  }

  const scanColumns: ColumnsType<DetectorMeta> = [
    { title: t('detector.colName', { defaultValue: '扫描引擎' }), dataIndex: 'id', width: 160,
      render: (id: string) => <Text strong style={{ fontSize: 'var(--fs-sm)' }}>{detectorLabel(id, t)}</Text> },
    { title: t('detector.colDeps', { defaultValue: '引擎依赖' }), width: 240,
      render: (_: unknown, r: DetectorMeta) => <Text style={{ fontSize: 'var(--fs-sm)' }}>{detectorDeps(r.id, r)}</Text> },
    { title: t('detector.colStatus', { defaultValue: '可用状态' }), width: 100,
      render: (_: unknown, r: DetectorMeta) => statusTag(r.available, t) },
    { title: t('detector.colEnable', { defaultValue: '启用' }), width: 80,
      render: (_: unknown, r: DetectorMeta) => {
        if (!draft) return null
        const enabled = r.id === 'rules' ? draft.rules.enabled
          : r.id === 'secret' ? draft.secret.enabled
          : draft.dep.enabled
        return (
          <Switch
            size="small"
            checked={enabled}
            onChange={(v) => {
              if (r.id === 'rules') patchScan({ rules: { ...draft.rules, enabled: v } })
              else if (r.id === 'secret') patchScan({ secret: { ...draft.secret, enabled: v } })
              else patchScan({ dep: { ...draft.dep, enabled: v } })
            }}
          />
        )
      },
    },
  ]

  const engineList = detectors.filter((d) => ['rules', 'secret', 'dep'].includes(d.id))

  // ---- 拦截配置 ----
  const patchGuard = (p: Partial<GuardConfig>) => setForm((prev) => prev ? { ...prev, ...p } : prev)

  const onSaveGuard = async () => {
    if (!form) return
    setSavingGuard(true)
    await saveGuardConfig(form)
    setSavingGuard(false)
  }

  // ---- Tab 内容 ----
  const scanTab = draft ? (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Card title={t('detector.title', { defaultValue: '扫描引擎' })} size="small" extra={
        <Button type="primary" size="small" loading={savingScan} onClick={onSaveScan}>{t('settings.saveConfig')}</Button>
      }>
        <Table
          size="small"
          rowKey="id"
          dataSource={engineList}
          columns={scanColumns}
          pagination={false}
          expandable={{
            expandedRowKeys: expandedRows,
            onExpandedRowsChange: (keys) => setExpandedRows(keys as string[]),
            expandedRowRender: expandedRender,
            expandIconColumnIndex: 4,
            expandIcon: ({ expanded, onExpand, record }) => {
              if (record.id === 'rules') return null
              return (
                <Button
                  type="text"
                  size="small"
                  icon={expanded ? <DownOutlined /> : <RightOutlined />}
                  onClick={(e) => onExpand(record, e)}
                  aria-label={expanded ? '收起' : '展开'}
                />
              )
            },
          }}
        />
      </Card>
      <SettingsSchedules />
    </div>
  ) : null

  const interceptTab = guardConfig && form ? (
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

      {/* 高级 — 参考通用设置个性化配置: 标题左控件右 */}
      <Card title={t('settings.advanced')} size="small">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <span style={{ fontSize: 'var(--fs-sm)' }}>{t('guard.mode')}</span>
            <Radio.Group value={form.mode} onChange={(e) => patchGuard({ mode: e.target.value })}>
              <Radio value="strict">{t('guard.modeStrict')}</Radio>
              <Radio value="lenient">{t('guard.modeLenient')}</Radio>
            </Radio.Group>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <span style={{ fontSize: 'var(--fs-sm)' }}>{t('guard.allowlistEnabled')}</span>
            <Switch checked={form.allowlist_enabled} onChange={(v) => patchGuard({ allowlist_enabled: v })} />
            <Typography.Text type="secondary" style={{ fontSize: 'var(--fs-sm)' }}>{t('guard.allowlistEnabledHint')}</Typography.Text>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <span style={{ fontSize: 'var(--fs-sm)' }}>{t('guard.deadlineMs')}</span>
            <InputNumber min={0} value={form.deadline_ms} onChange={(v) => patchGuard({ deadline_ms: typeof v === 'number' ? v : 0 })} style={{ width: 200 }} />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <span style={{ fontSize: 'var(--fs-sm)' }}>{t('guard.maxCommandBytes')}</span>
            <InputNumber min={0} value={form.max_command_bytes} onChange={(v) => patchGuard({ max_command_bytes: typeof v === 'number' ? v : 0 })} style={{ width: 200 }} />
          </div>
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 12 }}>
          <Button type="primary" loading={savingGuard} onClick={onSaveGuard}>{t('settings.saveConfig')}</Button>
        </div>
      </Card>
    </div>
  ) : null

  const tabItems = [
    { key: 'scan', label: t('nav.sub.scanConfig', { defaultValue: '扫描配置' }), children: scanTab },
    { key: 'intercept', label: t('nav.sub.interceptConfig', { defaultValue: '拦截配置' }), children: interceptTab },
  ]

  return (
    <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />
  )
}
