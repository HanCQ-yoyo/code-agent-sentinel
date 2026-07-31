import { useEffect, useState } from 'react'
import { Card, Switch, Input, Button, Space, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../../store'
import type { DetectorMeta, DetectorsConfig } from '../../types'
import { DetectorPanel } from '../../components/DetectorPanel'
import { SettingsSchedules } from '../SettingsSchedules'

const { Text } = Typography

// 检测器配置控件:从 Settings.tsx:15-65 迁出(同逻辑)
function DetectorConfigControls({ d, draft, setDraft, saving, onSave }: {
  d: DetectorMeta; draft: DetectorsConfig | null; setDraft: (c: DetectorsConfig) => void; saving: boolean; onSave: () => void
}) {
  const { t } = useTranslation()
  if (!draft) return null
  const patch = (p: Partial<DetectorsConfig>) => setDraft({ ...draft, ...p })
  return (
    <Card size="small" style={{ background: 'var(--color-surface)', marginTop: 8 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {d.id === 'rules' ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontSize: 'var(--fs-sm)' }}>{t('common.enabled')}</span>
            <Switch size="small" checked={draft.rules.enabled} onChange={(v) => patch({ rules: { ...draft.rules, enabled: v } })} />
          </div>
        ) : null}
        {d.id === 'secret' ? (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 'var(--fs-sm)' }}>{t('common.enabled')}</span>
              <Switch size="small" checked={draft.secret.enabled} onChange={(v) => patch({ secret: { ...draft.secret, enabled: v } })} />
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 'var(--fs-sm)', width: 80 }}>{t('settings.binaryPath')}</span>
              <Input size="small" style={{ flex: 1 }} placeholder={t('settings.defaultBinary', { name: 'gitleaks' })} value={draft.secret.binary} onChange={(e) => patch({ secret: { ...draft.secret, binary: e.target.value } })} />
            </div>
          </>
        ) : null}
        {d.id === 'dep' ? (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 'var(--fs-sm)' }}>{t('common.enabled')}</span>
              <Switch size="small" checked={draft.dep.enabled} onChange={(v) => patch({ dep: { ...draft.dep, enabled: v } })} />
            </div>
            {['npm', 'govulncheck'].map((name) => {
              const e = draft.dep.engines[name] ?? { enabled: true, binary: '' }
              return (
                <div key={name} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Switch size="small" checked={e.enabled} onChange={(v) => patch({ dep: { ...draft.dep, engines: { ...draft.dep.engines, [name]: { ...e, enabled: v } } } })} />
                  <span style={{ fontSize: 'var(--fs-sm)', width: 100 }}>{name}</span>
                  <Input size="small" style={{ flex: 1 }} placeholder={t('settings.defaultBinary', { name })} value={e.binary} onChange={(ev) => patch({ dep: { ...draft.dep, engines: { ...draft.dep.engines, [name]: { ...e, binary: ev.target.value } } } })} />
                </div>
              )
            })}
          </>
        ) : null}
      </div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 8 }}>
        <Button type="primary" size="small" loading={saving} onClick={onSave}>{t('settings.saveConfig')}</Button>
      </div>
    </Card>
  )
}

export default function ScanConfig() {
  const { detectors, fetchDetectors, detectorConfig, fetchDetectorConfig, saveDetectorConfig } = useStore()
  const [filter, setFilter] = useState<string | undefined>(undefined)
  const [draft, setDraft] = useState<DetectorsConfig | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  useEffect(() => { fetchDetectorConfig() }, [fetchDetectorConfig])
  useEffect(() => { if (detectorConfig) setDraft(detectorConfig) }, [detectorConfig])

  const selected = filter ? detectors.find((d) => d.id === filter) : undefined

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 上半: 定时扫描(原 SettingsSchedules) */}
      <SettingsSchedules />

      {/* 下半: 检测器引擎配置 */}
      <DetectorPanel detectors={detectors} selectedId={filter} onSelect={setFilter} />
      {selected ? (
        <DetectorConfigControls d={selected} draft={draft} setDraft={setDraft} saving={saving} onSave={async () => {
          if (!draft) return
          setSaving(true)
          await saveDetectorConfig(draft)
          setSaving(false)
        }} />
      ) : null}
    </div>
  )
}
