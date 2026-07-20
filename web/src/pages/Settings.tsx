import { useEffect, useState } from 'react'
import { Card, Empty, Tabs, Switch, Input, Button } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { DetectorMeta, DetectorsConfig } from '../types'
import { RulesTable } from '../components/RulesTable'
import { DetectorPanel } from '../components/DetectorPanel'
import { SettingsAgents } from './SettingsAgents'

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
  const totalRules = detectors.reduce((n, d) => n + (d.rules ?? []).length, 0)

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
      {totalRules === 0 ? <Empty description={t('settings.noRules')} /> : <RulesTable detectors={detectors} detectorFilter={filter} />}
    </div>
  )

  const items = [
    { key: 'agents', label: t('settings.agentsTab'), children: <SettingsAgents /> },
    { key: 'detectors-rules', label: t('settings.rulesConfig'), children: detectorsAndRules },
  ]

  return (
    <div>
      <Tabs items={items} />
    </div>
  )
}
